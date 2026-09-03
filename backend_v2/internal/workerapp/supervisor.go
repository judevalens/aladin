package workerapp

import (
	"context"
	"log/slog"

	"aladin/backend_v2/internal/safego"

	"github.com/hibiken/asynq"
)

type taskServer interface {
	Run(asynq.Handler) error
	Shutdown()
}

type lifecycleLoop struct {
	name string
	run  func(context.Context)
}

// LifecyclePlan collects background loops during composition. Nothing starts
// until Supervisor.Run, so startup has one owner and cancellation reaches every loop.
type LifecyclePlan struct {
	loops    []lifecycleLoop
	starters []lifecycleLoop
}

func NewLifecyclePlan() *LifecyclePlan { return &LifecyclePlan{} }

func (p *LifecyclePlan) Add(name string, run func(context.Context)) {
	p.loops = append(p.loops, lifecycleLoop{name: name, run: run})
}

// AddStarter registers a component whose Start method launches its own
// context-bound goroutine and returns immediately. It is invoked exactly once.
func (p *LifecyclePlan) AddStarter(name string, start func(context.Context)) {
	p.starters = append(p.starters, lifecycleLoop{name: name, run: start})
}

type Supervisor struct {
	server  taskServer
	handler asynq.Handler
	plan    *LifecyclePlan
}

func NewSupervisor(server taskServer, handler asynq.Handler, plan *LifecyclePlan) *Supervisor {
	return &Supervisor{server: server, handler: handler, plan: plan}
}

// Run starts all planned loops and the queue server, then owns deterministic
// shutdown. A server failure cancels the process; context cancellation shuts the
// server down and waits for Run to return.
func (s *Supervisor) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	for _, loop := range s.plan.loops {
		safego.Loop(ctx, loop.name, loop.run)
	}
	for _, starter := range s.plan.starters {
		starter.run(ctx)
	}
	errCh := make(chan error, 1)
	go func() { errCh <- s.server.Run(s.handler) }()
	select {
	case err := <-errCh:
		cancel()
		return err
	case <-ctx.Done():
		s.server.Shutdown()
		err := <-errCh
		if err != nil {
			slog.Debug("asynq server stopped during shutdown", "component", "worker", "err", err)
		}
		return nil
	}
}
