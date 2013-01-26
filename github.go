package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode"
)

var ghUser = flag.String("ghuser", "", "github username")
var ghPass = flag.String("ghpass", "", "github password")

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
		Title       string
		Body        string
		CreatedAt   time.Time `json:"created_at"`
		URL         string    `json:"html_url"`
		Labels      []struct{ Name string }
		User        githubUser
		EditURL     string `json:"url"`
		CommentsURL string `json:"comments_url"`
		Pull        struct {
			PatchURL *string `json:"patch_url"`
		} `json:"pull_request"`
	}
	Repository GithubRepository
}

type githubPullRequestHook struct {
	Action      string
	Callpath    string `json:"hook_callpath"`
	PullRequest struct {
		Title       string
		Body        string    `json:"body"`
		CreatedAt   time.Time `json:"created_at"`
		PatchURL    string    `json:"patch_url"`
		URL         string    `json:"html_url"`
		EditURL     string    `json:"issue_url"`
		CommentsURL string    `json:"comments_url"`
		User        githubUser
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

func closeGithubIssue(bug Bug, commentUrl, editUrl string) {
	if *ghUser == "" || *ghPass == "" {
		log.Printf("Github user not configured, not closing")
	}

	buf := &bytes.Buffer{}
	mustEncode(buf, map[string]interface{}{
		"body": "Issue has been moved to " + *baseURL + bug.Url(),
	})

	creq, err := http.NewRequest("POST", commentUrl,
		bytes.NewReader(buf.Bytes()))
	if err != nil {
		log.Printf("Error creating comment post: %v", err)
		return
	}
	creq.SetBasicAuth(*ghUser, *ghPass)
	creq.Header.Set("Content-Type", "application/json")
	creq.ContentLength = int64(buf.Len())

	cres, err := http.DefaultClient.Do(creq)
	if err != nil {
		log.Printf("Error posting close comment to %v: %v",
			commentUrl, err)
		return
	}
	defer cres.Body.Close()
	if cres.StatusCode != 201 {
		log.Printf("Could not create new comment: %v", cres.Status)
		return
	}

	closeBody := []byte(`{"state":"closed"}`)
	preq, err := http.NewRequest("PATCH", editUrl,
		bytes.NewReader(closeBody))
	if err != nil {
		log.Printf("Error creating patch URL: %v", err)
	}
	preq.SetBasicAuth(*ghUser, *ghPass)
	preq.Header.Set("Content-Type", "application/json")
	preq.ContentLength = int64(len(closeBody))

	pres, err := http.DefaultClient.Do(preq)
	if err != nil {
		log.Printf("Error patching to close at %v: %v",
			editUrl, err)
		return
	}
	defer pres.Body.Close()
	if pres.StatusCode != 200 {
		log.Printf("Could not close issue: %v", pres.Status)
		return
	}
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
		labels = append(labels, strings.ToLower(l.Name))
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

	go closeGithubIssue(bug, hookdata.Issue.CommentsURL,
		hookdata.Issue.EditURL)

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

	go closeGithubIssue(bug, hookdata.PullRequest.CommentsURL,
		hookdata.PullRequest.EditURL)

	w.WriteHeader(204)
}
