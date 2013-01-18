package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/couchbaselabs/go-couchbase"
	"github.com/gorilla/mux"
)

func newBugId() (uint64, error) {
	return db.Incr(".bugid", 1, 0, 0)
}

func serveBugRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/static/app.html#/bug/"+mux.Vars(r)["bugid"],
		http.StatusFound)
}

func serveNewBug(w http.ResponseWriter, r *http.Request) {
	email := whoami(r)

	id, err := newBugId()
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	bug := Bug{
		Id:          fmt.Sprintf("bug-%v", id),
		Title:       r.FormValue("title"),
		Description: r.FormValue("description"),
		Status:      "inbox",
		Creator:     email,
		Tags:        r.Form["tag"],
		Type:        "bug",
		Subscribers: []string{email},
		CreatedAt:   time.Now().UTC(),
	}

	added, err := db.Add(bug.Id, 0, bug)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}
	if !added {
		// This is a bug bug
		showError(w, r, "Bug collision on "+bug.Id, 500)
		return
	}

	http.Redirect(w, r, bug.Url(), 303)
}

func getBug(bugid string) (Bug, error) {
	bug := Bug{}
	err := db.Get(bugid, &bug)
	return bug, err
}

func serveBug(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["bugid"]
	bug, err := getBug(id)
	if err != nil {
		showError(w, r, err.Error(), 404)
		return
	}

	hist, err := getBugHistory(id)
	if err != nil {
		showError(w, r, err.Error(), 404)
		return
	}
	robj, err := json.Marshal(map[string]interface{}{
		"bug":     APIBug(bug),
		"history": hist,
	})
	if err != nil {
		showError(w, r, err.Error(), 404)
		return
	}
	w.Write(robj)
}

type BugHistoryItem struct {
	Key       string
	Timestamp time.Time
	ModInfo   map[string]interface{}
}

func getBugHistory(id string) ([]BugHistoryItem, error) {
	args := map[string]interface{}{
		"stale":     false,
		"start_key": []interface{}{id},
		"end_key":   []interface{}{id, map[string]string{}},
	}

	res, err := db.View("cbugg", "bug_history", args)
	if err != nil {
		return nil, err
	}

	histitems := []BugHistoryItem{}

	for _, r := range res.Rows {
		h := r.Value.(map[string]interface{})
		if s, ok := h["by"].(string); ok && s != "" {
			h["by"] = User(s)
		}
		t, err := time.Parse(time.RFC3339, r.Key.([]interface{})[1].(string))
		if err != nil {
			log.Printf("Error parsing timestamp: %v", err)
			continue
		}
		histitems = append(histitems, BugHistoryItem{
			r.ID,
			t,
			h,
		})
	}

	return histitems, nil
}

func serveBugUpdate(w http.ResponseWriter, r *http.Request) {
	email := whoami(r)

	id := mux.Vars(r)["bugid"]
	field := r.FormValue("id")
	r.ParseForm()
	val := r.FormValue("value")
	rval := []byte{}
	now := time.Now().UTC()

	historyKey := id + "-" + now.Format(time.RFC3339Nano)

	err := db.Update(id, 0, func(current []byte) ([]byte, error) {
		if len(current) == 0 {
			return nil, NotFound
		}
		bug := Bug{}
		err := json.Unmarshal(current, &bug)
		if err != nil {
			return nil, err
		}

		history := Bug{
			Id:         id,
			Type:       "bughistory",
			ModifiedAt: now,
			ModType:    r.FormValue("id"),
			ModBy:      email,
		}

		var oldval string
		switch field {
		case "description":
			oldval = bug.Description
			history.Description = bug.Description
			bug.Description = val
		case "title":
			oldval = bug.Title
			history.Title = bug.Title
			bug.Title = val
		case "status":
			oldval = bug.Title
			history.Status = bug.Status
			bug.Status = val
		case "owner":
			oldval = bug.Owner
			history.Owner = bug.Owner
			bug.Owner = val

			// Ensure the owner is subscribed
			if strings.Contains(val, "@") {
				bug.Subscribers = removeFromList(bug.Subscribers, val)
				bug.Subscribers = append(bug.Subscribers, val)
			}
		case "tags":
			history.Tags = bug.Tags
			bug.Tags = strings.FieldsFunc(val,
				func(r rune) bool {
					switch r {
					case ',', ' ':
						return true
					}
					return false
				})
		default:
			return nil, fmt.Errorf("Unhandled id: %v",
				r.FormValue("id"))
		}

		if oldval == val {
			rval, err = json.Marshal(APIBug(bug))
			if err != nil {
				return rval, err
			}
			return rval, couchbase.UpdateCancel
		}

		// This is a side-effect in a CAS operation.  It's is
		// correct and safe because the side effect is the
		// creation of a document that is only used and
		// correct relative to the final value from the CAS.
		err = db.Set(historyKey, 0, &history)
		if err != nil {
			return nil, err
		}

		bug.ModifiedAt = now
		bug.Parent = historyKey

		// The version that goes to the DB is different from
		// the one that goes to the API.
		dbval, err := json.Marshal(&bug)
		if err != nil {
			return nil, err
		}

		rval, err = json.Marshal(APIBug(bug))

		return dbval, err
	})

	switch err {
	case nil:
		notifyBugChange(id, field, email)
		if field == "owner" {
			if val != email {
				notifyBugAssignment(id, val)
			}
		}
	case couchbase.UpdateCancel:
		log.Printf("Ignoring identical update of %v", field)
	default:
		http.Error(w, err.Error(), 400)
		return
	}

	w.Write([]byte(rval))
}

func serveBugList(w http.ResponseWriter, r *http.Request) {
	args := map[string]interface{}{
		"reduce": false,
		"stale":  false,
	}

	viewName := "by_state"

	viewRes := struct {
		Rows []struct {
			ID    string
			Key   []string
			Value struct {
				Title string
				Owner User
			}
		}
	}{}

	if r.FormValue("user") == "" {
		if r.FormValue("state") != "" {
			st := r.FormValue("state")
			args["start_key"] = []interface{}{st}
			args["end_key"] = []interface{}{st, map[string]string{}}
		}
	} else {
		viewName = "owners"
		u := r.FormValue("user")
		if r.FormValue("state") != "" {
			st := r.FormValue("state")
			args["start_key"] = []interface{}{u, st}
			args["end_key"] = []interface{}{u, st, map[string]string{}}
		}
	}

	err := db.ViewCustom("cbugg", viewName, args, &viewRes)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	jres, err := json.Marshal(viewRes.Rows)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}
	w.Write(jres)
}

func serveSubscribeBug(w http.ResponseWriter, r *http.Request) {
	err := updateSubscription(mux.Vars(r)["bugid"],
		whoami(r), true)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	w.WriteHeader(204)
}

func serveUnsubscribeBug(w http.ResponseWriter, r *http.Request) {
	err := updateSubscription(mux.Vars(r)["bugid"],
		whoami(r), false)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	w.WriteHeader(204)
}
