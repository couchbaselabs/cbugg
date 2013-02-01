package main

import (
	"encoding/json"
	"log"

	"github.com/igm/sockjs-go/sockjs"
)

func init() {
	go changes_broadcaster.run()
}

type broadcast_hub struct {
	// Registered connections.
	connections map[*connection]bool

	// Inbound messages from the connections.
	broadcast chan interface{}

	// Register requests from the connections.
	register chan *connection

	// Unregister requests from connections.
	unregister chan *connection
}

var changes_broadcaster = broadcast_hub{
	broadcast:   make(chan interface{}),
	register:    make(chan *connection),
	unregister:  make(chan *connection),
	connections: make(map[*connection]bool),
}

func (h *broadcast_hub) run() {
	for {
		select {
		case c := <-h.register:
			h.connections[c] = true
		case c := <-h.unregister:
			delete(h.connections, c)
			close(c.send)
		case m := <-h.broadcast:
			for c := range h.connections {
				select {
				case c.send <- m:
				default:
					delete(h.connections, c)
					close(c.send)
					go c.ws.Close()
				}
			}
		}
	}
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
	changes_broadcaster.register <- c
	defer func() { changes_broadcaster.unregister <- c }()
	go c.writer()
	c.reader()
}
