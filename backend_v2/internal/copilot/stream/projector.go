// Package stream owns projection of Copilot events onto the realtime transport.
package stream

import (
	"context"

	"aladin/backend_v2/internal/realtime"
)

const ResourceKind = "copilot"

type Projector struct{ realtime realtime.EventService }

func NewProjector(events realtime.EventService) *Projector { return &Projector{realtime: events} }

func (p *Projector) Publish(userID, threadID, operation string, payload any) {
	if p == nil || p.realtime == nil {
		return
	}
	_ = p.realtime.Publish(context.Background(), realtime.PublishTarget{
		TenantID: userID, Stream: realtime.WorkspaceStream, ResourceKind: ResourceKind,
		ResourceID: threadID, Operation: operation,
	}, payload)
}
