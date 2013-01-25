package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"
	"unicode"
)

type githubUser struct {
	Avatar string `json:"avatar_url"`
	Login  string
}

type GithubRepository struct {
	Url  string `json:"html_url"`
	Name string
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
		Pull      struct {
			PatchURL *string `json:"patch_url"`
		} `json:"pull_request"`
	}
	Repository GithubRepository
}

type githubPullRequestHook struct {
	Action      string
	Callpath    string `json:"hook_callpath"`
	PullRequest struct {
		Title     string
		Body      string    `json:"body"`
		CreatedAt time.Time `json:"created_at"`
		PatchURL  string    `json:"patch_url"`
		URL       string    `json:"html_url"`
		User      githubUser
	} `json:"pull_request"`
	Repository GithubRepository
	Sender     githubUser
}

func cleanupPatchTitle(t string) string {
	rv := ""
	keepers := []*unicode.RangeTable{unicode.Letter, unicode.Number}
	for _, r := range t {
		switch {
		case unicode.IsOneOf(keepers, r):
			rv = rv + string(r)
		case unicode.IsSpace(r):
			rv = rv + "-"
		}
	}
	return rv
}

func getGithubPatch(bug Bug, url string) {
	gres, err := http.Get(url)
	if gres != nil && gres.Body != nil {
		defer gres.Body.Close()
	}
	if err != nil || gres.StatusCode != 200 {
		if err == nil {
			err = errors.New(gres.Status)
		}
		log.Printf("Error getting pull request for bug %v from %v: %v",
			bug.Id, url, err)
		return
	}

	filename := "0001-" + cleanupPatchTitle(bug.Title) + ".patch"

	attid := randstring(8)
	dest := *cbfsUrl + bug.Id + "/" + attid + "/" + filename

	cr := &countingReader{r: gres.Body}

	req, err := http.NewRequest("PUT", dest, cr)
	if err != nil {
		log.Printf("Error storing patch in CBFS: %v", err)
		return
	}

	pres, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Error sending to CBFS: %v", err)
		return
	}
	defer pres.Body.Close()
	if pres.StatusCode != 201 {
		log.Printf("Error sending to CBFS: %v", err)
		return
	}

	att := Attachment{
		Id:          bug.Id + "-" + attid,
		BugId:       bug.Id,
		Type:        "attachment",
		Url:         dest,
		Size:        cr.n,
		ContentType: "text/plain",
		Filename:    filename,
		User:        *mailFrom,
		CreatedAt:   time.Now().UTC(),
	}

	err = db.Set("att-"+bug.Id+"-"+attid, 0, &att)
	if err != nil {
		log.Printf("Error storing attachment ref: %v", err)
		return
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

	tags := append(labels, "github", hookdata.Repository.Name)
	if hookdata.Issue.Pull.PatchURL != nil {
		tags = append(tags, "patch")
	}

	bug := Bug{
		Id:          fmt.Sprintf("bug-%v", id),
		Title:       hookdata.Issue.Title,
		Description: body,
		Creator:     *mailFrom,
		Status:      "inbox",
		Tags:        tags,
		Type:        "bug",
		CreatedAt:   hookdata.Issue.CreatedAt.UTC(),
		ModifiedAt:  hookdata.Issue.CreatedAt.UTC(),
		ModBy:       *mailFrom,
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

	for _, t := range tags {
		notifyTagAssigned(bug.Id, t, bug.Creator)
	}

	if hookdata.Issue.Pull.PatchURL != nil {
		go getGithubPatch(bug, *hookdata.Issue.Pull.PatchURL)
	}

	w.WriteHeader(204)
}

func serveGithubPullRequest(w http.ResponseWriter, r *http.Request) {
	hookdata := githubPullRequestHook{}
	err := json.Unmarshal([]byte(r.FormValue("payload")), &hookdata)

	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	log.Printf("Got pull request hook: %+v", hookdata)

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

	body := hookdata.PullRequest.Body
	if body != "" {
		body += "\n\n----\n"
	}
	body += hookdata.PullRequest.URL +
		"\nby github user: " + hookdata.PullRequest.User.Login +
		" ![g](" + hookdata.PullRequest.User.Avatar + "&s=64)"

	tags := []string{"github", "pull-request", hookdata.Repository.Name}

	bug := Bug{
		Id:          fmt.Sprintf("bug-%v", id),
		Title:       hookdata.PullRequest.Title,
		Description: body,
		Creator:     *mailFrom,
		Status:      "inbox",
		Tags:        tags,
		Type:        "bug",
		CreatedAt:   hookdata.PullRequest.CreatedAt.UTC(),
		ModifiedAt:  hookdata.PullRequest.CreatedAt.UTC(),
		ModBy:       *mailFrom,
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

	for _, t := range tags {
		notifyTagAssigned(bug.Id, t, bug.Creator)
	}

	go getGithubPatch(bug, hookdata.PullRequest.PatchURL)

	w.WriteHeader(204)
}
