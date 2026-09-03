package relationship

import (
	"context"
	"strings"
	"time"

	coreservice "aladin/backend_v2/internal/service"
)

// Relationship is a typed edge connecting two workspace/ingestion entities — the
// additive "bridge" between the two worlds (artifacts ↔ records ↔ insights) that
// does NOT unify their tables. Endpoints are (kind, id) pairs; kind is one of
// "artifact" | "record" | "insight". See design/archive/DATA_MODEL.md.
type Relationship struct {
	ID        string         `json:"id"`
	UserID    string         `json:"-"`
	SrcKind   string         `json:"srcKind"`
	SrcID     string         `json:"srcId"`
	DstKind   string         `json:"dstKind"`
	DstID     string         `json:"dstId"`
	RelType   string         `json:"relType"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
}

// Valid endpoint kinds and edge types. Kept here (not just the DB CHECK) so the
// service layer can validate + return a clean BadRequest before hitting the DB.
var (
	RelationshipKinds = map[string]bool{"artifact": true, "record": true, "insight": true}
	RelationshipTypes = map[string]bool{
		"cites": true, "supports": true, "contradicts": true, "about": true, "derived_from": true,
	}
)

// RelationshipStore is the persistence port for the edge layer. Methods take an
// explicit userID (the service layer resolves it from the principal) so the repo
// stays a thin data-access layer.
type RelationshipStore interface {
	// Create upserts an edge (idempotent on the unique edge key) and returns it.
	Create(ctx context.Context, rel Relationship) (Relationship, error)
	// ListForNode returns every edge touching (kind,id) in EITHER direction.
	ListForNode(ctx context.Context, userID, kind, id string) ([]Relationship, error)
	// Delete removes an edge by id, scoped to the owner.
	Delete(ctx context.Context, userID, id string) error
}

// RelationshipService is the application port: it resolves the owner from the
// request principal and validates input before delegating to the store. The
// concrete impl is unexported; DI exposes this interface.
type RelationshipService interface {
	// Create validates + persists an edge for the current principal.
	Create(ctx context.Context, rel Relationship) (Relationship, error)
	// ListForNode returns every edge touching (kind,id) for the current principal.
	ListForNode(ctx context.Context, kind, id string) ([]Relationship, error)
	// Delete removes an edge by id for the current principal.
	Delete(ctx context.Context, id string) error
}

type relationshipService struct {
	store RelationshipStore
}

func NewRelationshipService(store RelationshipStore) RelationshipService {
	return &relationshipService{store: store}
}

func (s *relationshipService) Create(ctx context.Context, rel Relationship) (Relationship, error) {
	p, err := coreservice.RequirePrincipal(ctx)
	if err != nil {
		return Relationship{}, err
	}
	if !RelationshipKinds[rel.SrcKind] || !RelationshipKinds[rel.DstKind] {
		return Relationship{}, coreservice.BadRequest("srcKind and dstKind must each be one of: artifact, record, insight")
	}
	if !RelationshipTypes[rel.RelType] {
		return Relationship{}, coreservice.BadRequest("relType must be one of: cites, supports, contradicts, about, derived_from")
	}
	if strings.TrimSpace(rel.SrcID) == "" || strings.TrimSpace(rel.DstID) == "" {
		return Relationship{}, coreservice.BadRequest("srcId and dstId are required")
	}
	rel.UserID = p.UserID
	return s.store.Create(ctx, rel)
}

func (s *relationshipService) ListForNode(ctx context.Context, kind, id string) ([]Relationship, error) {
	p, err := coreservice.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if !RelationshipKinds[kind] {
		return nil, coreservice.BadRequest("kind must be one of: artifact, record, insight")
	}
	if strings.TrimSpace(id) == "" {
		return nil, coreservice.BadRequest("id is required")
	}
	return s.store.ListForNode(ctx, p.UserID, kind, id)
}

func (s *relationshipService) Delete(ctx context.Context, id string) error {
	p, err := coreservice.RequirePrincipal(ctx)
	if err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return coreservice.BadRequest("id is required")
	}
	return s.store.Delete(ctx, p.UserID, id)
}
