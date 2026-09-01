package service

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"time"

	"aladin/backend_v2/internal/safego"
	"aladin/backend_v2/internal/shardv2"
	"github.com/google/uuid"
)

// Refresh snapshots are an explicit consistency profile. Periodic authorized
// reads reconcile missed outbox notifications and process restarts without a
// database-specific event cursor leaking into the client protocol.
func (s *shardResourceService) Subscribe(ctx context.Context, target ResourceTarget, request ResourceRequest) (ResourceSubscription, error) {
	if err := s.admit(ctx); err != nil {
		return ResourceSubscription{}, err
	}
	if request.Query != nil && request.Query.Cursor != nil {
		return ResourceSubscription{}, ResourceFailure("unsupported-capability", "Live views cannot use a pagination cursor")
	}
	view, _, provider, err := s.resolve(ctx, target, request, "observe")
	if err != nil {
		return ResourceSubscription{}, err
	}
	if provider.Profile().Observation != "refresh-snapshots" && provider.Profile().Observation != "ordered-changes" {
		return ResourceSubscription{}, ResourceFailure("unsupported-capability", "Provider cannot be observed")
	}
	key := view.Namespace.ActorKey
	s.mu.Lock()
	if s.subscriptions[key] >= s.options.MaxSubscriptionsPerPrincipal {
		s.mu.Unlock()
		return ResourceSubscription{}, ResourceFailure("rate-limited", "Resource subscription limit exceeded")
	}
	s.subscriptions[key]++
	s.mu.Unlock()
	streamCtx, cancel := context.WithCancel(ctx)
	var once sync.Once
	closeSubscription := func() {
		once.Do(func() {
			cancel()
			s.mu.Lock()
			s.subscriptions[key]--
			if s.subscriptions[key] == 0 {
				delete(s.subscriptions, key)
			}
			s.mu.Unlock()
		})
	}
	identity := ResourceSubscriptionIdentity{SubscriptionID: uuid.NewString(), Resource: view.URI, Epoch: uuid.NewString()}
	// A single unread snapshot is enough: overflow retires the generation, it
	// never silently drops a sequence number or grows a per-client queue.
	events := make(chan ResourceStreamMessage, 1)
	var changes <-chan error
	if observer, ok := provider.(ResourceChangeObserver); ok {
		changes, err = observer.ObserveChanges(streamCtx, view)
		if err != nil {
			closeSubscription()
			return ResourceSubscription{}, err
		}
	}
	initial, err := s.snapshot(streamCtx, view, provider)
	if err != nil {
		closeSubscription()
		return ResourceSubscription{}, err
	}
	eventFor := func(snapshot ResourceSnapshot, seq uint64) *shardv2.Event {
		return &shardv2.Event{Protocol: shardv2.StreamVersion, SubscriptionID: identity.SubscriptionID, Resource: identity.Resource, Epoch: identity.Epoch, Seq: strconv.FormatUint(seq, 10), Op: "snapshot", Records: snapshot.Records, Complete: true, NextCursor: snapshot.NextCursor, SourceUpdatedAt: snapshot.SourceUpdatedAt}
	}
	initialEvent := eventFor(initial, 0)
	encoded, _ := json.Marshal(initialEvent)
	if _, err := shardv2.ValidateEvent(encoded, view.Definition, view.OutputSchema); err != nil {
		closeSubscription()
		return ResourceSubscription{}, ResourceFailure("invalid-schema", "Snapshot exceeds stream bounds")
	}
	events <- ResourceStreamMessage{Event: initialEvent}
	safego.Go("shard.resource.refresh", func() {
		defer close(events)
		defer closeSubscription()
		ticker := time.NewTicker(s.options.RefreshInterval)
		defer ticker.Stop()
		refresh := ticker.C
		if changes != nil {
			refresh = nil
		}
		last := resourceHash(initial)
		var seq uint64
		fail := func(err error) {
			// Replace an undelivered snapshot with the terminal error so sensitive
			// data cannot be delivered after this subscription observes revocation.
			select {
			case <-events:
			default:
			}
			select {
			case events <- ResourceStreamMessage{Error: &ResourceError{Code: ResourceErrorCode(err), Message: resourcePublicMessage(err)}}:
			case <-streamCtx.Done():
			}
		}
		for {
			select {
			case <-streamCtx.Done():
				return
			case <-refresh:
			case changeErr, open := <-changes:
				if !open || changeErr != nil {
					if changeErr == nil {
						changeErr = ResourceFailure("source-unavailable", "Resource change stream closed")
					}
					fail(changeErr)
					return
				}
			}
			nextView, _, nextProvider, err := s.resolve(streamCtx, target, request, "observe")
			if err != nil {
				fail(err)
				return
			}
			if nextView.URI != identity.Resource {
				fail(ResourceFailure("resync-required", "Binding parameters changed"))
				return
			}
			next, err := s.snapshot(streamCtx, nextView, nextProvider)
			if err != nil {
				fail(err)
				return
			}
			hash := resourceHash(next)
			if hash == last {
				continue
			}
			seq++
			event := eventFor(next, seq)
			raw, _ := json.Marshal(event)
			if _, err := shardv2.ValidateEvent(raw, nextView.Definition, nextView.OutputSchema); err != nil {
				fail(ResourceFailure("invalid-schema", "Snapshot exceeds stream bounds"))
				return
			}
			select {
			case events <- ResourceStreamMessage{Event: event}:
				last = hash
			case <-streamCtx.Done():
				return
			default:
				fail(ResourceFailure("resync-required", "Resource stream consumer is too slow"))
				return
			}
		}
	})
	return ResourceSubscription{Identity: identity, Events: events, Close: closeSubscription}, nil
}

func resourcePublicMessage(err error) string {
	switch ResourceErrorCode(err) {
	case "forbidden":
		return "Resource access is no longer permitted"
	case "not-found":
		return "Resource is no longer available"
	case "contract-changed":
		return "Reload the active shard release"
	case "source-unavailable":
		return "Resource source is unavailable"
	default:
		if e, ok := err.(*ResourceError); ok {
			return e.Message
		}
		return "Resource request failed"
	}
}
