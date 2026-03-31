// Package hub provides a broadcast hub for SSE (Server-Sent Events) clients.
// The pipeline calls Publish; HTTP handlers call Subscribe/Unsubscribe.
package hub

import (
	"sync"
)

// Hub broadcasts byte slices to all subscribed channels.
type Hub struct {
	mu      sync.RWMutex
	clients map[chan []byte]struct{}
}

// New creates a new Hub.
func New() *Hub {
	return &Hub{clients: make(map[chan []byte]struct{})}
}

// Subscribe registers a new client and returns its receive channel.
// Buffer of 8 — slow clients get drops, not backpressure.
func (h *Hub) Subscribe() chan []byte {
	ch := make(chan []byte, 8)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

// Unsubscribe removes and closes the channel.
func (h *Hub) Unsubscribe(ch chan []byte) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
	close(ch)
}

// Publish sends data to all connected clients (non-blocking, drops slow clients).
func (h *Hub) Publish(data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.clients {
		select {
		case ch <- data:
		default:
			// client too slow — skip this update
		}
	}
}

// Subscribers returns the number of active SSE clients.
func (h *Hub) Subscribers() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
