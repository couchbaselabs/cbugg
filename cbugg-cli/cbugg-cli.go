package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
)

var queryFlag = flag.String("query", "", "query to execute")

func maybeF(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func Config(maybeVal, varname string) string {
	if maybeVal != "" {
		return maybeVal
	}
	tagBytes, err := exec.Command("git", "config", varname).Output()
	if err != nil {
		log.Fatalf("Couldn't get git config value %s: %v", varname, err)
	}
	return strings.TrimSpace(string(tagBytes))
}

func CbuggSearch(query string) {
	base, err := url.Parse(Config("", "cbugg.url"))
	maybeF(err)
	apiRelURL, err := url.Parse("/api/search/")
	maybeF(err)
	apiURL := base.ResolveReference(apiRelURL)
	apiURL.RawQuery = url.Values{"query": {query}}.Encode()

	req, err := http.NewRequest("GET", apiURL.String(), nil)
	maybeF(err)
	req.SetBasicAuth(Config("", "cbugg.user"), Config("", "cbugg.key"))
	resp, err := http.DefaultClient.Do(req)
	maybeF(err)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Fatalf("Unexpected Response: %v", resp.Status)
	}

	m := map[string]interface{}{}
	d := json.NewDecoder(resp.Body)
	err = d.Decode(&m)
	maybeF(err)
	hits := m["hits"].(map[string]interface{})["hits"].([]interface{})
	for _, h := range hits {
		doc := h.(map[string]interface{})["source"].(map[string]interface{})["doc"].(map[string]interface{})
		id := doc["id"]
		title := doc["title"]
		fmt.Printf("%v\t\t%v\n", id, title)
	}
}

func main() {
	flag.Parse()
	CbuggSearch(*queryFlag)
}
