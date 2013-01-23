package main

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/mattbaird/elastigo/api"
	"github.com/mattbaird/elastigo/core"
)

// by default we only want documents of elasticsearch type "couchbaseDocument"
// and documents with type "bug"
func getDefaultFilterComponents() []Filter {
	return []Filter{
		buildTypeFilter("couchbaseDocument"),
		buildTermFilter("doc.type", "bug"),
	}
}

// powers the bug similarity feature when entering new bugs
func findSimilarBugs(w http.ResponseWriter, r *http.Request) {

	api.Domain = *esHost
	api.Port = *esPort
	api.Protocol = *esScheme

	if r.FormValue("query") != "" {

		filterComponents := getDefaultFilterComponents()
		matchFilter := buildAndFilter(filterComponents)

		activeBugsFilter := buildTermsFilter("doc.status", []string{"inbox", "new", "open"}, "")
		inactiveBugsFilter := buildTermsFilter("doc.status", []string{"resolved", "closed"}, "")

		boostFilters := Filters{
			map[string]interface{}{
				"filter": activeBugsFilter,
				"boost":  10,
			},
			map[string]interface{}{
				"filter": inactiveBugsFilter,
				"boost":  1,
			},
		}

		query := buildCustomFiltersScoreQuery(r.FormValue("query"), matchFilter, boostFilters, "first")

		if *debugEs {
			queryJson, err := json.Marshal(query)
			if err == nil {
				log.Printf("Elasticsearch query: %v", string(queryJson))
			}
		}

		searchresponse, err := core.Search(false, *esIndex, "couchbaseDocument", query, "")
		if err != nil {
			showError(w, r, err.Error(), 500)
			return
		}

		ourresponse := convertElasticSearchResponse(searchresponse)

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

// powers the bug search
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

		filterComponents := getDefaultFilterComponents()

		if r.FormValue("status") != "" {
			statusFilter := buildTermsFilter("doc.status", strings.Split(r.FormValue("status"), ","), "")
			filterComponents = append(filterComponents, statusFilter)
		}

		if r.FormValue("tags") != "" {
			tagsFilter := buildTermsFilter("doc.tags", strings.Split(r.FormValue("tags"), ","), "and")
			filterComponents = append(filterComponents, tagsFilter)
		}

		filter := buildAndFilter(filterComponents)

		statusFacet := buildTermsFacet("doc.status", filter, 5)
		tagsFacet := buildTermsFacet("doc.tags", filter, 5)

		facets := Facets{
			"statuses": statusFacet,
			"tags":     tagsFacet,
		}

		query := buildQueryStringQuery(r.FormValue("query"), filter, facets, from)

		if *debugEs {
			queryJson, err := json.Marshal(query)
			if err == nil {
				log.Printf("Elasticsearch query: %v", string(queryJson))
			}
		}

		searchresponse, err := core.Search(false, *esIndex, "couchbaseDocument", query, "")
		if err != nil {
			showError(w, r, err.Error(), 500)
			return
		}

		ourresponse := convertElasticSearchResponse(searchresponse)

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

// this unfortunate function converse a response from Elasticsearch
// into one we can return to the caller
// primarily it decodes some fields into JSON which were left as RawMessage
// by the elasticsearch library we're using
func convertElasticSearchResponse(searchresponse core.SearchResult) map[string]interface{} {
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
		if searchresponse.Facets != nil {
			err := json.Unmarshal(searchresponse.Facets, &ourfacets)
			if err != nil {
				log.Printf("Error unmarshalling search result facets: %v", err)
			}
		}

		ourhits := map[string]interface{}{
			"total": searchresponse.Hits.Total,
			"hits":  hitrecords,
		}
		ourresponse["hits"] = ourhits
		ourresponse["facets"] = ourfacets

	}

	return ourresponse
}

// this function takes a document from couchbase
// and merges it into a search result hit
// resulting in a response that looks like it contained a complete document
// even though ElasticSearch only had the meta data
// and we looked up the full document bodies in Couchbase
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

// ---------------------------------------
// Elasticsearch utility functions
// ---------------------------------------

type Filter map[string]interface{}
type Filters []map[string]interface{}
type Query map[string]interface{}
type Facet map[string]interface{}
type Facets map[string]interface{}

func buildTermsFilter(field string, terms []string, execution string) Filter {
	if execution == "" {
		execution = "bool"
	}

	return Filter{
		"terms": map[string]interface{}{
			field:       terms,
			"execution": execution,
		},
	}
}

func buildTermFilter(field string, term string) Filter {
	return Filter{
		"term": map[string]interface{}{
			field: term,
		},
	}
}

func buildTypeFilter(typ string) Filter {
	return Filter{
		"type": map[string]interface{}{
			"value": typ,
		},
	}
}

func buildAndFilter(components []Filter) Filter {
	return Filter{
		"and": components,
	}
}

func buildTermsFacet(field string, filter Filter, size int) Facet {
	return map[string]interface{}{
		"terms": map[string]interface{}{
			"field": field,
			"size":  size,
		},
		"facet_filter": filter,
	}
}

func buildQueryStringQuery(queryString string, filter Filter, facets Facets, from int) Query {
	return Query{
		"query": map[string]interface{}{
			"query_string": map[string]interface{}{
				"query": queryString,
			},
		},
		"filter": filter,
		"from":   from,
		"facets": facets,
	}
}

func buildCustomFiltersScoreQuery(queryString string, matchFilter Filter, boostFilters Filters, scoreMode string) Query {

	return Query{
		"query": map[string]interface{}{
			"custom_filters_score": map[string]interface{}{
				"query": map[string]interface{}{
					"query_string": map[string]interface{}{
						"query": queryString,
					},
				},
				"filters":    boostFilters,
				"score_mode": scoreMode,
			},
		},
		"filter": matchFilter,
	}
}
