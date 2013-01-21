package main

import (
	"net/http"
)

func serveTagList(w http.ResponseWriter, r *http.Request) {
	args := map[string]interface{}{
		"group_level": 1,
	}

	viewRes := struct {
		Rows []struct {
			Key   string
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
		rv[r.Key] = r.Value
	}

	mustEncode(w, rv)
}
