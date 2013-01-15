package main

import (
	"encoding/json"
	"net/http"

	"github.com/mattbaird/elastigo/api"
	"github.com/mattbaird/elastigo/core"
)

func searchBugs(w http.ResponseWriter, r *http.Request) {

	api.Domain = *esHost
	api.Port = *esPort
	api.Protocol = *esScheme

	if r.FormValue("query") != "" {

		query := map[string]interface{}{
			"query": map[string]interface{}{
				"query_string": map[string]interface{}{
					"query": r.FormValue("query"),
				},
			},
			"filter": map[string]interface{}{
				"type": map[string]interface{}{
					"value": "couchbaseDocument",
				},
			},
		}

		searchresponse, err := core.Search(false, *esIndex, "couchbaseDocument", query, "")
		if err != nil {
			showError(w, r, err.Error(), 500)
			return
		}

		jres, err := json.Marshal(searchresponse)
		if err != nil {
			showError(w, r, err.Error(), 500)
			return
		}
		w.Write(jres)
		return
	}

	showError(w, r, "Search query cannot be empty", 500)
}
