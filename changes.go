package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/igm/sockjs-go/sockjs"
)

var changes_broadcaster = newBroadcaster(100)

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
	}
	return nil
}

func ChangesHandler(conn sockjs.Conn) {
	c := &connection{send: make(chan interface{}, 256), ws: conn}
	changes_broadcaster.Register(c.send)
	defer changes_broadcaster.Unregister(c.send)
	go c.writer()
	c.reader()
}
