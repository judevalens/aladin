package pipeline

import (
	"fmt"
	"time"
)

// ErrRateLimit is returned when an external API rate limits the worker.
// The handler re-enqueues the task after RetryAfter has elapsed.
type ErrRateLimit struct {
	RetryAfter time.Duration
}

func (e ErrRateLimit) Error() string {
	return fmt.Sprintf("rate limited, retry after %s", e.RetryAfter)
}

// ErrTransient is a temporary failure — the task should be retried with backoff.
type ErrTransient struct {
	Cause error
}

func (e ErrTransient) Error() string {
	return fmt.Sprintf("transient error: %v", e.Cause)
}

func (e ErrTransient) Unwrap() error { return e.Cause }

// ErrPermanent is an unrecoverable failure — the task should be dropped.
type ErrPermanent struct {
	Cause error
}

func (e ErrPermanent) Error() string {
	return fmt.Sprintf("permanent error: %v", e.Cause)
}

func (e ErrPermanent) Unwrap() error { return e.Cause }
