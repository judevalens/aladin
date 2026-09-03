package session

import (
	"context"
	"testing"
)

func TestRegistryReservesOneTurnPerThreadAndScopesCancellation(t *testing.T) {
	registry := NewRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	if !registry.Reserve("s1", Turn{UserID: "u1", ThreadID: "t1", Cancel: cancel}) {
		t.Fatal("first reservation failed")
	}
	if registry.Reserve("s2", Turn{UserID: "u1", ThreadID: "t1", Cancel: func() {}}) {
		t.Fatal("second turn on one thread was accepted")
	}
	if got := registry.Cancel("u2", "s1"); got != CancelForbidden {
		t.Fatalf("wrong-owner cancel = %v", got)
	}
	select {
	case <-ctx.Done():
		t.Fatal("wrong owner cancelled turn")
	default:
	}
	if got := registry.Cancel("u1", "s1"); got != CancelAccepted {
		t.Fatalf("owner cancel = %v", got)
	}
	if ctx.Err() == nil {
		t.Fatal("owner cancellation did not propagate")
	}
	registry.Release("s1")
	if registry.ActiveCount() != 0 {
		t.Fatal("released turn remains active")
	}
}
