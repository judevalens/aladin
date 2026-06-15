package service

import (
	"context"
	"time"
)

// Relationship is a typed edge connecting two workspace/ingestion entities — the
// additive "bridge" between the two worlds (artifacts ↔ records ↔ insights) that
// does NOT unify their tables. Endpoints are (kind, id) pairs; kind is one of
// "artifact" | "record" | "insight". See design/DATA_MODEL.md.
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
