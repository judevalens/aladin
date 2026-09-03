// Package approval owns proposed tool-action correlation and owner scoping.
package approval

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrUnavailable = errors.New("approval resolver unavailable")

type Resolver interface {
	ResolveApproval(ctx context.Context, turnID, approvalID string, approved bool) error
}

type Action struct {
	UserID   string
	ThreadID string
	TurnID   string
	Tool     string
	Created  time.Time
}

type Registry struct {
	mu      sync.Mutex
	ttl     time.Duration
	actions map[string]Action
}

func NewRegistry(ttl time.Duration) *Registry {
	return &Registry{ttl: ttl, actions: map[string]Action{}}
}

func (r *Registry) Register(actionID string, action Action) {
	r.mu.Lock()
	now := time.Now()
	for id, existing := range r.actions {
		if now.Sub(existing.Created) > r.ttl {
			delete(r.actions, id)
		}
	}
	if action.Created.IsZero() {
		action.Created = now
	}
	r.actions[actionID] = action
	r.mu.Unlock()
}

func (r *Registry) Take(userID, actionID string) (Action, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	action, ok := r.actions[actionID]
	if !ok || action.UserID != userID {
		return Action{}, false
	}
	delete(r.actions, actionID)
	return action, true
}

func (r *Registry) Drop(actionID string) {
	r.mu.Lock()
	delete(r.actions, actionID)
	r.mu.Unlock()
}

func (r *Registry) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.actions)
}

// Gateway owns owner-scoped action correlation and provider resolution.
type Gateway struct {
	registry *Registry
	resolver Resolver
}

func NewGateway(resolver Resolver, ttl time.Duration) *Gateway {
	return &Gateway{registry: NewRegistry(ttl), resolver: resolver}
}

func (g *Gateway) Register(actionID string, action Action) { g.registry.Register(actionID, action) }
func (g *Gateway) Drop(actionID string)                    { g.registry.Drop(actionID) }
func (g *Gateway) PendingCount() int                       { return g.registry.Count() }

func (g *Gateway) Resolve(ctx context.Context, userID, actionID string, approved bool) (Action, bool, error) {
	action, ok := g.registry.Take(userID, actionID)
	if !ok {
		return Action{}, false, nil
	}
	if g.resolver == nil {
		return action, true, ErrUnavailable
	}
	return action, true, g.resolver.ResolveApproval(ctx, action.TurnID, actionID, approved)
}
