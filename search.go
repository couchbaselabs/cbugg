package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mschoch/elastigo/api"
	"github.com/mschoch/elastigo/core"
)

// by default we only want documents with type "bug"
func getDefaultFilterComponents(r *http.Request) []Filter {

	// base defaults for all users
	result := []Filter{
		buildTermFilter("doc.type", "bug"),
	}

	// if users is external, add additional filter
	if whoami(r).Internal == false {
		result = append(result, buildNotFilter(buildTermFilter("doc.private", "true")))
	}

	return result
}

// powers the bug similarity feature when entering new bugs
func findSimilarBugs(w http.ResponseWriter, r *http.Request) {

	api.Domain = *esHost
	api.Port = *esPort
	api.Protocol = *esScheme

	if r.FormValue("query") != "" {

		filterComponents := getDefaultFilterComponents(r)
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

		queryStringQuery := buildQueryStringQuery(r.FormValue("query"))
		customScoreQuery := buildCustomFiltersScoreQuery(queryStringQuery, boostFilters, "first")
		query := buildTopLevelQuery(customScoreQuery, matchFilter, nil, nil, 0, 10)

		if *debugEs {
			queryJson, err := json.Marshal(query)
			if err == nil {
				log.Printf("Elasticsearch query: %v", string(queryJson))
			}
		}

		searchresponse, err := core.SearchRequest(false, *esIndex, "", query, "")
		if err != nil {
			showError(w, r, err.Error(), 500)
			return
		}

		if *debugEs {
			searchresponseJson, err := json.Marshal(searchresponse)
			if err == nil {
				log.Printf("Elasticsearch response: %v", string(searchresponseJson))
			}
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

	size := 10
	if r.FormValue("size") != "" {
		size, err = strconv.Atoi(r.FormValue("size"))
		if err != nil {
			log.Printf("Invalid value for size: %v", r.FormValue("size"))
			size = 10
		}
	}

	sortItems := []SortItem{}
	if r.FormValue("sort") != "" {
		sortItems = append(sortItems, buildSortItemFromString(r.FormValue("sort")))

		// if they aren't sorting by +/- _score
		// add that after what they do want to sort by
		// this ensure score is still caclulated
		// and offers secondary sort by the score
		if !strings.HasSuffix(r.FormValue("sort"), "_score") {
			sortItems = append(sortItems, buildSortItemFromString("-_score"))
		}
	}

	filterComponents := getDefaultFilterComponents(r)

	if r.FormValue("status") != "" {
		statusFilter := buildTermsFilter("doc.status", strings.Split(r.FormValue("status"), ","), "")
		filterComponents = append(filterComponents, statusFilter)
	}

	if r.FormValue("tags") != "" {
		tagsFilter := buildTermsFilter("doc.tags", strings.Split(r.FormValue("tags"), ","), "and")
		filterComponents = append(filterComponents, tagsFilter)
	}

	if r.FormValue("modified") != "" {

		now := time.Now()
		var dateRange Range
		switch r.FormValue("modified") {
		case "lt7":
			sevenDaysAgo := now.Add(time.Duration(24) * time.Hour * -7)
			dateRange = buildRange(sevenDaysAgo, nil)
		case "7to30":
			sevenDaysAgo := now.Add(time.Duration(24) * time.Hour * -7)
			thirtyDaysAgo := now.Add(time.Duration(24) * time.Hour * -30)
			dateRange = buildRange(thirtyDaysAgo, sevenDaysAgo)
		case "gt30":
			thirtyDaysAgo := now.Add(time.Duration(24) * time.Hour * -30)
			dateRange = buildRange(nil, thirtyDaysAgo)

		}
		modifiedFilter := buildRangeFilter("doc.modified_at", dateRange)
		filterComponents = append(filterComponents, modifiedFilter)
	}

	filter := buildAndFilter(filterComponents)

	statusFacet := buildTermsFacet("doc.status", filter, 5)
	tagsFacet := buildTermsFacet("doc.tags", filter, 5)
	lastModifiedFacet := buildLastModifiedFacet("doc.modified_at", filter)

	facets := Facets{
		"statuses":      statusFacet,
		"tags":          tagsFacet,
		"last_modified": lastModifiedFacet,
	}

	// all the queries that should be matched
	shouldQueries := []Query{}

	// default to match all query
	insideQuery := buildMatchAllQuery()

	// if they actually provided a query string, run that instead
	if r.FormValue("query") != "" {
		insideQuery = buildQueryStringQuery(r.FormValue("query"))

		// only add these child queries if we actually have a query string
		childTypesToQuery := []string{"comment", "attachment"}
		for _, typ := range childTypesToQuery {
			queryComponent := buildHashChildQuery(typ, insideQuery)
			shouldQueries = append(shouldQueries, queryComponent)
		}
	}

	shouldQueries = append(shouldQueries, insideQuery)

	booleanQuery := buildBoolQuery(nil, shouldQueries, nil, 1)

	query := buildTopLevelQuery(booleanQuery, filter, facets, sortItems, from, size)

	if *debugEs {
		queryJson, err := json.Marshal(query)
		if err == nil {
			log.Printf("Elasticsearch query: %v", string(queryJson))
		}
	}

	searchresponse, err := core.SearchRequest(false, *esIndex, "", query, "")
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	if *debugEs {
		searchresponseJson, err := json.Marshal(searchresponse)
		if err == nil {
			log.Printf("Elasticsearch response: %v", string(searchresponseJson))
		}
	}

	ourresponse := convertElasticSearchResponse(searchresponse)

	jres, err := json.Marshal(ourresponse)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}
	w.Write(jres)
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
			cbDoc := APIBug{}
			// unmarshall the json
			err := json.Unmarshal(mcResponse.Body, &cbDoc)
			if err != nil {
				log.Printf("%v", err)
				continue
			}
			sort.Strings(cbDoc.Tags)

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

func buildLastModifiedFacet(field string, filter Filter) Facet {
	now := time.Now()
	sevenDaysAgo := now.Add(time.Duration(24) * time.Hour * -7)
	thirtyDaysAgo := now.Add(time.Duration(24) * time.Hour * -30)

	thisWeekRange := buildRange(sevenDaysAgo, nil)
	thisMonthRange := buildRange(thirtyDaysAgo, sevenDaysAgo)
	moreThanMonthRange := buildRange(nil, thirtyDaysAgo)

	return buildRangeFacet(field, filter, []Range{thisWeekRange, thisMonthRange, moreThanMonthRange})
}

// ---------------------------------------
// Elasticsearch utility functions
// ---------------------------------------

type Filter map[string]interface{}
type Filters []map[string]interface{}
type Query map[string]interface{}
type Facet map[string]interface{}
type Facets map[string]interface{}
type Range map[string]interface{}
type SortItem map[string]interface{}

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

func buildRangeFilter(field string, r Range) Filter {
	return Filter{
		"range": map[string]interface{}{
			field: r,
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

func buildNotFilter(filter Filter) Filter {
	return Filter{
		"not": filter,
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

func buildRangeFacet(field string, filter Filter, ranges []Range) Facet {
	return map[string]interface{}{
		"range": map[string]interface{}{
			field: ranges,
		},
		"facet_filter": filter,
	}
}

func buildRange(from, to interface{}) Range {
	return map[string]interface{}{
		"from": from,
		"to":   to,
	}
}

func buildTopLevelQuery(query Query, filter Filter, facets Facets, sortItems []SortItem, from int, size int) Query {
	q := Query{
		"query":  query,
		"filter": filter,
		"from":   from,
		"size":   size,
	}

	if sortItems != nil {
		q["sort"] = sortItems
	}

	if facets != nil {
		q["facets"] = facets
	}

	return q
}

func buildQueryStringQuery(queryString string) Query {
	return Query{
		"query_string": map[string]interface{}{
			"query": queryString,
		},
	}
}

func buildBoolQuery(must []Query, should []Query, must_not []Query, minimum_number_should_match int) Query {
	b := map[string]interface{}{
		"minimum_number_should_match": minimum_number_should_match,
	}

	if must != nil {
		b["must"] = must
	}

	if should != nil {
		b["should"] = should
	}

	if must_not != nil {
		b["must_not"] = must_not
	}

	q := Query{
		"bool": b,
	}

	return q
}

func buildHashChildQuery(typ string, query Query) Query {
	return Query{
		"has_child": map[string]interface{}{
			"type":  typ,
			"query": query,
		},
	}
}

func buildMatchAllQuery() Query {
	return Query{
		"match_all": map[string]interface{}{},
	}
}

func buildCustomFiltersScoreQuery(query Query, boostFilters Filters, scoreMode string) Query {

	return Query{
		"custom_filters_score": map[string]interface{}{
			"query":      query,
			"filters":    boostFilters,
			"score_mode": scoreMode,
		},
	}
}

func buildSortItemFromString(sort string) SortItem {
	rv := SortItem{}
	if strings.HasPrefix(sort, "-") {
		rv[sort[1:]] = "desc"
	} else {
		rv[sort] = "asc"
	}
	return rv
}
