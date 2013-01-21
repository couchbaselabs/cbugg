package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/dustin/gomemcached"
)

var NotAUser = errors.New("not a user")

func getUser(email string) (User, error) {
	rv := User{}
	k := "u-" + email
	err := db.Get(k, &rv)
	if err == nil && rv.Type != "user" {
		return User{}, NotAUser
	}
	return rv, err
}

func serveMe(w http.ResponseWriter, r *http.Request) {
	var rv User
	var err error

	me := whoami(r)
	if me != "" {
		rv, err = getUser(me)
		if !(err == nil || gomemcached.IsNotFound(err)) {
			showError(w, r, err.Error(), 500)
			return
		}
		rv.Id = me
		rv.Type = "user"
	}

	w.Header().Set("Content-type", "application/json")
	mustEncode(w, rv)
}

func serveSetMyPrefs(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Type") != "application/json" {
		log.Printf("POST should be application/json, not %v",
			r.Header.Get("Content-Type"))
	}

	me := whoami(r)
	key := "u-" + me
	user := User{}

	parsedPrefs := user.Prefs

	d := json.NewDecoder(r.Body)
	err := d.Decode(&parsedPrefs)
	if err != nil {
		showError(w, r, err.Error(), 400)
		return
	}

	err = db.Update(key, 0, func(current []byte) ([]byte, error) {
		if len(current) > 0 {
			err := json.Unmarshal(current, &user)
			if err != nil {
				return nil, err
			}
		}

		// Common fields
		user.Id = me
		user.Type = "user"

		user.Prefs = parsedPrefs

		return json.Marshal(user)
	})

	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	w.Header().Set("Content-type", "application/json")
	mustEncode(w, user)
}
