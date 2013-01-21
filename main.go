package main

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/couchbaselabs/go-couchbase"
	"github.com/gorilla/mux"
)

var db *couchbase.Bucket
var esHost = flag.String("elasticsearchHost", "localhost", "ElasticSearch hostname")
var esPort = flag.String("elasticsearchPort", "9200", "ElasticSearch port")
var esScheme = flag.String("elasticsearchScheme", "http", "ElasticSearch scheme")
var esIndex = flag.String("elasticsearchIndex", "cbugg", "ElasticSearch index")
var NotFound = errors.New("not found")

var staticPath = flag.String("static", "static", "Path to the static content")

var bugStates = []BugState{
	{"inbox", 5, nil},
	{"new", 10, []string{"open", "resolved", "closed"}},
	{"open", 20, []string{"resolved", "closed"}},
	{"resolved", 30, []string{"open", "closed"}},
	{"closed", 40, []string{"open"}},
}

func checkLastModified(w http.ResponseWriter, r *http.Request, modtime time.Time) bool {
	if modtime.IsZero() {
		return false
	}

	// The Date-Modified header truncates sub-second precision, so
	// use mtime < t+1s instead of mtime <= t to check for unmodified.
	if t, err := time.Parse(http.TimeFormat,
		r.Header.Get("If-Modified-Since")); err == nil &&
		modtime.Before(t.Add(time.Second)) {

		h := w.Header()
		delete(h, "Content-Type")
		delete(h, "Content-Length")
		w.WriteHeader(http.StatusNotModified)
		return true
	}
	w.Header().Set("Last-Modified", modtime.UTC().Format(http.TimeFormat))
	return false
}
func showError(w http.ResponseWriter, r *http.Request,
	msg string, code int) {
	log.Printf("Reporting error %v/%v", code, msg)
	http.Error(w, msg, code)
}

func mustEncode(w io.Writer, i interface{}) {
	if headered, ok := w.(http.ResponseWriter); ok {
		headered.Header().Set("Content-type", "application/json")
	}

	e := json.NewEncoder(w)
	if err := e.Encode(i); err != nil {
		panic(err)
	}
}

