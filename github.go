package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"
)

var ghUser = flag.String("ghuser", "", "github username")
var ghPass = flag.String("ghpass", "", "github password")

var bugRefRE *regexp.Regexp

var failedToAdd = errors.New("Failed to add value")

func init() {
	bugRefRE = regexp.MustCompile(`[Cc][Bb][Uu][Gg][Gg]:\s*(clos\w+)?\s*((bug-\d+\s*)+)`)
}

type githubUser struct {
	Avatar     string `json:"avatar_url"`
	Login      string
	GravatarID string `json:"gravatar_id"`
}

type githubAuthor struct {
	Email    string
	Name     string
	Username string
}

type GithubRepository struct {
	Url  string `json:"html_url"`
	Name string
}

type GithubIssue struct {
	Title       string
	Body        string
	CreatedAt   time.Time `json:"created_at"`
	URL         string    `json:"html_url"`
	Labels      []struct{ Name string }
	User        githubUser
	EditURL     string `json:"url"`
	Comments    int
	CommentsURL string `json:"comments_url"`
	Pull        struct {
		PatchURL *string `json:"patch_url"`
	} `json:"pull_request"`
}

type GithubIssueComment struct {
	Body      string
	User      githubUser
	Timestamp time.Time `json:"updated_at"`
}

type githubIssueHook struct {
	Action     string
	Callpath   string `json:"hook_callpath"`
	Issue      GithubIssue
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

type githubCommit struct {
	Added     []string
	Modified  []string
	Removed   []string
	Author    githubAuthor
	Committer githubAuthor
	Id        string
	Message   string
	URL       string
	Timestamp time.Time
}

type githubPushHook struct {
	Commits    []githubCommit
	Repository GithubRepository
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
		return
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
		log.Printf("Error creating PATCH request: %v", err)
		return
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
		log.Printf("Could not close issue at %v: %v",
			editUrl, pres.Status)
	}
}

func getGithubPatch(bug Bug, url, email string) {
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
		User:        email,
		CreatedAt:   time.Now().UTC(),
	}

	err = db.Set("att-"+bug.Id+"-"+attid, 0, &att)
	if err != nil {
		log.Printf("Error storing attachment ref: %v", err)
		return
	}
}

func getGithubIssueComments(bugid string, url string) {
	ob := []GithubIssueComment{}
	err := getGithubObject(url, &ob)
	if err != nil {
		log.Printf("Error getting github issue: %v", err)
		return
	}

	for _, gc := range ob {
		id := "c-" + bugid + "-" + time.Now().UTC().Format(time.RFC3339Nano)

		body := gc.Body
		theuser := findEmailByMD5(gc.User.GravatarID)
		if theuser == "" {
			theuser = *mailFrom
			body += "\nby github user: " + gc.User.Login +
				" ![g](" + gc.User.Avatar + "&s=64)"
		}

		c := Comment{
			Id:        id,
			BugId:     bugid,
			Type:      "comment",
			User:      theuser,
			Text:      body,
			CreatedAt: gc.Timestamp,
		}

		_, err := db.Add(c.Id, 0, c)
		if err != nil {
			log.Printf("Error adding new comment: %v", err)
		}
	}
}

