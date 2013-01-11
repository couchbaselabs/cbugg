package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/couchbaselabs/go-couchbase"
	"github.com/gorilla/mux"
)

var templates *template.Template
var db *couchbase.Bucket
var NotFound = errors.New("not found")

var staticPath = flag.String("static", "static", "Path to the static content")

func newBugId() (uint64, error) {
	return db.Incr(".bugid", 1, 0, 0)
}

func showError(w http.ResponseWriter, r *http.Request,
	msg string, code int) {
	log.Printf("Reporting error %v/%v", code, msg)
	http.Error(w, msg, code)
}

func mustEncode(w io.Writer, i interface{}) {
	e := json.NewEncoder(w)
	if err := e.Encode(i); err != nil {
		panic(err)
	}
}

func serveNewBug(w http.ResponseWriter, r *http.Request) {
	email := whoami(r)
	if email == "" {
		showError(w, r, "You are not authenticated", 401)
		return
	}

	id, err := newBugId()
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	bug := Bug{
		Id:          fmt.Sprintf("bug-%v", id),
		Title:       r.FormValue("title"),
		Description: r.FormValue("description"),
		Status:      "new",
		Creator:     email,
		Tags:        r.Form["tag"],
		Type:        "bug",
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

func serveBug(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["bugid"]
	bug := APIBug{}
	err := db.Get(id, &bug)
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
		"bug":     bug,
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
	if email == "" {
		showError(w, r, "You are not authenticated", 401)
		return
	}

	id := mux.Vars(r)["bugid"]
	r.ParseForm()
	val := r.FormValue("value")
	rval := []byte{}

	historyKey := id + "-" + time.Now().UTC().Format(time.RFC3339Nano)

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
			ModifiedAt: bug.ModifiedAt,
			ModType:    r.FormValue("id"),
			ModBy:      email,
		}
		if history.ModifiedAt.IsZero() {
			history.ModifiedAt = bug.CreatedAt
		}

		switch r.FormValue("id") {
		case "description":
			history.Description = bug.Description
			bug.Description = val
		case "title":
			history.Title = bug.Title
			bug.Title = val
		case "status":
			history.Status = bug.Status
			bug.Status = val
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

		// This is a side-effect in a CAS operation.  It's is
		// correct and safe because the side effect is the
		// creation of a document that is only used and
		// correct relative to the final value from the CAS.
		err = db.Set(historyKey, 0, &history)
		if err != nil {
			return nil, err
		}

		bug.ModifiedAt = time.Now().UTC()
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

	if err != nil {
		http.Error(w, err.Error(), 400)
	}

	w.Write([]byte(rval))
}

func serveBugList(w http.ResponseWriter, r *http.Request) {
	args := map[string]interface{}{
		"reduce": false,
	}

	if r.FormValue("state") != "" {
		st := r.FormValue("state")
		args["start_key"] = []interface{}{st}
		args["end_key"] = []interface{}{st, map[string]string{}}
	}

	res, err := db.View("cbugg", "by_state", args)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	jres, err := json.Marshal(res.Rows)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}
	w.Write(jres)
}

func serveStateCounts(w http.ResponseWriter, r *http.Request) {
	args := map[string]interface{}{"group_level": 1}
	states, err := db.View("cbugg", "by_state", args)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	statemap := map[string]interface{}{}
	for _, row := range states.Rows {
		statemap[row.Key.([]interface{})[0].(string)] = row.Value
	}

	jres, err := json.Marshal(map[string]interface{}{
		"states": statemap,
	})
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}
	w.Write(jres)
}

func serveNewComment(w http.ResponseWriter, r *http.Request) {
	email := whoami(r)
	if email == "" {
		showError(w, r, "You are not authenticated", 401)
		return
	}

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

func serveDelComment(w http.ResponseWriter, r *http.Request) {
	email := whoami(r)
	if email == "" {
		showError(w, r, "You are not authenticated", 401)
		return
	}

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

		comment.Deleted = true

		return json.Marshal(comment)
	})

	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	w.WriteHeader(204)
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	args := map[string]interface{}{"group_level": 1}
	states, err := db.View("cbugg", "by_state", args)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	templates.ExecuteTemplate(w, "index.html",
		map[string]interface{}{
			"states": states,
		})
}

func main() {
	addr := flag.String("addr", ":8066", "http listen address")
	cbServ := flag.String("couchbase", "http://localhost:8091/",
		"URL to couchbase")
	cbBucket := flag.String("bucket", "cbugg", "couchbase bucket")
	secCookKey := flag.String("cookieKey", "youcandobetter",
		"The secure cookie auth code.")

	flag.Parse()

	initSecureCookie([]byte(*secCookKey))

	var err error

	r := mux.NewRouter()
	// Bugs are fancy
	r.HandleFunc("/api/bug/", serveNewBug).Methods("POST")
	r.HandleFunc("/api/bug/", serveBugList).Methods("GET")
	r.HandleFunc("/api/bug/{bugid}", serveBug).Methods("GET")
	r.HandleFunc("/api/bug/{bugid}", serveBugUpdate).Methods("POST")
	r.HandleFunc("/api/bug/{bugid}/comments/", serveCommentList).Methods("GET")
	r.HandleFunc("/api/bug/{bugid}/comments/", serveNewComment).Methods("POST")
	r.HandleFunc("/api/bug/{bugid}/comments/{commid}",
		serveDelComment).Methods("DELETE")
	r.HandleFunc("/api/state-counts", serveStateCounts)
	r.HandleFunc("/auth/login", serveLogin).Methods("POST")
	r.HandleFunc("/auth/logout", serveLogout).Methods("POST")
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/",
		http.FileServer(http.Dir(*staticPath))))
	r.Handle("/", http.RedirectHandler("/static/app.html", 302))

	http.Handle("/", r)

	db, err = dbConnect(*cbServ, *cbBucket)
	if err != nil {
		log.Fatalf("Error connecting to couchbase: %v", err)
	}

	log.Printf("Listening on %v", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
