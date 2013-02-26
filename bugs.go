package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/couchbaselabs/go-couchbase"
	"github.com/dustin/gomemcached"
	"github.com/gorilla/mux"
)

var bugNotVisible = errors.New("this bug is not visible")

func newBugId() (uint64, error) {
	return db.Incr(".bugid", 1, 0, 0)
}

func serveNewBug(w http.ResponseWriter, r *http.Request) {
	if len(r.FormValue("title")) < 4 {
		showError(w, r, "Title is too short", 400)
		return
	}

	me := whoami(r)

	id, err := newBugId()
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	now := time.Now().UTC()

	bug := Bug{
		Id:          fmt.Sprintf("bug-%v", id),
		Title:       r.FormValue("title"),
		Description: r.FormValue("description"),
		Status:      "inbox",
		Creator:     me.Id,
		Tags:        r.Form["tag"],
		Type:        "bug",
		Subscribers: []string{me.Id},
		CreatedAt:   now,
		ModifiedAt:  now,
		ModBy:       me.Id,
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

	for _, t := range r.Form["tag"] {
		notifyTagAssigned(bug.Id, t, me.Id)
	}

	notifyBugChange(bug.Id, "", me.Id)

	http.Redirect(w, r, bug.Url(), 303)
}

func getBug(bugid string) (Bug, error) {
	bug := Bug{}
	err := db.Get(bugid, &bug)
	return bug, err
}

func getBugFor(bugid string, u User) (Bug, error) {
	bug, err := getBug(bugid)
	if err == nil && !isVisible(bug, u) {
		return Bug{}, bugNotVisible
	}
	return bug, err
}

func getBugOrDisplayErr(bugid string, u User,
	w http.ResponseWriter, r *http.Request) (Bug, error) {

	bug, err := getBugFor(bugid, u)
	if err != nil {
		showError(w, r, err.Error(), errorCode(err))
	}
	return bug, err
}

func serveBugHistory(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["bugid"]

	if _, err := getBugOrDisplayErr(id, whoami(r), w, r); err != nil {
		return
	}

	hist, err := getBugHistory(id)
	if err != nil {
		showError(w, r, err.Error(), 404)
		return
	}

	mustEncode(w, hist)
}

func serveBug(w http.ResponseWriter, r *http.Request) {
	bug, err := getBugOrDisplayErr(mux.Vars(r)["bugid"], whoami(r), w, r)
	if err != nil {
		return
	}

	if !checkLastModified(w, r, bug.ModifiedAt) {
		mustEncode(w, APIBug(bug))
	}
}

type BugHistoryItem struct {
	Key       string
	Timestamp time.Time
	ModInfo   map[string]interface{}
}

func getBugHistory(id string) ([]BugHistoryItem, error) {
	args := map[string]interface{}{
		"stale":     false,
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
			h["by"] = Email(s)
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

func newTags(o, n string) []string {
	oldm := map[string]bool{}

	for _, s := range strings.Split(o, ",") {
		oldm[s] = true
	}

	out := []string{}
	for _, s := range strings.Split(n, ",") {
		if !oldm[s] {
			out = append(out, s)
		}
	}
	return out
}

func updateBug(id, field, val string, me User) ([]byte, error) {
	now := time.Now().UTC()
	rval := []byte{}
	var oldval string

	historyKey := id + "-" + now.Format(time.RFC3339Nano)

	err := db.Update(id, 0, func(current []byte) ([]byte, error) {
		if len(current) == 0 {
			return nil, NotFound
		}
		bug := Bug{}
		err := json.Unmarshal(current, &bug)
		if err != nil {
			return nil, err
		}

		if !isVisible(bug, me) {
			return nil, bugNotVisible
		}

		history := Bug{
			Id:         id,
			Type:       "bughistory",
			ModifiedAt: bug.ModifiedAt,
			ModType:    bug.ModType,
			ModBy:      bug.ModBy,
		}

		switch field {
		case "private":
			oldval = fmt.Sprintf("%v", bug.Private)
			history.Private = bug.Private
			bug.Private = val == "true"
		case "description":
			oldval = bug.Description
			history.Description = bug.Description
			bug.Description = val
		case "title":
			oldval = bug.Title
			history.Title = bug.Title
			bug.Title = val
		case "status":
			oldval = bug.Status
			history.Status = bug.Status
			bug.Status = val

			// This smells of some kind of business logic
			// engine that needs to exist.
			if val == "resolved" || val == "closed" {
				bug.Owner = ""
			}
		case "owner":
			oldval = bug.Owner
			history.Owner = bug.Owner
			bug.Owner = val

			// Ensure the owner is subscribed
			if strings.Contains(val, "@") {
				bug.Subscribers = removeFromList(bug.Subscribers, val)
				bug.Subscribers = append(bug.Subscribers, val)
			}
		case "tags":
			history.Tags = bug.Tags
			oldval = strings.Join(bug.Tags, ",")
			bug.Tags = strings.FieldsFunc(val,
				func(r rune) bool {
					switch r {
					case ',', ' ':
						return true
					}
					return false
				})
		default:
			return nil, fmt.Errorf("Unhandled id: %v", field)
		}

		if (field == "description" || field == "owner") &&
			bug.Status == "inbox" {
			bug.Status = "new"
		}

		if oldval == val {
			rval, err = json.Marshal(APIBug(bug))
			if err != nil {
				return rval, err
			}
			return rval, couchbase.UpdateCancel
		}

		// This is a side-effect in a CAS operation.  It's is
		// correct and safe because the side effect is the
		// creation of a document that is only used and
		// correct relative to the final value from the CAS.
		err = db.Set(historyKey, 0, &history)
		if err != nil {
			return nil, err
		}

		bug.ModifiedAt = now
		bug.ModBy = me.Id
		bug.ModType = field
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

	switch err {
	case nil:
		notifyBugChange(id, field, me.Id)
		if field == "owner" {
			if val != me.Id {
				notifyBugAssignment(id, val)
			}
		} else if field == "tags" {
			for _, newtag := range newTags(oldval, val) {
				notifyTagAssigned(id, newtag, me.Id)
			}
		}
	case couchbase.UpdateCancel:
		log.Printf("Ignoring identical update of %v", field)
	default:
		return nil, err
	}

	return rval, nil
}

func serveBugUpdate(w http.ResponseWriter, r *http.Request) {
	rval, err := updateBug(mux.Vars(r)["bugid"],
		r.FormValue("id"),
		r.FormValue("value"),
		whoami(r))

	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	w.Write([]byte(rval))
}

type bugListResult struct {
	ID    string
	Key   []string
	Value struct {
		Title  string
		Owner  Email
		Status string
		Tags   []string
		Mod    time.Time
	}
}

func getBugList(viewName string,
	args map[string]interface{},
	chres chan []bugListResult, cherr chan error) {

	viewRes := struct {
		Rows []bugListResult
	}{}

	err := db.ViewCustom("cbugg", viewName, args, &viewRes)
	if err != nil {
		cherr <- err
		return
	}

	for i := range viewRes.Rows {
		sort.Strings(viewRes.Rows[i].Value.Tags)
	}

	chres <- viewRes.Rows
}

func serveBugList(w http.ResponseWriter, r *http.Request) {
	startPre := []interface{}{}

	viewName := "by_state"

	if r.FormValue("user") != "" {
		viewName = "owners"
		u := r.FormValue("user")
		startPre = []interface{}{u}
	}

	states := strings.Split(r.FormValue("state"), ",")
	todo := 0

	cherr := make(chan error)
	chres := make(chan []bugListResult, len(states)+1)

	for _, st := range states {
		start := make([]interface{}, len(startPre)+1)
		end := make([]interface{}, len(startPre)+2)

		copy(start, startPre)
		copy(end, startPre)
		start[len(startPre)] = st
		end[len(startPre)] = st
		end[len(startPre)+1] = map[string]string{}

		args := map[string]interface{}{
			"reduce":    false,
			"stale":     false,
			"start_key": start,
			"end_key":   end,
		}

		todo++
		go getBugList(viewName, args, chres, cherr)
	}

	if len(states) == 0 {
		todo++
		args := map[string]interface{}{
			"reduce": false,
			"stale":  false,
		}
		go getBugList(viewName, args, chres, cherr)
	}

	results := []bugListResult{}
	for i := 0; i < todo; i++ {
		select {
		case err := <-cherr:
			showError(w, r, err.Error(), 500)
			return
		case res := <-chres:
			results = append(results, res...)
		}
	}

	mustEncode(w, results)
}

func serveSubscribeBug(w http.ResponseWriter, r *http.Request) {
	err := updateSubscription(mux.Vars(r)["bugid"],
		whoami(r).Id, true)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	w.WriteHeader(204)
}

func serveUnsubscribeBug(w http.ResponseWriter, r *http.Request) {
	err := updateSubscription(mux.Vars(r)["bugid"],
		whoami(r).Id, false)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	w.WriteHeader(204)
}

func updateBugAlsoVisible(bugid string, me User, email string, add bool) error {
	historyKey := bugid + "-" + time.Now().Format(time.RFC3339Nano)

	return db.Update(bugid, 0, func(current []byte) ([]byte, error) {
		if len(current) == 0 {
			return nil, NotFound
		}
		bug := Bug{}
		err := json.Unmarshal(current, &bug)
		if err != nil {
			return nil, err
		}

		if bug.Type != "bug" {
			return nil, fmt.Errorf("Expected a bug, got %v",
				bug.Type)
		}

		history := Bug{
			Id:            bugid,
			Type:          "bughistory",
			ModifiedAt:    bug.ModifiedAt,
			ModType:       bug.ModType,
			ModBy:         bug.ModBy,
			AlsoVisibleTo: bug.AlsoVisibleTo,
		}

		if add {
			for _, e := range bug.AlsoVisibleTo {
				if e == email {
					// Already subscribed
					return nil, couchbase.UpdateCancel
				}
			}
			bug.AlsoVisibleTo = append(bug.AlsoVisibleTo, email)
		} else {
			old := bug.AlsoVisibleTo
			bug.AlsoVisibleTo = []string{}
			for _, e := range old {
				log.Printf("Checking %v (%v) = %v",
					e, md5string(e), email)
				if md5string(e) != email {
					bug.AlsoVisibleTo = append(bug.AlsoVisibleTo, e)
				}
			}
		}

		err = db.Set(historyKey, 0, &history)
		if err != nil {
			return nil, err
		}

		bug.ModType = "also_visible_to"
		bug.ModBy = me.Id
		bug.Parent = historyKey
		bug.ModifiedAt = time.Now().UTC()

		return json.Marshal(bug)
	})
}

func doBugVisibleUpdate(w http.ResponseWriter, r *http.Request, add bool) {
	bugid := mux.Vars(r)["bugid"]
	err := updateBugAlsoVisible(bugid, whoami(r),
		r.FormValue("email"), add)
	if err != nil && err != couchbase.UpdateCancel {
		showError(w, r, err.Error(), 500)
		return
	}

	bug, err := getBug(bugid)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}
	mustEncode(w, APIBug(bug))
}

func serveRemoveBugViewer(w http.ResponseWriter, r *http.Request) {
	doBugVisibleUpdate(w, r, false)
}

func serveAddBugViewer(w http.ResponseWriter, r *http.Request) {
	doBugVisibleUpdate(w, r, true)
}

func serveBugPing(w http.ResponseWriter, r *http.Request) {
	me := whoami(r)

	id := mux.Vars(r)["bugid"]
	bug, err := getBugOrDisplayErr(id, me, w, r)
	if err != nil {
		return
	}

	from := me.Id
	to := r.FormValue("to")
	if !strings.Contains(to, "@") {
		showError(w, r, "Invalid 'to' parameter", 400)
		return
	}

	// This may error on an unkown user, but will return a User
	// object that is not considered special (internal, admin), so
	// for these purposes, the error is not required.
	recipient, _ := getUser(to)

	if !isVisible(bug, recipient) {
		showError(w, r, "Cannot ping a user who can't see the bug", 403)
		return
	}

	notifyBugPing(bug, from, to)

	now := time.Now().UTC()
	pingid := "ping-" + id + "-" + now.Format(time.RFC3339Nano)
	err = db.Set(pingid, 0, &Ping{id, "ping", now, from, to})
	if err != nil {
		log.Printf("Failed to record ping notification: %v", err)
	}

	mustEncode(w, Email(to))
}

func deleteDocsMatching(ddoc, view string, args map[string]interface{},
	deleted chan couchbase.ViewRow, cherr chan error, wg *sync.WaitGroup) {

	defer wg.Done()

	viewres, err := db.View(ddoc, view, args)
	if err != nil {
		cherr <- err
		return
	}

	for _, r := range viewres.Rows {
		err = db.Delete(r.ID)
		if err != nil {
			cherr <- err
		}
		deleted <- r
	}

	cherr <- nil
}

func serveBugDeletion(w http.ResponseWriter, r *http.Request) {
	me := whoami(r)
	bugid := mux.Vars(r)["bugid"]
	_, err := getBugOrDisplayErr(bugid, me, w, r)
	if err != nil {
		return
	}

	cherr := make(chan error)
	deleted := make(chan couchbase.ViewRow)
	delatt := make(chan couchbase.ViewRow)

	wg := &sync.WaitGroup{}

	wg.Add(1)
	go deleteDocsMatching("cbugg", "comments",
		map[string]interface{}{
			"stale":     false,
			"reduce":    false,
			"start_key": []interface{}{bugid},
			"end_key":   []interface{}{bugid, map[string]string{}},
		}, deleted, cherr, wg)

	wg.Add(1)
	go deleteDocsMatching("cbugg", "attachments",
		map[string]interface{}{
			"stale":        false,
			"reduce":       false,
			"start_key":    []interface{}{bugid},
			"end_key":      []interface{}{bugid, map[string]string{}},
			"include_docs": true,
		}, delatt, cherr, wg)

	wg.Add(1)
	go deleteDocsMatching("cbugg", "bug_history",
		map[string]interface{}{
			"stale":     false,
			"reduce":    false,
			"start_key": []interface{}{bugid},
			"end_key":   []interface{}{bugid, map[string]string{}},
		}, deleted, cherr, wg)

	go func() {
		wg.Wait()
		close(cherr)
	}()

	running := true
	for running {
		select {
		case del := <-deleted:
			log.Printf("Deleted %v", del.ID)
		case del := <-delatt:
			log.Printf("Deleted attachment %v", del.ID)
			mo := (*del.Doc).(map[string]interface{})
			mi := mo["json"].(map[string]interface{})
			u := mi["url"].(string)
			err := deleteAttachmentFile(u)
			if err != nil {
				log.Printf("Error deleting attachment %v: %v",
					u, err)
			}
		case err, ok := <-cherr:
			if !ok {
				running = false
			}
			if err != nil {
				log.Printf("Error:  %v", err)
			}
		}
	}

	log.Printf("Done deleting children.")

	// This bug should already be deleted as a bug is also bug
	// history.  But give it a go anyway for clarity.
	err = db.Delete(bugid)
	if err != nil && !gomemcached.IsNotFound(err) {
		log.Printf("Error deleting the bug.")
	}

	w.WriteHeader(204)
}
