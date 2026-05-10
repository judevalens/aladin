package ratelimit

import (
	"context"
	"time"
)

// Limiter is a simple token bucket rate limiter.
type Limiter struct {
	tokens   chan struct{}
	interval time.Duration
}

// New creates a limiter allowing rpm requests per minute.
func New(rpm int) *Limiter {
	return newLimiter(rpm, rpm)
}

// NewPaced creates a limiter allowing rpm requests per minute without an
// initial burst. Use this for provider APIs that punish short request spikes.
func NewPaced(rpm int) *Limiter {
	return newLimiter(rpm, 0)
}

func newLimiter(rpm int, initialTokens int) *Limiter {
	if rpm <= 0 {
		rpm = 1
	}
	if initialTokens < 0 {
		initialTokens = 0
	}
	if initialTokens > rpm {
		initialTokens = rpm
	}
	interval := time.Minute / time.Duration(rpm)
	l := &Limiter{
		tokens:   make(chan struct{}, rpm),
		interval: interval,
	}
	// Fill bucket
	for i := 0; i < initialTokens; i++ {
		l.tokens <- struct{}{}
	}
	// Refill one token per interval
	go l.refill()
	return l
}

// Wait blocks until a token is available or context is cancelled.
func (l *Limiter) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-l.tokens:
		return nil
	}
}

func (l *Limiter) refill() {
	ticker := time.NewTicker(l.interval)
	defer ticker.Stop()
	for range ticker.C {
		select {
		case l.tokens <- struct{}{}:
		default: // bucket full
		}
	}
}
