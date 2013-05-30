package main

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/couchbaselabs/go-couchbase"
	"github.com/couchbaselabs/sockjs-go/sockjs"
	"github.com/dustin/gomemcached"
	"github.com/gorilla/mux"
)

var db *couchbase.Bucket
var esHost = flag.String("elasticsearchHost", "localhost", "ElasticSearch hostname")
var esPort = flag.String("elasticsearchPort", "9200", "ElasticSearch port")
var esScheme = flag.String("elasticsearchScheme", "http", "ElasticSearch scheme")
var esIndex = flag.String("elasticsearchIndex", "cbugg", "ElasticSearch index")
var debugEs = flag.Bool("elasticsearchDebug", false, "ElasticSearch debugging")
var staticEtag = flag.String("staticEtag", "", "A static etag value.")
var NotFound = errors.New("not found")

var staticPath = flag.String("static", "static", "Path to the static content")

var bugStates = []BugState{
	{"inbox", 5, nil},
	{"new", 10, []string{"open", "inprogress", "resolved", "closed"}},
	{"open", 20, []string{"inprogress", "resolved", "closed"}},
	{"inprogress", 30, []string{"open", "resolved", "closed"}},
	{"resolved", 40, []string{"open", "closed"}},
	{"closed", 50, []string{"open"}},
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

func errorCode(err error) int {
	switch {
	case err == bugNotVisible:
		return 401
	case gomemcached.IsNotFound(err):
		return 404
	}
	return 500
}

func showError(w http.ResponseWriter, r *http.Request,
	msg string, code int) {
	log.Printf("Reporting error %v/%v", code, msg)
	http.Error(w, msg, code)
}

func mustEncode(w io.Writer, i interface{}) {
	if headered, ok := w.(http.ResponseWriter); ok {
		headered.Header().Set("Cache-Control", "no-cache")
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

func serveRecent(w http.ResponseWriter, r *http.Request) {
	output := []interface{}{}

	me := whoami(r)
	for _, r := range recentChanges.Slice() {
		co, ok := r.(changeEligible)
		if ok {
			c, err := co.changeObjectFor(me)
			if err == nil {
				output = append(output, c)
			}
		} else {
			log.Printf("%T isn't changeEligible", r)
		}
	}

	mustEncode(w, output)
}

func authRequired(r *http.Request, rm *mux.RouteMatch) bool {
	return whoami(r).Id != ""
}

func internalRequired(r *http.Request, rm *mux.RouteMatch) bool {
	return whoami(r).Internal
}

func adminRequired(r *http.Request, rm *mux.RouteMatch) bool {
	return whoami(r).Admin
}

func notAuthed(w http.ResponseWriter, r *http.Request) {
	showError(w, r, "You are not authorized", 401)
}

func RewriteURL(to string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = to
		h.ServeHTTP(w, r)
	})
}

type myFileHandler struct {
	h http.Handler
}

func (mfh myFileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if *staticEtag != "" {
		w.Header().Set("Etag", *staticEtag)
	}
	mfh.h.ServeHTTP(w, r)
}

func serveVersion(w http.ResponseWriter, r *http.Request) {
	mustEncode(w, map[string]string{"version": VERSION})
}

func main() {
	addr := flag.String("addr", ":8066", "http listen address")
	cbServ := flag.String("couchbase", "http://localhost:8091/",
		"URL to couchbase")
	cbBucket := flag.String("bucket", "cbugg", "couchbase bucket")
	secCookKey := flag.String("cookieKey", "youcandobetter",
		"The secure cookie auth code.")
	rebuildPath := flag.String("rebuildpath", "",
		"a path that will cause a rebuild and restart if requested")

	flag.Parse()

	initSecureCookie([]byte(*secCookKey))

	var err error

	r := mux.NewRouter()
	// Bug CRUD
	r.HandleFunc("/api/bug/", serveNewBug).Methods("POST").MatcherFunc(authRequired)
	r.HandleFunc("/api/bug/", notAuthed).Methods("POST")
	r.HandleFunc("/api/bug/", serveBugList).Methods("GET")

	r.HandleFunc("/api/bug/{bugid}", serveBug).Methods("GET")
	r.HandleFunc("/api/bug/{bugid}",
		serveBugUpdate).Methods("POST").MatcherFunc(authRequired)
	r.HandleFunc("/api/bug/{bugid}",
		serveBugDeletion).Methods("DELETE").MatcherFunc(adminRequired)
	r.HandleFunc("/api/bug/{bugid}", notAuthed).Methods("POST", "DELETE")

	// Bug history
	r.HandleFunc("/api/bug/{bugid}/history/", serveBugHistory).Methods("GET")

	// Attachments
	r.HandleFunc("/api/bug/{bugid}/attachments/",
		serveFileUpload).Methods("POST").MatcherFunc(authRequired)
	r.HandleFunc("/api/bug/{bugid}/attachments/", notAuthed).Methods("POST")
	r.HandleFunc("/api/bug/{bugid}/attachments/",
		serveAttachmentList).Methods("GET")
	r.HandleFunc("/api/bug/{bugid}/attachments/{attid}/{fn}",
		serveAttachment).Methods("GET")
	r.HandleFunc("/api/bug/{bugid}/attachments/{attid}/{fn}",
		serveHeadAttachment).Methods("HEAD")
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

	// Private bug visibility
	r.HandleFunc("/api/bug/{bugid}/viewer/add/",
		serveAddBugViewer).Methods("POST").MatcherFunc(internalRequired)
	r.HandleFunc("/api/bug/{bugid}/viewer/remove/",
		serveRemoveBugViewer).Methods("POST").MatcherFunc(internalRequired)

	// Bug Pinging
	r.HandleFunc("/api/bug/{bugid}/ping/",
		serveBugPing).Methods("POST").MatcherFunc(authRequired)
	r.HandleFunc("/api/bug/{bugid}/ping/",
		notAuthed).Methods("POST")
	// Or yourself, later.
	r.HandleFunc("/api/bug/{bugid}/remindme/",
		serveNewReminder).Methods("POST").MatcherFunc(authRequired)
	r.HandleFunc("/api/bug/{bugid}/remindme/",
		notAuthed).Methods("POST")

	// User list
	r.HandleFunc("/api/users/", serveUserList).Methods("GET")
	r.HandleFunc("/api/users/special/", serveSpecialUserList).Methods("GET")
	r.HandleFunc("/api/users/mod/", serveAdminUserMod).Methods("POST")

	// All about tags
	r.HandleFunc("/api/tags/", serveTagList).Methods("GET")
	r.HandleFunc("/api/tags/{tag}/", serveTagStates).Methods("GET")
	r.HandleFunc("/api/tags/{tag}/css/",
		serveTagCSSUpdate).Methods("POST").MatcherFunc(internalRequired)
	r.HandleFunc("/api/tags/{tag}/sub/",
		serveSubscribeTag).Methods("POST").MatcherFunc(authRequired)
	r.HandleFunc("/api/tags/{tag}/sub/",
		serveUnsubscribeTag).Methods("DELETE").MatcherFunc(authRequired)
	r.HandleFunc("/api/tags/{tag}/sub/", notAuthed).Methods("POST", "DELETE")
	r.HandleFunc("/tags.css", serveTagCSS).Methods("GET")

	r.HandleFunc("/api/recent/", serveRecent).Methods("GET")
	r.HandleFunc("/api/states/", serveStates).Methods("GET")

	// simple search
	r.HandleFunc("/api/search/", searchBugs).Methods("POST", "GET")
	// find similar bugs
	r.HandleFunc("/api/bugslike/", findSimilarBugs).Methods("POST")

	r.HandleFunc("/api/me/", serveMe).Methods("GET")
	r.HandleFunc("/api/me/prefs/",
		serveSetMyPrefs).Methods("POST").MatcherFunc(authRequired)
	r.HandleFunc("/api/me/prefs/", notAuthed).Methods("POST")
	r.HandleFunc("/api/me/token/",
		serveUserAuthToken).Methods("GET").MatcherFunc(authRequired)
	r.HandleFunc("/api/me/token/",
		serveUpdateUserAuthToken).Methods("POST").MatcherFunc(authRequired)
	r.HandleFunc("/api/me/token/", notAuthed)

	r.HandleFunc("/hooks/github/issue/", serveGithubIssue).Methods("POST")
	r.HandleFunc("/hooks/github/pull/", serveGithubPullRequest).Methods("POST")
	r.HandleFunc("/hooks/github/push/", serveGithubPush).Methods("POST")
	r.HandleFunc("/api/github/issue/fetch/",
		serveGithubIssueFetch).MatcherFunc(internalRequired).Methods("POST")

	r.HandleFunc("/api/state-counts", serveStateCounts)
	r.HandleFunc("/auth/login", serveLogin).Methods("POST")
	r.HandleFunc("/auth/logout", serveLogout).Methods("POST")

	r.HandleFunc("/api/version", serveVersion)

	sockjs.Install("/api/changes", ChangesHandler, sockjs.DefaultConfig)

	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/",
		myFileHandler{http.FileServer(http.Dir(*staticPath))}))

	// application pages
	appPages := []string{
		"/bug/",
		"/user/",
		"/state/",
		"/search/",
		"/statecounts/",
		"/tag/",
		"/tags/",
		"/navigator/",
		"/admin/",
		"/changes/",
		"/prefs/",
	}

	for _, p := range appPages {
		r.PathPrefix(p).Handler(RewriteURL("app.html",
			http.FileServer(http.Dir(*staticPath))))
	}

	if *rebuildPath != "" && *buildCmd != "" {
		ch := make(chan bool, 1)
		go rebuilder(ch)
		r.HandleFunc(*rebuildPath, func(w http.ResponseWriter, r *http.Request) {
			ch <- true
			w.WriteHeader(202)
		})
	}

	// Backwards compatibility
	r.Handle("/statecounts", http.RedirectHandler("/statecounts/", 302))
	r.Handle("/", http.RedirectHandler("/static/app.html", 302))

	http.Handle("/", r)

	db, err = dbConnect(*cbServ, *cbBucket)
	if err != nil {
		log.Fatalf("Error connecting to couchbase: %v", err)
	}

	go loadRecent()

	log.Printf("Listening on %v", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
