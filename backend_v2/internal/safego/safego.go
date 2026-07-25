// Package safego runs background goroutines that cannot crash the process.
//
// Go's net/http only recovers panics on the request goroutine — never on a
// goroutine a handler spawns. A single unrecovered panic in a background loop
// (a malformed external frame, a nil deref) otherwise takes down the whole
// process, or silently kills a subsystem. These helpers contain that: every
// goroutine body runs behind a recover that logs the panic + stack.
package safego

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"
)

// restartBackoff paces the supervised-loop restart so a body that panics
// immediately on every entry can't hot-spin the CPU.
const restartBackoff = 500 * time.Millisecond

// Go runs fn once in a goroutine, recovering (and logging) a panic so it cannot
// crash the process. Use for fire-and-forget work — a single unit that should
// NOT restart on failure (one copilot turn, one snapshot fetch).
func Go(name string, fn func()) {
	go func() {
		defer recoverLog(name)
		fn()
	}()
}

// Loop runs fn in a goroutine and RESTARTS it (after a short backoff) if it
// panics or returns, until ctx is cancelled. Use for supervised long-lived
// loops that must stay alive for the process lifetime (the outbox drain, the
// alert engine, the market stream). fn is expected to block until ctx is done;
// a panic no longer ends the subsystem — it restarts.
func Loop(ctx context.Context, name string, fn func(context.Context)) {
	go func() {
		for ctx.Err() == nil {
			runOnce(ctx, name, fn)
			// Paused restart: on a clean return this only fires if fn exited
			// before ctx was cancelled (unexpected); on a panic it prevents a
			// tight crash-loop while still recovering.
			select {
			case <-ctx.Done():
				return
			case <-time.After(restartBackoff):
			}
		}
	}()
}

// runOnce isolates the recover to a single invocation so Loop keeps iterating.
func runOnce(ctx context.Context, name string, fn func(context.Context)) {
	defer recoverLog(name)
	fn(ctx)
}

func recoverLog(name string) {
	if r := recover(); r != nil {
		slog.Error("safego: recovered panic in background goroutine",
			"goroutine", name, "panic", r, "stack", string(debug.Stack()))
	}
}
