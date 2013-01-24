package main

import (
	"net/http"

	"github.com/gorilla/mux"
)

func serveTagList(w http.ResponseWriter, r *http.Request) {
	args := map[string]interface{}{
		"group_level": 1,
	}

	viewRes := struct {
		Rows []struct {
			Key   []string
			Value int
		}
	}{}

	err := db.ViewCustom("cbugg", "tags", args, &viewRes)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	rv := map[string]int{}
	for _, r := range viewRes.Rows {
		rv[r.Key[0]] = r.Value
	}

	mustEncode(w, rv)
}

func serveTagStates(w http.ResponseWriter, r *http.Request) {
	t := mux.Vars(r)["tag"]
	args := map[string]interface{}{
		"group_level": 2,
		"stale":       false,
		"start_key":   []interface{}{t},
		"end_key":     []interface{}{t, map[string]interface{}{}},
	}
	states, err := db.View("cbugg", "tags", args)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	statemap := map[string]interface{}{}
	for _, row := range states.Rows {
		statemap[row.Key.([]interface{})[1].(string)] = row.Value
	}

	mustEncode(w, map[string]interface{}{"states": statemap})
}
