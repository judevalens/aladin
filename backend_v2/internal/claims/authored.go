package claims

import (
	"context"
	"strings"

	"aladin/backend_v2/internal/db"
)

// EntityGroundingSource supplies the resolved entities a page is grounded in — its tags +
// @mentions. Satisfied by db.EntityRepository (EntitiesForArtifact).
type EntityGroundingSource interface {
	EntitiesForArtifact(ctx context.Context, artifactID string) ([]db.EntityRef, error)
}

// AuthoredExtractor turns a page's manual entity links (tags + @mentions) into the
// grounding set and runs authored claim extraction over the page text — the deterministic
// floor (tags) feeding the LLM ceiling (claims). It closes the loop the spec describes:
// ExtractFromAuthored needs a resolved entity set, and tags/mentions are exactly that.
type AuthoredExtractor struct {
	svc       *Service
	grounding EntityGroundingSource
}

func NewAuthoredExtractor(svc *Service, grounding EntityGroundingSource) *AuthoredExtractor {
	return &AuthoredExtractor{svc: svc, grounding: grounding}
}

// ForArtifact extracts authored claims for a page. It grounds on the page's tagged /
// @mentioned entities; if the page has none (or extraction is disabled), it's a no-op —
// the same graceful degrade as the record path.
func (a *AuthoredExtractor) ForArtifact(ctx context.Context, artifactID, ownerID, text string) (int, error) {
	if a == nil || a.svc == nil || a.grounding == nil {
		return 0, nil
	}
	refs, err := a.grounding.EntitiesForArtifact(ctx, artifactID)
	if err != nil {
		return 0, err
	}
	if len(refs) == 0 {
		return 0, nil
	}
	return a.svc.ExtractFromAuthored(ctx, AuthoredInput{
		OwnerID:    ownerID,
		ArtifactID: artifactID,
		Summary:    text,
		KeyClaims:  keyClaimCandidates(text),
		Entities:   refs,
	})
}

// keyClaimCandidates splits page text into candidate claim lines (the extraction context).
// Returns nil for empty text, which makes extraction a no-op.
func keyClaimCandidates(text string) []string {
	out := make([]string, 0)
	for _, line := range strings.Split(text, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
