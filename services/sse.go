package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type SSEHub struct {
	clients    map[chan string]bool
	register   chan chan string
	unregister chan chan string
	broadcast  chan []byte
	mutex      sync.Mutex
}

var Hub *SSEHub

func InitSSEHub() {
	Hub = &SSEHub{
		clients:    make(map[chan string]bool),
		register:   make(chan chan string),
		unregister: make(chan chan string),
		broadcast:  make(chan []byte),
	}
	go Hub.run()
	go func() {
		for {
			time.Sleep(10 * time.Second)
			Hub.Broadcast("ping", "pong")
		}
	}()
}

func (h *SSEHub) run() {
	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			h.clients[client] = true
			h.mutex.Unlock()

		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client)
			}
			h.mutex.Unlock()

		case message := <-h.broadcast:
			h.mutex.Lock()
			for client := range h.clients {
				select {
				case client <- string(message):
				default:
					close(client)
					delete(h.clients, client)
				}
			}
			h.mutex.Unlock()
		}
	}
}

func (h *SSEHub) Broadcast(event string, data interface{}) {
	payload, _ := json.Marshal(data)
	msg := fmt.Sprintf("event: %s\ndata: %s\n\n", event, payload)
	h.broadcast <- []byte(msg)
}

func ServeSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	clientChan := make(chan string, 100)
	Hub.register <- clientChan

	// Send initial connection event
	fmt.Fprintf(w, "event: connected\ndata: true\n\n")
	w.(http.Flusher).Flush()

	defer func() {
		Hub.unregister <- clientChan
	}()

	for {
		select {
		case msg, ok := <-clientChan:
			if !ok {
				return
			}
			if _, err := fmt.Fprintf(w, "%s", msg); err != nil {
				return
			}
			w.(http.Flusher).Flush()
		case <-r.Context().Done():
			return
		}
	}
}
