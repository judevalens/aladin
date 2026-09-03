package workerapp

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hibiken/asynq"
)

type fakeTaskServer struct {
	started chan struct{}
	stopped chan struct{}
	once    sync.Once
	err     error
}

func (s *fakeTaskServer) Run(asynq.Handler) error {
	close(s.started)
	<-s.stopped
	return s.err
}
func (s *fakeTaskServer) Shutdown() { s.once.Do(func() { close(s.stopped) }) }

func TestSupervisorStartsOnceAndShutsDownOnCancellation(t *testing.T) {
	server := &fakeTaskServer{started: make(chan struct{}), stopped: make(chan struct{})}
	plan := NewLifecyclePlan()
	loopStarted := make(chan struct{})
	loopStopped := make(chan struct{})
	plan.Add("loop", func(ctx context.Context) {
		close(loopStarted)
		<-ctx.Done()
		close(loopStopped)
	})
	starterCalls := 0
	plan.AddStarter("scheduler", func(context.Context) { starterCalls++ })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- NewSupervisor(server, asynq.HandlerFunc(func(context.Context, *asynq.Task) error { return nil }), plan).Run(ctx)
	}()
	select {
	case <-server.started:
	case <-time.After(time.Second):
		t.Fatal("server did not start")
	}
	select {
	case <-loopStarted:
	case <-time.After(time.Second):
		t.Fatal("loop did not start")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("supervisor did not stop")
	}
	select {
	case <-loopStopped:
	case <-time.After(time.Second):
		t.Fatal("loop did not receive cancellation")
	}
	if starterCalls != 1 {
		t.Fatalf("starter calls = %d, want 1", starterCalls)
	}
}

func TestSupervisorReturnsServerFailure(t *testing.T) {
	want := errors.New("server failed")
	server := &fakeTaskServer{started: make(chan struct{}), stopped: make(chan struct{}), err: want}
	close(server.stopped)
	err := NewSupervisor(server, asynq.HandlerFunc(func(context.Context, *asynq.Task) error { return nil }), NewLifecyclePlan()).Run(context.Background())
	if !errors.Is(err, want) {
		t.Fatalf("Run error = %v, want %v", err, want)
	}
}
