// Package control provides the Capper control plane primitives: an in-process
// event bus, a reconciler loop, and an admission chain.
package control

import "sync"

// Event carries a lifecycle notification for a managed resource.
type Event struct {
	Type         string         // "created" | "updated" | "deleted" | "failed"
	ResourceType string         // "instance" | "network" | "volume" | ...
	ResourceID   string
	Project      string
	Data         map[string]any // optional supplemental data
}

// Bus is a non-blocking in-process pub/sub event bus.
// Subscribers receive events on a buffered channel; slow consumers drop events
// (the channel send is non-blocking) rather than stalling producers.
type Bus struct {
	mu   sync.Mutex
	subs map[string][]chan Event // keyed by ResourceType; "*" receives everything
}

// NewBus creates a ready-to-use Bus.
func NewBus() *Bus {
	return &Bus{subs: make(map[string][]chan Event)}
}

// Publish sends e to all subscribers for e.ResourceType and all "*" subscribers.
// The send is non-blocking; events are dropped for subscribers that are full.
func (b *Bus) Publish(e Event) {
	b.mu.Lock()
	chans := make([]chan Event, 0, len(b.subs[e.ResourceType])+len(b.subs["*"]))
	chans = append(chans, b.subs[e.ResourceType]...)
	chans = append(chans, b.subs["*"]...)
	b.mu.Unlock()

	for _, ch := range chans {
		select {
		case ch <- e:
		default:
		}
	}
}

// Subscribe returns a channel that will receive events for the given
// resourceType. Pass "*" to receive all events. The returned channel is
// buffered (cap 128).
func (b *Bus) Subscribe(resourceType string) <-chan Event {
	ch := make(chan Event, 128)
	b.mu.Lock()
	b.subs[resourceType] = append(b.subs[resourceType], ch)
	b.mu.Unlock()
	return ch
}
