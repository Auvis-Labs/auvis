package stream

import (
	"sync"
)

// Broadcaster manages active Server-Sent Event (SSE) subscriber channels.
type Broadcaster struct {
	mu          sync.RWMutex
	subscribers map[chan StreamEvent]bool
}

// GlobalBroadcaster is the singleton SSE broadcaster used across Auvis.
var GlobalBroadcaster = NewBroadcaster()

// NewBroadcaster initializes a new SSE Broadcaster.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		subscribers: make(map[chan StreamEvent]bool),
	}
}

// Subscribe registers a new subscriber channel with a buffer size of 100.
func (b *Broadcaster) Subscribe() chan StreamEvent {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan StreamEvent, 100)
	b.subscribers[ch] = true
	return ch
}

// Unsubscribe safely removes and closes a subscriber channel.
func (b *Broadcaster) Unsubscribe(ch chan StreamEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.subscribers[ch]; ok {
		delete(b.subscribers, ch)
		close(ch)
	}
}

// SubscriberCount returns the current number of connected SSE clients.
func (b *Broadcaster) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}

// Publish broadcasts a StreamEvent to all connected SSE clients.
// If a subscriber channel is full, the event is dropped for that client to avoid blocking hot-path execution.
func (b *Broadcaster) Publish(event StreamEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for ch := range b.subscribers {
		select {
		case ch <- event:
		default:
			// Non-blocking drop if client buffer is full
		}
	}
}
