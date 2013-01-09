package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
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

func serveBug(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["bugid"]
	bug := Bug{}
	err := db.Get(id, &bug)
	if err != nil {
		showError(w, r, err.Error(), 404)
		return
	}
	bug.Id = id

	templates.ExecuteTemplate(w, "bug.html", bug)
}

func serveBugUpdate(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["bugid"]
	r.ParseForm()
	val := r.FormValue("value")

	err := db.Update(id, 0, func(current []byte) ([]byte, error) {
		if len(current) == 0 {
			return nil, NotFound
		}
		bug := Bug{}
		err := json.Unmarshal(current, &bug)
		if err != nil {
			return nil, err
		}
		bug.ModifiedAt = time.Now().UTC()

		switch r.FormValue("id") {
		case "description":
			bug.Description = val
		case "title":
			bug.Title = val
		case "status":
			bug.Status = val
		case "tags":
			bug.Tags = strings.FieldsFunc(val,
				func(r rune) bool {
					switch r {
					case ',', ' ':
						return true
					}
					return false
				})
			val = strings.Join(bug.Tags, ", ")
		default:
			return nil, fmt.Errorf("Unhandled id: %v",
				r.FormValue("id"))
		}

		return json.Marshal(&bug)
	})

	if err != nil {
		http.Error(w, err.Error(), 400)
	}

	w.Write([]byte(val))
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

	flag.Parse()

	var err error
	templates, err = template.ParseGlob("html/*.html")
	if err != nil {
		panic("Couldn't parse templates.")
	}

	r := mux.NewRouter()
	// Bugs are fancy
	r.HandleFunc("/bug/", serveNewBug).Methods("POST")
	r.HandleFunc("/bug/", serveBugList).Methods("GET")
	r.HandleFunc("/bug/{bugid}", serveBug).Methods("GET")
	r.HandleFunc("/bug/{bugid}", serveBugUpdate).Methods("POST")
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/",
		http.FileServer(http.Dir(*staticPath))))
	r.HandleFunc("/", serveHome)

	http.Handle("/", r)

	db, err = dbConnect(*cbServ, *cbBucket)
	if err != nil {
		log.Fatalf("Error connecting to couchbase: %v", err)
	}

	log.Printf("Listening on %v", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
