package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/couchbaselabs/sockjs-go/sockjs"
	"github.com/dustin/go-broadcast"
)

var changes_broadcaster = broadcast.NewBroadcaster(100)

var recentChanges = newChangeRing(100)

func init() {
	go rememberChanges()
}

func rememberChanges() {
	changes_broadcaster.Register(recentChanges.chin)
}

type Change struct {
	User    Email     `json:"user"`
	Action  string    `json:"action"`
	Bug     APIBug    `json:"bug"`
	BugID   string    `json:"bugid"`
	Time    time.Time `json:"time"`
	Status  string    `json:"status"`
	Title   string    `json:"title"`
	Private bool      `json:"private"`
}

type changeEligible interface {
	changeObjectFor(u User) (Change, error)
}

type changeRing struct {
	start int
	items []interface{}
	chin  chan interface{}
	req   chan chan []interface{}
}

func newChangeRing(size int) *changeRing {
	rv := &changeRing{
		items: make([]interface{}, 0, size),
		chin:  make(chan interface{}),
		req:   make(chan chan []interface{}),
	}
	go rv.process()
	return rv
}

func (cr *changeRing) Add(i interface{}) {
	cr.chin <- i
}

func (cr *changeRing) Slice() []interface{} {
	ch := make(chan []interface{}, 1)
	cr.req <- ch
	return <-ch
}

func (cr *changeRing) Latest(n int) []interface{} {
	r := cr.Slice()
	if len(r) > n {
		r = r[len(r)-n:]
	}
	return r
}

func (cr *changeRing) process() {
	for {
		select {
		case i := <-cr.chin:
			cr.addItem(i)
		case r := <-cr.req:
			r <- cr.slice()
		}
	}
}

func (cr *changeRing) addItem(i interface{}) {
	if len(cr.items) < cap(cr.items) {
		cr.items = append(cr.items, i)
	} else {
		if cr.start == cap(cr.items) {
			cr.start = 0
		}
		cr.items[cr.start] = i
		cr.start++
	}
}

func (cr *changeRing) slice() []interface{} {
	rv := make([]interface{}, 0, cap(cr.items))
	for i := cr.start; i < len(cr.items); i++ {
		rv = append(rv, cr.items[i])
	}
	for i := 0; i < cr.start; i++ {
		rv = append(rv, cr.items[i])
	}
	return rv
}

type connection struct {
	// The websocket connection.
	ws sockjs.Conn

	// Buffered channel of outbound messages.
	send chan interface{}

	// Authenticated User
	user User
}

func (c *connection) reader() {
	for {

		if msg, err := c.ws.ReadMessage(); err == nil {

			// this section is unfortunately ugly, they seem to have
			// double encoded the JSON string in another string

			// first parse the message as a JSON string
			var parsedString string
			err = json.Unmarshal(msg, &parsedString)
			if err != nil {
				log.Printf("error decoding message string %v", err)
				continue
			}

			// no parse that string as a JSON object
			parsedMessage := map[string]interface{}{}
			err = json.Unmarshal([]byte(parsedString), &parsedMessage)
			if err != nil {
				log.Printf("error decoding message json %v", err)
				continue
			}

			// now if this is an auth message, validate the cookie
			if parsedMessage["type"] == "auth" {
				switch cookie := parsedMessage["cookie"].(type) {
				case string:
					user, err := userFromCookie(cookie)
					if err == nil {
						log.Printf("authenticated realtime stream as user %v", user)
						c.user = user
					}
				}
			}
		} else {
			break
		}
	}
	c.ws.Close()
}

func (c *connection) writer() {
	for message := range c.send {
		changes := convertMessageToChangeNotifications(message, c.user)
		for _, change := range changes {
			bytes, err := json.Marshal(change)
			if err != nil {
				log.Print("Failed to marshall notification to JSON, ignoring")
				continue
			}
			_, err = c.ws.WriteMessage(bytes)
			if err != nil {
				break
			}
		}
	}
	c.ws.Close()
}

func convertMessageToChangeNotifications(message interface{},
	connUser User) []interface{} {

	co, ok := message.(changeEligible)
	if ok {
		c, err := co.changeObjectFor(connUser)
		if err == nil {
			return []interface{}{c}
		}
	} else {
		log.Printf("%T isn't changeEligible", message)
	}
	return nil
}

func ChangesHandler(conn sockjs.Conn) {
	c := &connection{send: make(chan interface{}, 256), ws: conn}
	for _, change := range recentChanges.Latest(cap(c.send)) {
		c.send <- change
	}
	changes_broadcaster.Register(c.send)
	defer changes_broadcaster.Unregister(c.send)
	go c.writer()
	c.reader()
}

func loadChangeObject(doctype, docid string) (interface{}, error) {
	switch doctype {
	case "bug", "bughistory":
		bug, err := getBug(docid)
		if err != nil {
			return bug, err
		}
		return bugChange{bug.Id,
			bug.ModBy,
			[]string{bug.ModType},
			"",
			&bug,
		}, nil
	case "comment":
		return getComment(docid)
	}
	return nil, fmt.Errorf("Unhandled type: %v", doctype)
}

func loadRecent() {
	args := map[string]interface{}{
		"descending": true,
		"limit":      20,
		"stale":      false,
	}

	viewRes := struct {
		Rows []struct {
			ID    string
			Key   string
			Value struct {
				Type string
			}
		}
	}{}

	err := db.ViewCustom("cbugg", "changes", args, &viewRes)
	if err != nil {
		log.Printf("Error initializing recent changes: %v", err)
		return
	}

	for i := range viewRes.Rows {
		r := viewRes.Rows[len(viewRes.Rows)-i-1]
		change, err := loadChangeObject(r.Value.Type, r.ID)
		if err == nil {
			recentChanges.Add(change)
		} else {
			log.Printf("Error loading %v/%v: %v",
				r.Value.Type, r.ID, err)
		}
	}

}
