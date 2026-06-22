// This file streams server-sent events to HTTP clients.
package httpx

import (
	"fmt"
	"net/http"
	"sync"
)

type Event struct {
	Name string
	Data string
}

type Broker struct {
	mu      sync.Mutex
	closed  bool
	clients map[chan Event]struct{}
}

func NewBroker() *Broker {
	return &Broker{clients: make(map[chan Event]struct{})}
}

func (b *Broker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		WriteError(w, http.StatusInternalServerError, "sse_unsupported", "Streaming is not supported")
		return
	}

	client := make(chan Event, 8)
	if !b.addClient(client) {
		WriteError(w, http.StatusServiceUnavailable, "sse_closed", "Event stream is closed")
		return
	}
	defer b.removeClient(client)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-client:
			if !ok {
				return
			}
			if event.Name != "" {
				_, _ = fmt.Fprintf(w, "event: %s\n", event.Name)
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", event.Data)
			flusher.Flush()
		}
	}
}

func (b *Broker) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for client := range b.clients {
		close(client)
		delete(b.clients, client)
	}
}

func (b *Broker) addClient(client chan Event) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return false
	}
	b.clients[client] = struct{}{}
	return true
}

func (b *Broker) removeClient(client chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.clients[client]; ok {
		close(client)
		delete(b.clients, client)
	}
}
