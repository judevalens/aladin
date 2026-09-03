// Package realtime owns bounded in-process fanout and reconnect replay.
package realtime

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	resolver     KeyResolver
	mu           sync.RWMutex
	subscribers  map[string]subscriber
	recentEvents []recentEvent
	recentLimit  int
	dropped      int64
}

type subscriber struct {
	keys []SubscriptionKey
	ch   chan AppEvent
	done chan struct{}
}
type recentEvent struct {
	event AppEvent
	keys  []SubscriptionKey
}

func NewService(resolver KeyResolver) *Service {
	return &Service{resolver: resolver, subscribers: make(map[string]subscriber), recentLimit: 256}
}

func (s *Service) Publish(ctx context.Context, target PublishTarget, payload any) error {
	keys, eventType, err := s.resolver.ResolvePublishKeys(ctx, target)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	event := AppEvent{EventID: "evt-" + uuid.NewString(), Type: eventType, SubscriptionKey: keys[0].Public(), Payload: payload, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano)}
	s.mu.Lock()
	s.recentEvents = append(s.recentEvents, recentEvent{event: event, keys: cloneKeys(keys)})
	if len(s.recentEvents) > s.recentLimit {
		s.recentEvents = s.recentEvents[len(s.recentEvents)-s.recentLimit:]
	}
	for id, sub := range s.subscribers {
		if !matchesAny(sub.keys, keys) {
			continue
		}
		select {
		case sub.ch <- event:
		default:
			s.dropped++
			if s.dropped == 1 || s.dropped%100 == 0 {
				slog.Warn("realtime: dropped frame to full subscriber buffer", "component", "realtime", "subscriber", id, "event_type", eventType, "dropped_total", s.dropped)
			}
		}
	}
	s.mu.Unlock()
	return nil
}

func (s *Service) Subscribe(ctx context.Context, keys []SubscriptionKey, afterEventID string) (<-chan AppEvent, func(), error) {
	id := uuid.NewString()
	ch := make(chan AppEvent, 64)
	done := make(chan struct{})
	s.mu.Lock()
	s.subscribers[id] = subscriber{keys: keys, ch: ch, done: done}
	replay := s.replayLocked(keys, afterEventID)
	s.mu.Unlock()
	go func() {
		for _, event := range replay {
			select {
			case ch <- event:
			case <-done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	unsubscribe := func() {
		s.mu.Lock()
		if sub, ok := s.subscribers[id]; ok {
			delete(s.subscribers, id)
			close(sub.done)
		}
		s.mu.Unlock()
	}
	return ch, unsubscribe, nil
}

func (s *Service) replayLocked(keys []SubscriptionKey, afterEventID string) []AppEvent {
	if afterEventID == "" {
		return nil
	}
	found := false
	var out []AppEvent
	for _, item := range s.recentEvents {
		if !found {
			found = item.event.EventID == afterEventID
			continue
		}
		if matchesAny(keys, item.keys) {
			out = append(out, item.event)
		}
	}
	return out
}

func cloneKeys(keys []SubscriptionKey) []SubscriptionKey {
	out := make([]SubscriptionKey, 0, len(keys))
	for _, key := range keys {
		key.Qualifiers = normalizeQualifiers(key.Qualifiers)
		out = append(out, key)
	}
	return out
}

func matchesAny(subscriberKeys, publishedKeys []SubscriptionKey) bool {
	for _, sub := range subscriberKeys {
		for _, pub := range publishedKeys {
			if sub.Matches(pub) {
				return true
			}
		}
	}
	return false
}
