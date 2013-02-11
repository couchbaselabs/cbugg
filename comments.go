package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

func serveNewComment(w http.ResponseWriter, r *http.Request) {
	me := whoami(r)
	bugid := mux.Vars(r)["bugid"]

	if _, err := getBugOrDisplayErr(bugid, me, w, r); err != nil {
		return
	}

	id := "c-" + bugid + "-" + time.Now().UTC().Format(time.RFC3339Nano)

	c := Comment{
		Id:        id,
		BugId:     bugid,
		Type:      "comment",
		User:      me.Id,
		Text:      r.FormValue("body"),
		CreatedAt: time.Now().UTC(),
		Private:   r.FormValue("private") == "true",
	}

	added, err := db.Add(c.Id, 0, c)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}
	if !added {
		// This is a bug bug
		showError(w, r, "Comment collision on "+c.Id, 500)
		return
	}

	notifyComment(c)

	err = updateSubscription(bugid, me.Id, true)
	if err != nil {
		log.Printf("Error subscribing commenter %v to bug %v: %v",
			me.Id, bugid, err)
	}

	mustEncode(w, APIComment(c))
}

func serveCommentList(w http.ResponseWriter, r *http.Request) {
	me := whoami(r)
	bugid := mux.Vars(r)["bugid"]

	if _, err := getBugOrDisplayErr(bugid, me, w, r); err != nil {
		return
	}

	args := map[string]interface{}{
		"stale":        false,
		"start_key":    []interface{}{bugid},
		"end_key":      []interface{}{bugid, map[string]string{}},
		"include_docs": true,
	}

	viewRes := struct {
		Rows []struct {
			Value string
			Doc   struct {
				Json *json.RawMessage
			}
		}
	}{}

	err := db.ViewCustom("cbugg", "comments", args, &viewRes)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	type innerThing struct {
		Type    string `json:"type"`
		Private bool   `json:"private"`
	}

	// Below we mix comments and pings together for the "comment"
	// UI, but we can't just pass the document body straight to
	// the client because we want gravatars calculated and
	// usernames obscured, so we do a first pass parse to detect
	// the type and then parse it into the correct target type.
	rv := []interface{}{}
	for _, row := range viewRes.Rows {
		t := innerThing{}
		err := json.Unmarshal([]byte(*row.Doc.Json), &t)
		if err != nil {
			showError(w, r, err.Error(), 500)
			return
		}

		var parseTo interface{}

		switch t.Type {
		case "comment":
			parseTo = &APIComment{}
		case "ping":
			parseTo = &APIPing{}
		}

		if isVisible(parseTo, me) {
			err = json.Unmarshal([]byte(*row.Doc.Json), parseTo)
			if err != nil {
				showError(w, r, err.Error(), 500)
				return
			}

			rv = append(rv, parseTo)
		}
	}

	mustEncode(w, rv)
}

func updateCommentDeleted(w http.ResponseWriter, r *http.Request, to bool) {
	me := whoami(r)

	err := db.Update(mux.Vars(r)["commid"], 0, func(current []byte) ([]byte, error) {
		if len(current) == 0 {
			return nil, NotFound
		}
		comment := Comment{}
		err := json.Unmarshal(current, &comment)
		if err != nil {
			return nil, err
		}

		if comment.Type != "comment" {
			return nil, fmt.Errorf("Expected a comment, got %v",
				comment.Type)
		}

		if !(me.Admin || comment.User == me.Id) {
			return nil, fmt.Errorf("You can only delete your own comments")
		}

		comment.Deleted = to

		return json.Marshal(comment)
	})

	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	w.WriteHeader(204)
}

func getComment(commid string) (Comment, error) {
	comment := Comment{}
	err := db.Get(commid, &comment)
	return comment, err
}

func serveCommentUpdate(w http.ResponseWriter, r *http.Request) {
	me := whoami(r)
	commid := mux.Vars(r)["commid"]

	c := Comment{}
	err := db.Get(commid, &c)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	if c.Type != "comment" || c.User != me.Id {
		showError(w, r, "You can't change this comment", 403)
		return
	}

	c.Text = r.FormValue("body")

	err = db.Set(commid, 0, &c)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	w.WriteHeader(204)
}

func serveDelComment(w http.ResponseWriter, r *http.Request) {
	updateCommentDeleted(w, r, true)
}

func serveUnDelComment(w http.ResponseWriter, r *http.Request) {
	updateCommentDeleted(w, r, false)
}
