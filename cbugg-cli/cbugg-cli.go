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
	"time"
)

var queryFlag = flag.String("query", "", "query to execute")

type User struct {
	Email string
	MD5   string
}

type searchDoc struct {
	Id          string
	Title       string
	Tags        []string
	Description string
	CreatedAt   time.Time `json:"created_at"`
	Creator     User
	ModifiedAt  time.Time `json:"modified_at"`
	ModifiedBy  User      `json:"modified_by"`
	Owner       User
	Private     bool
	Subscribers []User
}

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

func CbuggSearch(query string) ([]searchDoc, error) {
	base, err := url.Parse(Config("", "cbugg.url"))
	if err != nil {
		return nil, err
	}
	apiRelURL, err := url.Parse("/api/search/")
	if err != nil {
		return nil, err
	}
	apiURL := base.ResolveReference(apiRelURL)
	apiURL.RawQuery = url.Values{"query": {query}}.Encode()

	req, err := http.NewRequest("GET", apiURL.String(), nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(Config("", "cbugg.user"), Config("", "cbugg.key"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP Error: %v", resp.Status)
	}

	results := struct {
		Hits struct {
			Hits []struct {
				Source struct {
					Doc searchDoc
				}
			}
		}
	}{}

	d := json.NewDecoder(resp.Body)
	err = d.Decode(&results)
	if err != nil {
		return nil, err
	}

	rv := []searchDoc{}
	for _, r := range results.Hits.Hits {
		rv = append(rv, r.Source.Doc)
	}

	return rv, nil
}

func main() {
	flag.Parse()
	res, err := CbuggSearch(*queryFlag)
	maybeF(err)

	for _, r := range res {
		fmt.Printf("%v\t\t%v\n", r.Id, r.Title)
	}
}
