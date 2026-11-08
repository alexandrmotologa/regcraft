package telemetry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// EventType categorizes live system actions.
type EventType string

const (
	EventPullStart    EventType = "pull_start"
	EventPullProgress EventType = "pull_progress"
	EventPullComplete EventType = "pull_complete"
	EventCacheHit     EventType = "cache_hit"
	EventWarmProgress EventType = "warm_progress"
	EventGCComplete   EventType = "gc_complete"
)

// LiveEvent describes an event broadcast to dashboard subscribers.
type LiveEvent struct {
	Type        EventType `json:"type"`
	Repository  string    `json:"repository,omitempty"`
	Reference   string    `json:"reference,omitempty"`
	Digest      string    `json:"digest,omitempty"`
	Bytes       int64     `json:"bytes,omitempty"`
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
}

// Broadcaster manages SSE subscribers and distributes live events.
type Broadcaster struct {
	subscribers map[chan LiveEvent]struct{}
	mu          sync.RWMutex
}

var defaultBroadcaster = NewBroadcaster()

// Events returns the global event broadcaster instance.
func Events() *Broadcaster {
	return defaultBroadcaster
}

// NewBroadcaster creates an event distribution hub.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		subscribers: make(map[chan LiveEvent]struct{}),
	}
}

// Subscribe registers a client to receive real-time events.
func (b *Broadcaster) Subscribe() (<-chan LiveEvent, func()) {
	ch := make(chan LiveEvent, 64)

	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()

	unsubscribe := func() {
		b.mu.Lock()
		delete(b.subscribers, ch)
		close(ch)
		b.mu.Unlock()
	}

	return ch, unsubscribe
}

// Broadcast dispatches an event to all connected listeners.
func (b *Broadcaster) Broadcast(e LiveEvent) {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	for ch := range b.subscribers {
		select {
		case ch <- e:
		default:
			// Non-blocking write: drop if subscriber channel buffer is full
		}
	}
}

// ServeHTTP implements standard HTTP Server-Sent Events (SSE).
func (b *Broadcaster) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	events, unsubscribe := b.Subscribe()
	defer unsubscribe()

	// Initial ping event
	initEvent, _ := json.Marshal(LiveEvent{
		Type:      "connected",
		Message:   "Connected to regcraft live telemetry stream",
		Timestamp: time.Now().UTC(),
	})
	fmt.Fprintf(w, "data: %s\n\n", initEvent)
	flusher.Flush()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			// Keep-alive heartbeat comment
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		case ev, ok := <-events:
			if !ok {
				return
			}
			data, err := json.Marshal(ev)
			if err == nil {
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
		}
	}
}
