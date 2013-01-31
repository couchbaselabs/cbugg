package main

import (
	"github.com/igm/sockjs-go/sockjs"
	"encoding/json"
	"log"
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
}

func (c *connection) reader() {
	for {

		if _, err := c.ws.ReadMessage(); err == nil {
			// do nothing
		} else {
			break
		}
		// we simply eat these?
	}
	c.ws.Close()
}

func (c *connection) writer() {
	for message := range c.send {
		changes := convertMessageToChangeNotifications(message)
		if changes != nil {
			for _, change := range changes {
				bytes, err := json.Marshal(change)
				if err != nil {
					log.Print("Failed to marshall notification to JSON, ignoring")
					continue
				}
				log.Printf("sending notifications %s, string(bytes)", bytes)
				_,err = c.ws.WriteMessage(bytes)
				if err != nil {
					break
				}
			}
		}
	}
	c.ws.Close()
}

func convertMessageToChangeNotifications(message interface{}) []map[string]interface{} {
	switch message := message.(type) {
	case bugChange:
		user := Email(message.actor)
		bug, err := getBug(message.bugid)
		if err == nil {
			result := []map[string]interface{}{}
			for _, v := range message.fields {
				change := map[string]interface{}{
					"user":   user,
					"bug":    bug,
					"action": "changed " + v,
				}
				result = append(result, change)
			}
			return result
		}
	case Comment:
		log.Printf("see comment")
		user := Email(message.User)
		bug, err := getBug(message.BugId)
		if err == nil {
			result := []map[string]interface{}{}
			change := map[string]interface{}{
				"user":   user,
				"bug":    bug,
				"action": "commented on",
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
