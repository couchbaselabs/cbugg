package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sort"
	"strings"

	"github.com/gorilla/mux"
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

func emailIsInternal(email string) bool {
	u, err := getUser(email)
	return err == nil && u.Internal
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

func serveUserList(w http.ResponseWriter, r *http.Request) {
	rv := []string{}

	if whoami(r).Id != "" {
		args := map[string]interface{}{
			"group_level": 1,
		}

		viewRes := struct {
			Rows []struct {
				Key string
			}
		}{}

		err := db.ViewCustom("cbugg", "users", args, &viewRes)
		if err != nil {
			showError(w, r, err.Error(), 500)
			return
		}

		for _, r := range viewRes.Rows {
			if strings.Contains(r.Key, "@") {
				rv = append(rv, r.Key)
			}
		}
		sort.Strings(rv)
	}

	mustEncode(w, rv)
}

func serveSpecialUserList(w http.ResponseWriter, r *http.Request) {
	rv := []Email{}
	t := mux.Vars(r)["type"]
	me := whoami(r)

	if !me.Admin {
		showError(w, r, "You are not an admin", 403)
		return
	}

	args := map[string]interface{}{
		"reduce": false,
		"key":    t,
	}

	viewRes := struct {
		Rows []struct {
			Value string
		}
	}{}

	emails := []string{}
	err := db.ViewCustom("cbugg", "special_users", args, &viewRes)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	for _, r := range viewRes.Rows {
		emails = append(emails, r.Value)
	}
	sort.Strings(emails)

	for _, e := range emails {
		rv = append(rv, Email(e))
	}

	mustEncode(w, rv)
}