func serveStateCounts(w http.ResponseWriter, r *http.Request) {
	args := map[string]interface{}{"group_level": 1, "stale": false}
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

func serveStates(w http.ResponseWriter, r *http.Request) {
	mustEncode(w, bugStates)
}

func serveUserList(w http.ResponseWriter, r *http.Request) {
	rv := []string{}

	if whoami(r) != "" {
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

func serveRecent(w http.ResponseWriter, r *http.Request) {
	args := map[string]interface{}{
		"descending": true,
		"limit":      20,
	}

	viewRes := struct {
		Rows []struct {
			ID    string
			Key   string
			Value struct {
				Actor  string
				Action string
				BugId  string
			}
		}
	}{}

	err := db.ViewCustom("cbugg", "changes", args, &viewRes)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	bugs := map[string]Bug{}

	for _, r := range viewRes.Rows {
		bugs[r.Value.BugId] = Bug{}
	}

	for k := range bugs {
		b, err := getBug(k)
		if err == nil {
			bugs[k] = b
		}
	}

	type OutType struct {
		Time   string `json:"time"`
		User   Email  `json:"user"`
		Action string `json:"action"`
		BugId  string `json:"bugid"`
		Status string `json:"status"`
		Title  string `json:"title"`
	}

	output := []OutType{}

	for _, r := range viewRes.Rows {
		output = append(output,
			OutType{r.Key,
				Email(r.Value.Actor),
				r.Value.Action,
				r.Value.BugId,
				bugs[r.Value.BugId].Status,
				bugs[r.Value.BugId].Title,
			})
	}

	mustEncode(w, output)
}

func authRequired(r *http.Request, rm *mux.RouteMatch) bool {
	return whoami(r) != ""
}

func notAuthed(w http.ResponseWriter, r *http.Request) {
	showError(w, r, "You are not authenticated", 401)
}

func main() {
	addr := flag.String("addr", ":8066", "http listen address")
	cbServ := flag.String("couchbase", "http://localhost:8091/",
		"URL to couchbase")
	cbBucket := flag.String("bucket", "cbugg", "couchbase bucket")
	secCookKey := flag.String("cookieKey", "youcandobetter",
		"The secure cookie auth code.")
	quitPath := flag.String("quitpath", "",
		"a path that will shut down the service if requested")

	flag.Parse()

	initSecureCookie([]byte(*secCookKey))

	var err error

	r := mux.NewRouter()
	// Bugs are fancy
	r.HandleFunc("/api/bug/", serveNewBug).Methods("POST").MatcherFunc(authRequired)
	r.HandleFunc("/api/bug/", notAuthed).Methods("POST")
	r.HandleFunc("/api/bug/", serveBugList).Methods("GET")

	r.HandleFunc("/api/bug/{bugid}/history/", serveBugHistory).Methods("GET")

	r.HandleFunc("/api/bug/{bugid}", serveBug).Methods("GET")
	r.HandleFunc("/api/bug/{bugid}",
		serveBugUpdate).Methods("POST").MatcherFunc(authRequired)
	r.HandleFunc("/api/bug/{bugid}", notAuthed).Methods("POST")
	// short url for bug
	r.HandleFunc("/bug/{bugid}", serveBugRedirect).Methods("GET")

	r.HandleFunc("/api/bug/{bugid}/attachments/",
		serveFileUpload).Methods("POST").MatcherFunc(authRequired)
	r.HandleFunc("/api/bug/{bugid}/attachments/", notAuthed).Methods("POST")
	r.HandleFunc("/api/bug/{bugid}/attachments/",
		serveAttachmentList).Methods("GET")
	r.HandleFunc("/api/bug/{bugid}/attachments/{attid}/{fn}",
		serveAttachment).Methods("GET")
	r.HandleFunc("/api/bug/{bugid}/attachments/{attid}/",
		serveDeleteAttachment).Methods("DELETE").MatcherFunc(authRequired)
	r.HandleFunc("/api/bug/{bugid}/attachments/{attid}/",
		notAuthed).Methods("DELETE")

	// comments
	r.HandleFunc("/api/bug/{bugid}/comments/", serveCommentList).Methods("GET")
	r.HandleFunc("/api/bug/{bugid}/comments/",
		serveNewComment).Methods("POST").MatcherFunc(authRequired)
	r.HandleFunc("/api/bug/{bugid}/comments/", notAuthed).Methods("POST")
	r.HandleFunc("/api/bug/{bugid}/comments/{commid}",
		serveDelComment).Methods("DELETE").MatcherFunc(authRequired)
	r.HandleFunc("/api/bug/{bugid}/comments/{commid}", notAuthed).Methods("DELETE")
	r.HandleFunc("/api/bug/{bugid}/comments/{commid}/undel",
		serveUnDelComment).Methods("POST").MatcherFunc(authRequired)
	r.HandleFunc("/api/bug/{bugid}/comments/{commid}/undel", notAuthed).Methods("POST")
	r.HandleFunc("/api/bug/{bugid}/comments/{commid}",
		serveCommentUpdate).Methods("POST").MatcherFunc(authRequired)
	// Bug subscriptions
	r.HandleFunc("/api/bug/{bugid}/sub/",
		serveSubscribeBug).Methods("POST").MatcherFunc(authRequired)
	r.HandleFunc("/api/bug/{bugid}/sub/",
		serveUnsubscribeBug).Methods("DELETE").MatcherFunc(authRequired)
	r.HandleFunc("/api/bug/{bugid}/sub/", notAuthed)

	// Bug Pinging
	r.HandleFunc("/api/bug/{bugid}/ping/",
		serveBugPing).Methods("POST").MatcherFunc(authRequired)
	r.HandleFunc("/api/bug/{bugid}/ping/",
		notAuthed).Methods("POST")

	r.HandleFunc("/api/users/", serveUserList).Methods("GET")
	r.HandleFunc("/api/tags/", serveTagList).Methods("GET")
	r.HandleFunc("/api/recent/", serveRecent).Methods("GET")
	r.HandleFunc("/api/states/", serveStates).Methods("GET")

	r.HandleFunc("/api/search/", searchBugs).Methods("POST")

	r.HandleFunc("/api/me/", serveMe).Methods("GET")
	r.HandleFunc("/api/me/prefs/",
		serveSetMyPrefs).Methods("POST").MatcherFunc(authRequired)
	r.HandleFunc("/api/me/prefs/", notAuthed).Methods("POST")
	r.HandleFunc("/api/me/token/",
		serveUserAuthToken).Methods("GET").MatcherFunc(authRequired)
	r.HandleFunc("/api/me/token/",
		serveUpdateUserAuthToken).Methods("POST").MatcherFunc(authRequired)
	r.HandleFunc("/api/me/token/", notAuthed)

	r.HandleFunc("/api/state-counts", serveStateCounts)
	r.HandleFunc("/auth/login", serveLogin).Methods("POST")
	r.HandleFunc("/auth/logout", serveLogout).Methods("POST")
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/",
		http.FileServer(http.Dir(*staticPath))))

	if *quitPath != "" {
		r.HandleFunc(*quitPath, func(w http.ResponseWriter, r *http.Request) {
			time.AfterFunc(time.Second, func() {
				log.Printf("Quitting per user request.")
				os.Exit(0)
			})
			w.WriteHeader(202)
		})
	}

	r.Handle("/", http.RedirectHandler("/static/app.html", 302))

	http.Handle("/", r)

	db, err = dbConnect(*cbServ, *cbBucket)
	if err != nil {
		log.Fatalf("Error connecting to couchbase: %v", err)
	}

	log.Printf("Listening on %v", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
