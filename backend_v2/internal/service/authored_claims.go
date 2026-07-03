package service

import "context"

// ClaimConnection is one of a page's freshly-extracted claims together with the discovered
// evidence for/against it — the Y3 "what supports / contradicts this hunch" payload.
type ClaimConnection struct {
	ClaimID    string `json:"claimId"`
	Text       string `json:"text"`
	Polarity   string `json:"polarity"`
	Support    int    `json:"support"`    // distinct sources that assert the proposition
	Contradict int    `json:"contradict"` // distinct sources that deny it
}

// ConnectResult is the Y3 manual-trigger payload: claims stored this run + the connections
// surfaced back on the page's claims.
type ConnectResult struct {
	ClaimsStored int               `json:"claimsStored"`
	Connections  []ClaimConnection `json:"connections"`
}

// AuthoredClaimsService extracts contestable claims from a page, grounded in the entities
// the page is tagged with / @mentions (P3). Implemented by claims.AuthoredExtractor;
// degrades to a no-op when the page has no entities or no LLM extractor is configured.
type AuthoredClaimsService interface {
	ForArtifact(ctx context.Context, artifactID, ownerID, text string) (int, error)
	// Connect runs authored extraction (Y1's job, forced) and then surfaces the
	// support/contradiction on the page's claims (Y3). It's the manual "Connect" path:
	// write a hunch, see what agrees and disagrees.
	Connect(ctx context.Context, artifactID, ownerID, text string) (ConnectResult, error)
}
