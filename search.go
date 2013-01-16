package main

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/mattbaird/elastigo/api"
	"github.com/mattbaird/elastigo/core"
)

func searchBugs(w http.ResponseWriter, r *http.Request) {

	api.Domain = *esHost
	api.Port = *esPort
	api.Protocol = *esScheme

	from := 0
	var err error
	if r.FormValue("from") != "" {
		from, err = strconv.Atoi(r.FormValue("from"))
		if err != nil {
			log.Printf("Invalid value for from: %v", r.FormValue("from"))
			from = 0
		}
	}

	if r.FormValue("query") != "" {

		filter := map[string]interface{}{
			"and": []interface{}{
				map[string]interface{}{
					"type": map[string]interface{}{
						"value": "couchbaseDocument",
					},
				},
				map[string]interface{}{
					"term": map[string]interface{}{
						"doc.type": "bug",
					},
				},
			},
		}

		query := map[string]interface{}{
			"query": map[string]interface{}{
				"query_string": map[string]interface{}{
					"query": r.FormValue("query"),
				},
			},
			"filter": filter,
			"from":   from,
			"facets": map[string]interface{}{
				"statuses": map[string]interface{}{
					"terms": map[string]interface{}{
						"field": "doc.status",
						"size":  5,
					},
					"facet_filter": filter,
				},
				"tags": map[string]interface{}{
					"terms": map[string]interface{}{
						"field": "doc.tags",
						"size":  5,
					},
					"facet_filter": filter,
				},
			},
		}

		searchresponse, err := core.Search(false, *esIndex, "couchbaseDocument", query, "")
		if err != nil {
			showError(w, r, err.Error(), 500)
			return
		}

		ourresponse := map[string]interface{}{
			"took":      searchresponse.Took,
			"timed_out": searchresponse.TimedOut,
			"hits": map[string]interface{}{
				"total": 0,
				"hits":  []interface{}{},
			},
			"_shards":    searchresponse.ShardStatus,
			"_scroll_id": searchresponse.ScrollId,
		}

		if searchresponse.Hits.Total > 0 {
			hitrecords := make([]interface{}, 0)

			ids := make([]string, 0)

			// walk through the hits, building list of ids
			for _, hit := range searchresponse.Hits.Hits {
				ids = append(ids, hit.Id)
			}

			// bulk get the docs we're interested in
			bulkResponse := db.GetBulk(ids)

			// walk through the hits again, adding the original document to the source
			for _, hit := range searchresponse.Hits.Hits {

				// find the couchbase response for this hit
				mcResponse := bulkResponse[hit.Id]
				cbDoc := map[string]interface{}{}
				// unmarshall the json
				jsonErr := json.Unmarshal(mcResponse.Body, &cbDoc)
				if jsonErr != nil {
					// if any error occurred, assume it wasnt json, return full data as base64
					cbDoc = map[string]interface{}{"base64": base64.StdEncoding.EncodeToString(mcResponse.Body)}
				}

				ourhit, err := combineSearchHitWithDoc(hit, cbDoc)
				if err != nil {
					log.Printf("%v", err)
					continue
				}
				hitrecords = append(hitrecords, ourhit)
			}

			ourfacets := map[string]interface{}{}
			err = json.Unmarshal(searchresponse.Facets, &ourfacets)
			if err != nil {
				log.Printf("Error unmarshalling search result facets: %v", err)
			}

			ourhits := map[string]interface{}{
				"total": searchresponse.Hits.Total,
				"hits":  hitrecords,
			}
			ourresponse["hits"] = ourhits
			ourresponse["facets"] = ourfacets

		}

		jres, err := json.Marshal(ourresponse)
		if err != nil {
			showError(w, r, err.Error(), 500)
			return
		}
		w.Write(jres)
		return
	}

	showError(w, r, "Search query cannot be empty", 500)
}

func combineSearchHitWithDoc(hit core.Hit, doc interface{}) (map[string]interface{}, error) {
	result := map[string]interface{}{
		"_index": hit.Index,
		"_type":  hit.Type,
		"_id":    hit.Id,
		"_score": hit.Score,
	}

	var source map[string]interface{}
	err := json.Unmarshal(hit.Source, &source)
	if err != nil {
		return nil, err
	}

	source["doc"] = doc

	result["source"] = source

	return result, nil
}
