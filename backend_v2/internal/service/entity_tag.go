package service

import (
	"context"
	"errors"
	"strings"
)

// EntityHit is a typeahead match for the tag / @entity picker.
type EntityHit struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Scope     string `json:"scope"`
	TrustTier string `json:"trustTier"`
	// Aliases are the entity's other known surfaces (synonyms), shown in the picker so
	// "NVDA · Nvidia, NVIDIA Corp" reads as one thing. Excludes the canonical name.
	Aliases []string `json:"aliases"`
}

// AttachedEntity is an entity linked to an artifact (a tag or a projected @mention).
type AttachedEntity struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Origin  string `json:"origin"` // tag | mention
	BlockID string `json:"blockId,omitempty"`
}

// CreateEntityInput mints a new canonical entity from the "create new" typeahead path.
type CreateEntityInput struct {
	Name string
	Kind string
}

// AttachEntityInput links an existing entity to an artifact as a tag.
type AttachEntityInput struct {
	ArtifactID string
	EntityID   string
	AddedBy    string // user id; "" if unauthenticated
}

// MentionRef is one projected @entity occurrence in a page: the entity referenced, the
// block it sits in, the literal label shown, and the block's plain text.
type MentionRef struct {
	EntityID string `json:"entityId"`
	BlockID  string `json:"blockId"`
	Surface  string `json:"surface"`
	// Snippet is the block's text, verbatim — what the Entity Context surface renders as
	// "your note". Without it we'd know a page mentions an entity but not what it said.
	Snippet string `json:"snippet"`
}

var ErrInvalidEntityTag = errors.New("invalid entity tag request")

type EntityTagService interface {
	Search(ctx context.Context, ownerUserID, query string, limit int) ([]EntityHit, error)
	CreateEntity(ctx context.Context, in CreateEntityInput) (EntityHit, error)
	Attach(ctx context.Context, in AttachEntityInput) error
	Detach(ctx context.Context, artifactID, entityID string) error
	ListForArtifact(ctx context.Context, artifactID string) ([]AttachedEntity, error)
	// SyncMentions reconciles the projected @entity mentions for an artifact (P2):
	// the given set replaces all existing origin='mention' rows for that page.
	SyncMentions(ctx context.Context, artifactID string, mentions []MentionRef) error
}

type EntityTagRepository interface {
	SearchEntities(ctx context.Context, ownerUserID, query string, limit int) ([]EntityHit, error)
	CreateEntity(ctx context.Context, kind, canonicalName, normalizedKey string) (EntityHit, error)
	AttachTag(ctx context.Context, artifactID, entityID, addedBy string) error
	DetachTag(ctx context.Context, artifactID, entityID string) error
	ListForArtifact(ctx context.Context, artifactID string) ([]AttachedEntity, error)
	ReplaceMentions(ctx context.Context, artifactID string, mentions []MentionRef) error
}

// Normalizer reduces an entity surface form to its blocking key (entities.Normalize),
// so entities minted here dedupe with resolver-created ones.
type Normalizer func(surface string) string

type DefaultEntityTagService struct {
	repo      EntityTagRepository
	normalize Normalizer
}

func NewEntityTagService(repo EntityTagRepository, normalize Normalizer) *DefaultEntityTagService {
	return &DefaultEntityTagService{repo: repo, normalize: normalize}
}

const entityTagSearchMaxLimit = 20

func (s *DefaultEntityTagService) Search(ctx context.Context, ownerUserID, query string, limit int) ([]EntityHit, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []EntityHit{}, nil
	}
	if limit <= 0 || limit > entityTagSearchMaxLimit {
		limit = entityTagSearchMaxLimit
	}
	return s.repo.SearchEntities(ctx, ownerUserID, query, limit)
}

func (s *DefaultEntityTagService) CreateEntity(ctx context.Context, in CreateEntityInput) (EntityHit, error) {
	name := strings.TrimSpace(in.Name)
	kind := strings.TrimSpace(in.Kind)
	if name == "" {
		return EntityHit{}, ErrInvalidEntityTag
	}
	// "unknown" is retired (00020); map it and empty to the generic "other" so legacy
	// callers keep working. Kind is usually backfilled later by the judge anyway —
	// create is deliberately a zero-decision path (P1.2).
	if kind == "" || kind == "unknown" {
		kind = "other"
	}
	key := s.normalize(name)
	if key == "" {
		return EntityHit{}, ErrInvalidEntityTag
	}
	return s.repo.CreateEntity(ctx, kind, name, key)
}

func (s *DefaultEntityTagService) Attach(ctx context.Context, in AttachEntityInput) error {
	if strings.TrimSpace(in.ArtifactID) == "" || strings.TrimSpace(in.EntityID) == "" {
		return ErrInvalidEntityTag
	}
	return s.repo.AttachTag(ctx, in.ArtifactID, in.EntityID, in.AddedBy)
}

func (s *DefaultEntityTagService) Detach(ctx context.Context, artifactID, entityID string) error {
	if strings.TrimSpace(artifactID) == "" || strings.TrimSpace(entityID) == "" {
		return ErrInvalidEntityTag
	}
	return s.repo.DetachTag(ctx, artifactID, entityID)
}

func (s *DefaultEntityTagService) ListForArtifact(ctx context.Context, artifactID string) ([]AttachedEntity, error) {
	return s.repo.ListForArtifact(ctx, artifactID)
}

func (s *DefaultEntityTagService) SyncMentions(ctx context.Context, artifactID string, mentions []MentionRef) error {
	if strings.TrimSpace(artifactID) == "" {
		return ErrInvalidEntityTag
	}
	// Keep only well-formed mentions (an entity must be set); dedupe on (entity, block).
	seen := make(map[string]bool, len(mentions))
	clean := make([]MentionRef, 0, len(mentions))
	for _, m := range mentions {
		if strings.TrimSpace(m.EntityID) == "" {
			continue
		}
		key := m.EntityID + "\x00" + m.BlockID
		if seen[key] {
			continue
		}
		seen[key] = true
		clean = append(clean, m)
	}
	return s.repo.ReplaceMentions(ctx, artifactID, clean)
}
