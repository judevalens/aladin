package service

import "context"

// AuthoredClaimsService extracts contestable claims from a page, grounded in the entities
// the page is tagged with / @mentions (P3). Implemented by claims.AuthoredExtractor;
// degrades to a no-op when the page has no entities or no LLM extractor is configured.
type AuthoredClaimsService interface {
	ForArtifact(ctx context.Context, artifactID, ownerID, text string) (int, error)
}
