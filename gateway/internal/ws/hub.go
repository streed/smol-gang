package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type Message struct {
	Type    string `json:"type"`
	Content string `json:"content"`
	Source  string `json:"source,omitempty"`
}

type client struct {
	conn         *websocket.Conn
	workstreamID string
	send         chan []byte
	closeOnce    sync.Once
}

type Hub struct {
	mu      sync.RWMutex
	rooms   map[string]map[*client]bool
	upgrader websocket.Upgrader
}

func NewHub() *Hub {
	return &Hub{
		rooms: make(map[string]map[*client]bool),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request, workstreamID string) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return
	}

	c := &client{
		conn:         conn,
		workstreamID: workstreamID,
		send:         make(chan []byte, 256),
	}

	h.addClient(workstreamID, c)

	go h.writePump(c)
	go h.readPump(c)
}

func (h *Hub) BroadcastToWorkstream(workstreamID string, msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	h.mu.RLock()
	room, ok := h.rooms[workstreamID]
	h.mu.RUnlock()

	if !ok {
		return
	}

	for c := range room {
		select {
		case c.send <- data:
		default:
			h.removeClient(workstreamID, c)
		}
	}
}

func (h *Hub) addClient(workstreamID string, c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.rooms[workstreamID]; !ok {
		h.rooms[workstreamID] = make(map[*client]bool)
	}
	h.rooms[workstreamID][c] = true
}

func (h *Hub) removeClient(workstreamID string, c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if room, ok := h.rooms[workstreamID]; ok {
		delete(room, c)
		if len(room) == 0 {
			delete(h.rooms, workstreamID)
		}
	}
	c.closeOnce.Do(func() {
		close(c.send)
		c.conn.Close()
	})
}

func (h *Hub) writePump(c *client) {
	defer func() {
		h.removeClient(c.workstreamID, c)
	}()

	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

func (h *Hub) readPump(c *client) {
	defer func() {
		h.removeClient(c.workstreamID, c)
	}()

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		// Client messages are handled via REST API, WS is for push only
	}
}
