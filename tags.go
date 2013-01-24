package main

import (
	"encoding/json"
	"fmt"
	"log"
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

	tag := Tag{}
	subs := []Email{}
	err = db.Get("tag-"+t, &tag)
	if err == nil {
		for _, e := range tag.Subscribers {
			subs = append(subs, Email(e))
		}
	} else {
		log.Printf("Error fetching tag %v: %v", t, err)
	}

	mustEncode(w, map[string]interface{}{
		"states":      statemap,
		"subscribers": subs,
		"name":        t,
	})
}

func updateTagSubscription(tagname, email string, add bool) error {
	return db.Update("tag-"+tagname, 0, func(current []byte) ([]byte, error) {
		tag := Tag{}
		if len(current) > 0 {
			err := json.Unmarshal(current, &tag)
			if err != nil {
				return nil, err
			}
			if tag.Type != "tag" {
				return nil, fmt.Errorf("Expected a tag, got %v",
					tag.Type)
			}
		}

		tag.Type = "tag"
		tag.Subscribers = removeFromList(tag.Subscribers, email)

		if add {
			tag.Subscribers = append(tag.Subscribers, email)
		}

		return json.Marshal(tag)
	})
}

func serveSubscribeTag(w http.ResponseWriter, r *http.Request) {
	err := updateTagSubscription(mux.Vars(r)["tag"], whoami(r), true)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	w.WriteHeader(204)
}

func serveUnsubscribeTag(w http.ResponseWriter, r *http.Request) {
	err := updateTagSubscription(mux.Vars(r)["tag"], whoami(r), false)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	w.WriteHeader(204)
}
