package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/mux"
)

func parseTimeThing(s string) (time.Time, error) {
	res, err := http.Get("http://www.timeapi.org/utc/" +
		url.QueryEscape(s))
	if err != nil {
		return time.Time{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return time.Time{}, fmt.Errorf("HTTP Error: %v", res.Status)
	}
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return time.Time{}, err
	}

	return time.Parse(time.RFC3339, string(data))
}

func serveNewReminder(w http.ResponseWriter, r *http.Request) {
	id := fmt.Sprintf("remind-%v",
		time.Now().UTC().Format(time.RFC3339))

	when, err := parseTimeThing(r.FormValue("when"))
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	reminder := Reminder{
		BugId:     mux.Vars(r)["bugid"],
		Type:      "reminder",
		CreatedAt: time.Now().UTC(),
		When:      when,
		User:      whoami(r).Id,
	}

	err = db.Set(id, 0, reminder)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	w.WriteHeader(202)
}
