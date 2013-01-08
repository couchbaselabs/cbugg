package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/couchbaselabs/go-couchbase"
)

var templates *template.Template
var db *couchbase.Bucket

var staticPath = flag.String("static", "static", "Path to the static content")

func init() {
	var err error
	templates, err = template.ParseGlob(fmt.Sprintf("html/%c.html", '*'))
	if err != nil {
		panic("Couldn't parse templates.")
	}
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/bug/", serveBugPath)
	http.Handle("/static/", http.StripPrefix("/static/",
		http.FileServer(http.Dir(*staticPath))))
}

func newBugId() (uint64, error) {
	return db.Incr(".bugid", 1, 0, 0)
}

func showError(w http.ResponseWriter, r *http.Request,
	msg string, code int) {
	http.Error(w, msg, code)
}

func serveNewBug(w http.ResponseWriter, r *http.Request) {
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
		Creator:     "me", // XXX: Better
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

func serveBug(w http.ResponseWriter, r *http.Request, id string) {
	bug := Bug{}
	err := db.Get(id, &bug)
	if err != nil {
		showError(w, r, err.Error(), 404)
		return
	}
	bug.Id = id

	templates.ExecuteTemplate(w, "bug.html", bug)
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

	templates.ExecuteTemplate(w, "buglist.html", res)
}

func minusPrefix(s, prefix string) string {
	return s[len(prefix):]
}

func serveBugPath(w http.ResponseWriter, r *http.Request) {
	path := minusPrefix(r.URL.Path, "/bug/")

	switch {
	case r.Method == "POST" && path == "":
		serveNewBug(w, r)
	case r.Method == "GET" && path == "":
		serveBugList(w, r)
	case r.Method == "GET":
		serveBug(w, r, path)
	default:
		showError(w, r, "Can't service "+r.Method+":"+path,
			http.StatusMethodNotAllowed)
	}
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

func initCb(serv, bucket string) {
	var err error
	db, err = couchbase.GetBucket(serv, "default", bucket)
	if err != nil {
		log.Fatalf("Error connecting to couchbase: %v", err)
	}
}

func main() {
	addr := flag.String("addr", ":8066", "http listen address")
	cbServ := flag.String("couchbase", "http://localhost:8091/",
		"URL to couchbase")
	cbBucket := flag.String("bucket", "cbugg", "couchbase bucket")

	flag.Parse()

	initCb(*cbServ, *cbBucket)

	log.Printf("Listening on %v", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
