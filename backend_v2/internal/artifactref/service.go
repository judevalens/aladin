package artifactref

import (
	"context"
	"errors"
	"strings"
)

// Reference target kinds for the `#` picker.
// pages and shards are navigational artifact links (artifacts.type 'page' / 'app').
const (
	RefKindPage  = "page"
	RefKindShard = "shard"
)

// RefHit is one typeahead match for the `#` reference picker (a page or shard).
// The picker sections results by Kind.
type RefHit struct {
	Kind   string `json:"kind"`             // page | shard
	ID     string `json:"id"`               // artifact id
	Label  string `json:"label"`            // artifact title
	Detail string `json:"detail,omitempty"` // claim polarity, or "" for artifacts
}

// ArtifactRef is one projected `#` reference occurrence in a page: the target referenced,
// the block it sits in, and the literal label shown in the chip.
type ArtifactRef struct {
	Kind     string `json:"kind"` // page | shard
	TargetID string `json:"targetId"`
	BlockID  string `json:"blockId"`
	Surface  string `json:"surface"`
}

// AttachedRef is a ref linked to an artifact, resolved for display (hydration / backlinks).
type AttachedRef struct {
	Kind     string `json:"kind"`
	TargetID string `json:"targetId"`
	Label    string `json:"label"`
	BlockID  string `json:"blockId,omitempty"`
}

var ErrInvalidArtifactRef = errors.New("invalid artifact ref request")

// ArtifactRefService backs the `#` cross-reference picker: search across
// artifacts, and the reconcile of a page's projected refs.
type ArtifactRefService interface {
	// Search returns up to perKind matches for each target kind (pages, shards),
	// concatenated. ownerUserID scopes the user's artifacts.
	Search(ctx context.Context, ownerUserID, query string, perKind int) ([]RefHit, error)
	// SyncRefs reconciles the projected `#` refs for a page: the given set replaces all
	// existing origin='reference' rows for that artifact.
	SyncRefs(ctx context.Context, artifactID string, refs []ArtifactRef) error
	// ListForArtifact returns a page's outgoing refs, resolved to current labels.
	ListForArtifact(ctx context.Context, artifactID string) ([]AttachedRef, error)
}

type ArtifactRefRepository interface {
	SearchArtifacts(ctx context.Context, ownerUserID, query string, limit int) ([]RefHit, error)
	ReplaceRefs(ctx context.Context, artifactID string, refs []ArtifactRef) error
	ListForArtifact(ctx context.Context, artifactID string) ([]AttachedRef, error)
}

const artifactRefSearchMaxPerKind = 8

var validRefKinds = map[string]bool{RefKindPage: true, RefKindShard: true}

type DefaultArtifactRefService struct {
	repo ArtifactRefRepository
}

func NewArtifactRefService(repo ArtifactRefRepository) *DefaultArtifactRefService {
	return &DefaultArtifactRefService{repo: repo}
}

func (s *DefaultArtifactRefService) Search(ctx context.Context, ownerUserID, query string, perKind int) ([]RefHit, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []RefHit{}, nil
	}
	if perKind <= 0 || perKind > artifactRefSearchMaxPerKind {
		perKind = artifactRefSearchMaxPerKind
	}
	// The claim layer was removed, so the picker offers pages + shards only.
	return s.repo.SearchArtifacts(ctx, ownerUserID, query, perKind)
}

func (s *DefaultArtifactRefService) SyncRefs(ctx context.Context, artifactID string, refs []ArtifactRef) error {
	if strings.TrimSpace(artifactID) == "" {
		return ErrInvalidArtifactRef
	}
	// Keep only well-formed refs (valid kind + target set); dedupe on (kind, target, block).
	seen := make(map[string]bool, len(refs))
	clean := make([]ArtifactRef, 0, len(refs))
	for _, r := range refs {
		if !validRefKinds[r.Kind] || strings.TrimSpace(r.TargetID) == "" {
			continue
		}
		key := r.Kind + "\x00" + r.TargetID + "\x00" + r.BlockID
		if seen[key] {
			continue
		}
		seen[key] = true
		clean = append(clean, r)
	}
	return s.repo.ReplaceRefs(ctx, artifactID, clean)
}

func (s *DefaultArtifactRefService) ListForArtifact(ctx context.Context, artifactID string) ([]AttachedRef, error) {
	if strings.TrimSpace(artifactID) == "" {
		return nil, ErrInvalidArtifactRef
	}
	return s.repo.ListForArtifact(ctx, artifactID)
}
