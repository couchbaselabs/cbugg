package main

import (
	"encoding/json"
	"log"

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

func convertMessageToChangeNotifications(message interface{}, connUser User) []map[string]interface{} {
	switch message := message.(type) {
	case bugChange:
		user := Email(message.actor)
		bug, err := getBugFor(message.bugid, connUser)

		if err == nil {
			result := []map[string]interface{}{}
			for _, v := range message.fields {
				change := map[string]interface{}{
					"user":    user,
					"bug":     bug,
					"bugid":   bug.Id,
					"action":  "changed " + v,
					"status":  bug.Status,
					"title":   bug.Title,
					"time":    bug.ModifiedAt,
					"private": bug.Private,
				}
				result = append(result, change)
			}
			return result
		}
	case Comment:

		if !isVisible(message, connUser) {
			log.Printf("this comment not visiable to this user")
			return nil
		}

		user := Email(message.User)
		bug, err := getBugFor(message.BugId, connUser)

		if err == nil {
			result := []map[string]interface{}{}
			change := map[string]interface{}{
				"user":    user,
				"bug":     bug,
				"bugid":   bug.Id,
				"action":  "commented on",
				"status":  bug.Status,
				"title":   bug.Title,
				"time":    message.CreatedAt,
				"private": bug.Private,
			}
			result = append(result, change)
			return result
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
