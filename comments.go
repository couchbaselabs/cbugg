package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

func serveNewComment(w http.ResponseWriter, r *http.Request) {
	email := whoami(r)

	bugid := mux.Vars(r)["bugid"]
	id := "c-" + bugid + "-" + time.Now().UTC().Format(time.RFC3339Nano)

	c := Comment{
		Id:        id,
		BugId:     bugid,
		Type:      "comment",
		User:      email,
		Text:      r.FormValue("body"),
		CreatedAt: time.Now().UTC(),
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

	mustEncode(w, APIComment(c))
}

func serveCommentList(w http.ResponseWriter, r *http.Request) {
	bugid := mux.Vars(r)["bugid"]

	args := map[string]interface{}{
		"stale":        false,
		"start_key":    []interface{}{bugid},
		"end_key":      []interface{}{bugid, map[string]string{}},
		"include_docs": true,
	}

	viewRes := struct {
		Rows []struct {
			Doc struct {
				Json APIComment
			}
		}
	}{}

	err := db.ViewCustom("cbugg", "comments", args, &viewRes)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	comments := []APIComment{}

	for _, r := range viewRes.Rows {
		comments = append(comments, r.Doc.Json)
	}

	w.Header().Set("Content-type", "application/json")
	mustEncode(w, comments)
}

func updateCommentDeleted(w http.ResponseWriter, r *http.Request, to bool) {
	email := whoami(r)

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

		if comment.User != email {
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

func serveDelComment(w http.ResponseWriter, r *http.Request) {
	updateCommentDeleted(w, r, true)
}

func serveUnDelComment(w http.ResponseWriter, r *http.Request) {
	updateCommentDeleted(w, r, false)
}
