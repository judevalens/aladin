package stream

import (
	"context"
	"testing"

	"aladin/backend_v2/internal/realtime"
)

type captureEvents struct {
	target  realtime.PublishTarget
	payload any
}

func (c *captureEvents) Publish(_ context.Context, target realtime.PublishTarget, payload any) error {
	c.target, c.payload = target, payload
	return nil
}
func (*captureEvents) Subscribe(context.Context, []realtime.SubscriptionKey, string) (<-chan realtime.AppEvent, func(), error) {
	return nil, func() {}, nil
}

func TestProjectorUsesTenantScopedCopilotStream(t *testing.T) {
	events := &captureEvents{}
	NewProjector(events).Publish("u1", "t1", "token", map[string]string{"delta": "hi"})
	if events.target.TenantID != "u1" || events.target.Stream != realtime.WorkspaceStream ||
		events.target.ResourceKind != ResourceKind || events.target.ResourceID != "t1" || events.target.Operation != "token" {
		t.Fatalf("target = %+v", events.target)
	}
}
