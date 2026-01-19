package eventbus

import (
	"sync"

	"bops/internal/core"
)

type Subscription struct {
	C      <-chan core.Event
	Cancel func()
}

type Bus struct {
	mu     sync.RWMutex
	subs   map[int]chan core.Event
	nextID int
	closed bool
}

func New() *Bus {
	return &Bus{
		subs: make(map[int]chan core.Event),
	}
}

func (b *Bus) Subscribe(buffer int) Subscription {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		ch := make(chan core.Event)
		close(ch)
		return Subscription{C: ch, Cancel: func() {}}
	}

	if buffer < 0 {
		buffer = 0
	}
	ch := make(chan core.Event, buffer)
	id := b.nextID
	b.nextID++
	b.subs[id] = ch

	return Subscription{
		C: ch,
		Cancel: func() {
			b.unsubscribe(id)
		},
	}
}

func (b *Bus) Publish(event core.Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return
	}

	for _, ch := range b.subs {
		select {
		case ch <- event:
		default:
			// Drop if subscriber is slow.
		}
	}
}

func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}
	b.closed = true
	for _, ch := range b.subs {
		close(ch)
	}
	b.subs = map[int]chan core.Event{}
}

func (b *Bus) unsubscribe(id int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch, ok := b.subs[id]
	if !ok {
		return
	}
	delete(b.subs, id)
	close(ch)
}
