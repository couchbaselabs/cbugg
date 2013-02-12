package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/igm/sockjs-go/sockjs"
)

var changes_broadcaster = newBroadcaster(100)

var recentChanges = newChangeRing(100)

func init() {
	go rememberChanges()
}

func rememberChanges() {
	changes_broadcaster.Register(recentChanges.chin)
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

func convertMessageToChangeNotifications(message interface{}, connUser User) []interface{} {

	type rtChange struct {
		User    Email     `json:"user"`
		Bug     APIBug    `json:"bug"`
		BugID   string    `json:"bugid"`
		Action  string    `json:"action"`
		Status  string    `json:"status"`
		Title   string    `json:"title"`
		Time    time.Time `json:"timetitle"`
		Private bool      `json:"private"`
	}

	switch message := message.(type) {
	case bugChange:
		bug, err := getBugFor(message.bugid, connUser)

		if err == nil {
			result := []interface{}{}
			for _, v := range message.fields {
				result = append(result, rtChange{
					User:    Email(message.actor),
					Bug:     APIBug(bug),
					BugID:   bug.Id,
					Action:  "changed " + v,
					Status:  bug.Status,
					Title:   bug.Title,
					Time:    bug.ModifiedAt,
					Private: bug.Private,
				})
			}
			return result
		}
	case Comment:
		if !isVisible(message, connUser) {
			log.Printf("this comment not visiable to this user")
			return nil
		}

		if bug, err := getBugFor(message.BugId, connUser); err == nil {
			return []interface{}{rtChange{
				User:    Email(message.User),
				Bug:     APIBug(bug),
				BugID:   bug.Id,
				Action:  "commented on",
				Status:  bug.Status,
				Title:   bug.Title,
				Time:    message.CreatedAt,
				Private: bug.Private,
			}}
		}
	default:
		log.Printf("Unhandled RT Change message type: %T", message)
	}
	return nil
}

func ChangesHandler(conn sockjs.Conn) {
	c := &connection{send: make(chan interface{}, 256), ws: conn}
	changes_broadcaster.Register(c.send)
	defer changes_broadcaster.Unregister(c.send)
	go func() {
		for _, change := range recentChanges.Slice() {
			select {
			case c.send <- change:
			default:
			}
		}
	}()
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
