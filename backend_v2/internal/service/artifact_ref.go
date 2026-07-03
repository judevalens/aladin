package service

import (
	"context"
	"errors"
	"strings"
)

// Reference target kinds for the `#` picker. Claims are the engine-reasoned thesis layer;
// pages and shards are navigational artifact links (artifacts.type 'page' / 'app').
const (
	RefKindClaim = "claim"
	RefKindPage  = "page"
	RefKindShard = "shard"
)

// RefHit is one typeahead match for the `#` reference picker (a claim, page, or shard).
// The picker sections results by Kind.
type RefHit struct {
	Kind   string `json:"kind"`             // claim | page | shard
	ID     string `json:"id"`               // claim uuid or artifact id
	Label  string `json:"label"`            // claim canonical_text or artifact title
	Detail string `json:"detail,omitempty"` // claim polarity, or "" for artifacts
}

// ArtifactRef is one projected `#` reference occurrence in a page: the target referenced,
// the block it sits in, and the literal label shown in the chip.
type ArtifactRef struct {
	Kind     string `json:"kind"` // claim | page | shard
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

// ArtifactRefService backs the `#` cross-reference picker: unified search across claims +
// artifacts, and the reconcile of a page's projected refs.
type ArtifactRefService interface {
	// Search returns up to perKind matches for each target kind (claims, pages, shards),
	// concatenated. ownerUserID scopes tenant claims + the user's artifacts.
	Search(ctx context.Context, ownerUserID, query string, perKind int) ([]RefHit, error)
	// SyncRefs reconciles the projected `#` refs for a page: the given set replaces all
	// existing origin='reference' rows for that artifact.
	SyncRefs(ctx context.Context, artifactID string, refs []ArtifactRef) error
	// ListForArtifact returns a page's outgoing refs, resolved to current labels.
	ListForArtifact(ctx context.Context, artifactID string) ([]AttachedRef, error)
}

type ArtifactRefRepository interface {
	SearchClaims(ctx context.Context, ownerUserID, query string, limit int) ([]RefHit, error)
	SearchArtifacts(ctx context.Context, ownerUserID, query string, limit int) ([]RefHit, error)
	ReplaceRefs(ctx context.Context, artifactID string, refs []ArtifactRef) error
	ListForArtifact(ctx context.Context, artifactID string) ([]AttachedRef, error)
}

const artifactRefSearchMaxPerKind = 8

var validRefKinds = map[string]bool{RefKindClaim: true, RefKindPage: true, RefKindShard: true}

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
	claims, err := s.repo.SearchClaims(ctx, ownerUserID, query, perKind)
	if err != nil {
		return nil, err
	}
	arts, err := s.repo.SearchArtifacts(ctx, ownerUserID, query, perKind)
	if err != nil {
		return nil, err
	}
	// Claims first (the differentiator), then artifacts; the frontend sections by kind.
	out := make([]RefHit, 0, len(claims)+len(arts))
	out = append(out, claims...)
	out = append(out, arts...)
	return out, nil
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