func makeIssueFromGithub(issue GithubIssue, repository GithubRepository) (Bug, error) {
	log.Printf("Creating github issue:\n%v\n%v", issue, repository)

	id, err := newBugId()
	if err != nil {
		return Bug{}, err
	}

	body := issue.Body
	if issue.Body != "" {
		body += "\n\n----\n"
	}
	body += issue.URL +
		"\nby github user: " + issue.User.Login +
		" ![g](" + issue.User.Avatar + "&s=64)"

	labels := []string{}
	for _, l := range issue.Labels {
		labels = append(labels, strings.ToLower(l.Name))
	}

	tags := append(labels, repository.Name)
	if issue.Pull.PatchURL != nil {
		tags = append(tags, "patch")
	}

	subs := []string{}
	originator := findEmailByMD5(issue.User.GravatarID)
	if originator == "" {
		originator = *mailFrom
	} else {
		subs = []string{originator}
	}

	bug := Bug{
		Id:          fmt.Sprintf("bug-%v", id),
		Title:       issue.Title,
		Description: body,
		Subscribers: subs,
		Creator:     originator,
		Status:      "inbox",
		Tags:        tags,
		Type:        "bug",
		CreatedAt:   issue.CreatedAt.UTC(),
		ModifiedAt:  issue.CreatedAt.UTC(),
		ModBy:       originator,
	}

	added, err := db.Add(bug.Id, 0, bug)
	if err != nil {
		return bug, err
	}
	if !added {
		return bug, fmt.Errorf("Bug collision on %v", bug.Id)
	}

	for _, t := range tags {
		notifyTagAssigned(bug.Id, t, bug.Creator)
	}

	if issue.Pull.PatchURL != nil {
		go getGithubPatch(bug, *issue.Pull.PatchURL, originator)
	}

	if issue.Comments > 0 {
		go getGithubIssueComments(bug.Id, issue.CommentsURL)
	}

	go closeGithubIssue(bug, issue.CommentsURL, issue.EditURL)

	notifyBugChange(bug.Id, "", originator)

	return bug, nil
}

func serveGithubIssue(w http.ResponseWriter, r *http.Request) {
	hookdata := githubIssueHook{}
	err := json.Unmarshal([]byte(r.FormValue("payload")), &hookdata)

	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	log.Printf("Got hook: %+v", hookdata)

	if hookdata.Action != "opened" {
		log.Printf("Something other than 'opened' happened: %q, skipping",
			hookdata.Action)
		w.WriteHeader(204)
		return
	}

	_, err = makeIssueFromGithub(hookdata.Issue, hookdata.Repository)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	w.WriteHeader(204)
}

