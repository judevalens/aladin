// Package session owns active Copilot turn concurrency and cancellation.
package session

import (
	"context"
	"sync"
)

type Turn struct {
	UserID   string
	ThreadID string
	Cancel   context.CancelFunc
}

type CancelResult int

const (
	CancelMissing CancelResult = iota
	CancelForbidden
	CancelAccepted
)

type Registry struct {
	mu    sync.Mutex
	turns map[string]Turn
}

func NewRegistry() *Registry { return &Registry{turns: map[string]Turn{}} }

func (r *Registry) Reserve(sessionID string, turn Turn) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, active := range r.turns {
		if active.ThreadID == turn.ThreadID {
			return false
		}
	}
	r.turns[sessionID] = turn
	return true
}

func (r *Registry) Release(sessionID string) {
	r.mu.Lock()
	delete(r.turns, sessionID)
	r.mu.Unlock()
}

func (r *Registry) Cancel(userID, sessionID string) CancelResult {
	r.mu.Lock()
	turn, ok := r.turns[sessionID]
	r.mu.Unlock()
	if !ok {
		return CancelMissing
	}
	if turn.UserID != userID {
		return CancelForbidden
	}
	turn.Cancel()
	return CancelAccepted
}

func (r *Registry) ActiveCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.turns)
}
