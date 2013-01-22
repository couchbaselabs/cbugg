package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type githubUser struct {
	Avatar string `json:"avatar_url"`
	Login  string
}

type githubIssueHook struct {
	Action   string
	Callpath string `json:"hook_callpath"`
	Issue    struct {
		Title     string
		Body      string
		CreatedAt time.Time `json:"created_at"`
		URL       string    `json:"html_url"`
		Labels    []struct{ Name string }
		User      githubUser
	}
	Repository struct {
		Url  string `json:"html_url"`
		Name string
	}
}

func serveGithubIssue(w http.ResponseWriter, r *http.Request) {
	hookdata := githubIssueHook{}
	err := json.Unmarshal([]byte(r.FormValue("payload")), &hookdata)

	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	log.Printf("Got hook: %+v", hookdata)

	if hookdata.Action != "opened" || hookdata.Callpath != "new" {
		log.Printf("Something other than create happened, skipping")
		w.WriteHeader(204)
		return
	}

	id, err := newBugId()
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	body := hookdata.Issue.Body
	if body != "" {
		body += "\n\n----\n"
	}
	body += hookdata.Issue.URL +
		"\nby github user: " + hookdata.Issue.User.Login +
		" ![g](" + hookdata.Issue.User.Avatar + "&s=64)"

	labels := []string{}
	for _, l := range hookdata.Issue.Labels {
		labels = append(labels, l.Name)
	}

	bug := Bug{
		Id:          fmt.Sprintf("bug-%v", id),
		Title:       hookdata.Issue.Title,
		Description: body,
		Creator:     *mailFrom,
		Status:      "inbox",
		Tags:        append(labels, "github", hookdata.Repository.Name),
		Type:        "bug",
		CreatedAt:   hookdata.Issue.CreatedAt.UTC(),
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

	w.WriteHeader(204)
}
