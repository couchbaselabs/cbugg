package main

import (
	"encoding/json"
	"errors"
	"flag"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/couchbaselabs/go-couchbase"
	"github.com/gorilla/mux"
)

var templates *template.Template
var db *couchbase.Bucket
var NotFound = errors.New("not found")

var staticPath = flag.String("static", "static", "Path to the static content")

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

func serveHome(w http.ResponseWriter, r *http.Request) {
	args := map[string]interface{}{"group_level": 1, "stale": false}
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

func serveUserList(w http.ResponseWriter, r *http.Request) {
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

	rv := []string{}
	for _, r := range viewRes.Rows {
		if strings.Contains(r.Key, "@") {
			rv = append(rv, r.Key)
		}
	}
	sort.Strings(rv)

	w.Header().Set("Content-type", "application/json")
	mustEncode(w, rv)
}

func serveTagList(w http.ResponseWriter, r *http.Request) {
	args := map[string]interface{}{
		"group_level": 1,
	}

	viewRes := struct {
		Rows []struct {
			Key string
		}
	}{}

	err := db.ViewCustom("cbugg", "tags", args, &viewRes)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	rv := []string{}
	for _, r := range viewRes.Rows {
		rv = append(rv, r.Key)
	}
	sort.Strings(rv)

	w.Header().Set("Content-type", "application/json")
	mustEncode(w, rv)
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

	r.HandleFunc("/api/bug/{bugid}", serveBug).Methods("GET")
	r.HandleFunc("/api/bug/{bugid}",
		serveBugUpdate).Methods("POST").MatcherFunc(authRequired)
	r.HandleFunc("/api/bug/{bugid}", notAuthed).Methods("POST")

	r.HandleFunc("/api/bug/{bugid}/comments/", serveCommentList).Methods("GET")
	r.HandleFunc("/api/bug/{bugid}/comments/",
		serveNewComment).Methods("POST").MatcherFunc(authRequired)
	r.HandleFunc("/api/bug/{bugid}/comments/", notAuthed).Methods("POST")
	r.HandleFunc("/api/bug/{bugid}/comments/{commid}",
		serveDelComment).Methods("DELETE").MatcherFunc(authRequired)
	r.HandleFunc("/api/bug/{bugid}/comments/{commid}", notAuthed).Methods("DELETE")

	r.HandleFunc("/api/users/", serveUserList).Methods("GET")
	r.HandleFunc("/api/tags/", serveTagList).Methods("GET")

	r.HandleFunc("/api/state-counts", serveStateCounts)
	r.HandleFunc("/auth/login", serveLogin).Methods("POST")
	r.HandleFunc("/auth/logout", serveLogout).Methods("POST")
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/",
		http.FileServer(http.Dir(*staticPath))))

	if *quitPath != "" {
		r.HandleFunc(*quitPath, func(w http.ResponseWriter, r *http.Request) {
			log.Printf("Quitting per user request.")
			os.Exit(0)
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
