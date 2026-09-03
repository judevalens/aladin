package copilot

import (
	"context"

	coreservice "aladin/backend_v2/internal/service"
)

// These aliases describe the remaining cross-domain inputs to Copilot. They
// keep the dependency visible while Auth, Artifact, and Entity are migrated in
// their own bounded slices.
type Principal = coreservice.Principal
type ArtifactService = coreservice.ArtifactService
type ArtifactResponse = coreservice.ArtifactResponse
type EntityContextService = coreservice.EntityContextService
type BadRequest = coreservice.BadRequest

var (
	ErrConflict        = coreservice.ErrConflict
	ErrNotFound        = coreservice.ErrNotFound
	ErrUnauthenticated = coreservice.ErrUnauthenticated
)

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return coreservice.WithPrincipal(ctx, principal)
}
