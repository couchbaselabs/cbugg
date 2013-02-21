package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sort"
	"strings"
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

func serveAdminUserMod(w http.ResponseWriter, r *http.Request) {
	me := whoami(r)

	if !me.Admin {
		showError(w, r, "Must be admin to do this", 403)
		return
	}

	email := r.FormValue("email")
	if email == "" {
		showError(w, r, "no email given", 400)
		return
	}

	key := "u-" + email

	err := db.Update(key, 0, func(current []byte) ([]byte, error) {
		var user User
		if len(current) > 0 {
			err := json.Unmarshal(current, &user)
			if err != nil {
				return nil, err
			}
		}

		// Common fields
		user.Id = email
		user.Type = "user"

		adminVal := r.FormValue("admin")
		if adminVal != "" {
			user.Admin = adminVal == "true"
		}

		internalVal := r.FormValue("internal")
		if internalVal != "" {
			user.Internal = internalVal == "true"
		}

		return json.Marshal(user)
	})

	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	mustEncode(w, Email(email))
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

func findEmailByMD5(m string) string {
	rv := ""
	if ul, err := listUsers(); err == nil {
		for _, e := range ul {
			if md5string(e) == m {
				return e
			}
		}
	}
	return rv
}

func listUsers() ([]string, error) {
	rv := []string{}
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
		return nil, err
	}

	for _, r := range viewRes.Rows {
		if strings.Contains(r.Key, "@") {
			rv = append(rv, r.Key)
		}
	}
	sort.Strings(rv)
	return rv, nil
}

func serveUserList(w http.ResponseWriter, r *http.Request) {
	rv := []string{}

	if whoami(r).Id != "" {
		var err error
		rv, err = listUsers()
		if err != nil {
			showError(w, r, err.Error(), 500)
		}
	}

	mustEncode(w, rv)
}

func serveSpecialUserList(w http.ResponseWriter, r *http.Request) {
	me := whoami(r)

	if !(me.Admin || me.Internal) {
		showError(w, r, "You are not allowed to see this list", 403)
		return
	}

	args := map[string]interface{}{
		"reduce": false,
	}

	viewRes := struct {
		Rows []struct {
			Key, Value string
		}
	}{}

	err := db.ViewCustom("cbugg", "special_users", args, &viewRes)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	rv := map[string]map[string]Email{}
	for _, r := range viewRes.Rows {
		_, ok := rv[r.Key]
		if !ok {
			rv[r.Key] = map[string]Email{}
		}
		rv[r.Key][r.Value] = Email(r.Value)
	}

	mustEncode(w, rv)
}
