// Package provider owns the protocol boundary to the Copilot sidecar.
package provider

import (
	"context"

	"aladin/backend_v2/internal/copilotagent"
)

type Client interface {
	StartTurn(ctx context.Context, request copilotagent.TurnRequest) (<-chan copilotagent.Event, error)
	Cancel(ctx context.Context, turnID string) error
	ResolveApproval(ctx context.Context, turnID, approvalID string, approved bool) error
	Healthz(ctx context.Context) (copilotagent.Health, error)
}