func getGithubObject(path string, ob interface{}) error {
	url := path
	if !strings.HasPrefix(path, "https:") {
		url = "https://api.github.com/" + path
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(*ghUser, *ghPass)
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	d := json.NewDecoder(res.Body)
	return d.Decode(ob)
}

func serveGithubIssueFetch(w http.ResponseWriter, r *http.Request) {
	repoName := r.FormValue("repo")
	repo := GithubRepository{}
	err := getGithubObject("repos/"+repoName, &repo)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	issueId := r.FormValue("issue")
	if issueId == "" {
		issues := []GithubIssue{}
		err = getGithubObject("repos/"+repoName+"/issues", &issues)
		if err != nil {
			showError(w, r, err.Error(), 500)
			return
		}

		for _, issue := range issues {
			_, err := makeIssueFromGithub(issue, repo)
			if err != nil {
				showError(w, r, err.Error(), 500)
				return
			}
		}

		w.WriteHeader(204)
	} else {
		issue := GithubIssue{}
		err = getGithubObject("repos/"+repoName+"/issues/"+issueId,
			&issue)
		if err != nil {
			showError(w, r, err.Error(), 500)
			return
		}

		bug, err := makeIssueFromGithub(issue, repo)
		if err != nil {
			showError(w, r, err.Error(), 500)
			return
		}

		http.Redirect(w, r, bug.Url(), 303)
	}
}

func serveGithubPullRequest(w http.ResponseWriter, r *http.Request) {
	hookdata := githubPullRequestHook{}
	err := json.Unmarshal([]byte(r.FormValue("payload")), &hookdata)

	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	log.Printf("Got pull request hook: %+v", hookdata)

	if hookdata.Action != "opened" {
		log.Printf("Something other than 'opened' happened: %q, skipping",
			hookdata.Action)
		w.WriteHeader(204)
		return
	}

	id, err := newBugId()
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	body := hookdata.PullRequest.Body
	if hookdata.PullRequest.Body != "" {
		body += "\n\n----\n"
	}
	body += hookdata.PullRequest.URL +
		"\nby github user: " + hookdata.PullRequest.User.Login +
		" ![g](" + hookdata.PullRequest.User.Avatar + "&s=64)"

	tags := []string{"pull-request", hookdata.Repository.Name}

	subs := []string{}
	originator := findEmailByMD5(hookdata.PullRequest.User.GravatarID)
	if originator == "" {
		originator = *mailFrom
	} else {
		subs = []string{originator}
	}

	bug := Bug{
		Id:          fmt.Sprintf("bug-%v", id),
		Title:       hookdata.PullRequest.Title,
		Description: body,
		Creator:     originator,
		Subscribers: subs,
		Status:      "inbox",
		Tags:        tags,
		Type:        "bug",
		CreatedAt:   hookdata.PullRequest.CreatedAt.UTC(),
		ModifiedAt:  hookdata.PullRequest.CreatedAt.UTC(),
		ModBy:       originator,
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

	go getGithubPatch(bug, hookdata.PullRequest.PatchURL, originator)

	go closeGithubIssue(bug, hookdata.PullRequest.CommentsURL,
		hookdata.PullRequest.EditURL)

	w.WriteHeader(204)
}

type githubCBRef struct {
	bugid  string
	closed bool
}

func extractRefsFromGithub(msg string) []githubCBRef {
	rv := []githubCBRef{}

	for _, l := range strings.Split(msg, "\n") {
		matches := bugRefRE.FindAllStringSubmatch(l, 100000)

		for _, x := range matches {
			for _, b := range strings.Split(x[2], " ") {
				rv = append(rv, githubCBRef{
					b, strings.HasPrefix(strings.ToLower(x[1]), "clos"),
				})
			}
		}
	}

	return rv
}

func markRefBug(commit, bugid string) error {
	added, err := db.Add("mark-"+commit+"."+bugid, 900, "")
	if err != nil {
		return err
	}
	if !added {
		return failedToAdd
	}
	return nil
}

func refBug(hookdata githubPushHook, commit githubCommit, ref githubCBRef) {
	bugid := ref.bugid

	if err := markRefBug(commit.Id, bugid); err != nil {
		log.Printf("Could not mark ref bug for %v/%v: %v",
			commit.Id, bugid, err)
		return
	}

	me, _ := getUser(commit.Author.Email)
	me.Id = commit.Author.Email // Update the author for when getUser fails

	if _, err := getBugFor(bugid, me); err != nil {
		return
	}

	commentMsg := "Commit [" + commit.Id + "](" + commit.URL + ")\n\n" +
		"```\n" + commit.Message + "\n```\n"

	id := "c-" + bugid + "-" + time.Now().UTC().Format(time.RFC3339Nano)

	c := Comment{
		Id:        id,
		BugId:     bugid,
		Type:      "comment",
		User:      me.Id,
		Text:      commentMsg,
		CreatedAt: time.Now().UTC(),
	}

	added, err := db.Add(c.Id, 0, c)
	if err != nil {
		log.Printf("Error adding new comment from github: %v", err)
		return
	}
	if !added {
		log.Printf("Failed to add new comment (this is a bug)")
		return
	}

	notifyComment(c)

	if ref.closed {
		updateBug(bugid, "status", "resolved", me)
	}
}

func processPushHook(hookdata githubPushHook) {
	for _, commit := range hookdata.Commits {
		for _, ref := range extractRefsFromGithub(commit.Message) {
			refBug(hookdata, commit, ref)
		}
	}
}

func serveGithubPush(w http.ResponseWriter, r *http.Request) {
	hookdata := githubPushHook{}
	err := json.Unmarshal([]byte(r.FormValue("payload")), &hookdata)

	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	log.Printf("Got push hook: %+v", hookdata)

	go processPushHook(hookdata)

	w.WriteHeader(204)
}
