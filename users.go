package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
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
	me := whoami(r)
	me.AuthToken = ""

	mustEncode(w, me)
}

func serveSetMyPrefs(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Type") != "application/json" {
		log.Printf("POST should be application/json, not %v",
			r.Header.Get("Content-Type"))
	}

	me := whoami(r)
	key := "u-" + me.Id
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
		user.Id = me.Id
		user.Type = "user"

		user.Prefs = parsedPrefs

		return json.Marshal(user)
	})

	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	mustEncode(w, user)
}

func serveUserAuthToken(w http.ResponseWriter, r *http.Request) {
	me := whoami(r)
	// If the user doesn't have an auth token, make one.
	if me.AuthToken == "" {
		serveUpdateUserAuthToken(w, r)
		return
	}

	mustEncode(w, map[string]string{"token": me.AuthToken})
}

func serveUpdateUserAuthToken(w http.ResponseWriter, r *http.Request) {
	me := whoami(r)
	key := "u-" + me.Id
	user := User{}

	err := db.Update(key, 0, func(current []byte) ([]byte, error) {
		if len(current) > 0 {
			err := json.Unmarshal(current, &user)
			if err != nil {
				return nil, err
			}
		}

		// Common fields
		user.Id = me.Id
		user.Type = "user"

		user.AuthToken = randstring(16)

		return json.Marshal(user)
	})

	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	mustEncode(w, map[string]string{"token": user.AuthToken})
}
