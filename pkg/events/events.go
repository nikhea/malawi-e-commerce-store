// Package events is a tiny in-process domain event bus. Synchronous:
// Publish calls subscribers inline, in registration order. Subscribers
// must be FAST (record + return) and must never fail checkout — Publish
// recovers panics per subscriber and keeps going.
//
// Rule: events carry side effects (notify, sync, bill), never answers.
// Anything needing a return value is a direct public.Service call (§4).
package events

import (
	"context"
	"log"
	"sync"
	"time"
)

// Event is one domain fact. Name is namespaced ("orders.created").
// Payload is the publishing module's public struct — subscribers import
// the publisher's public/ package to read it (contract-to-contract).
type Event struct {
	Name    string
	Payload any
	At      time.Time
}

// Handler reacts to an event. No return values by design.
type Handler func(ctx context.Context, e Event)

// Bus fans events out to subscribers.
type Bus struct {
	mu   sync.RWMutex
	subs map[string][]Handler
}

func New() *Bus {
	return &Bus{subs: map[string][]Handler{}}
}

// Subscribe registers h for name. Safe for concurrent use; typically
// called once per subscriber at wiring.
func (b *Bus) Subscribe(name string, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs[name] = append(b.subs[name], h)
}

// Publish delivers e to its subscribers, in order. A panicking
// subscriber is recovered and logged — checkout never dies because a
// notification handler did.
func (b *Bus) Publish(ctx context.Context, e Event) {
	if e.At.IsZero() {
		e.At = time.Now()
	}
	b.mu.RLock()
	handlers := append([]Handler(nil), b.subs[e.Name]...)
	b.mu.RUnlock()
	for _, h := range handlers {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("events: subscriber panic on %s: %v", e.Name, r)
				}
			}()
			h(ctx, e)
		}()
	}
}
