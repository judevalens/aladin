package claims

import (
	"context"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/llm"
)

// fakeGrounding returns a fixed entity set for any artifact (a page's tags/@mentions).
type fakeGrounding struct {
	refs []db.EntityRef
	err  error
}

func (f fakeGrounding) EntitiesForArtifact(_ context.Context, _ string) ([]db.EntityRef, error) {
	return f.refs, f.err
}

// TestAuthoredExtractor_GroundsOnTags verifies the P3 bridge: a page's tagged entities
// become the grounding set, and extracted claims are stored against the artifact.
func TestAuthoredExtractor_GroundsOnTags(t *testing.T) {
	store := newFakeClaimStore()
	ext := fakeExtractor{claims: []llm.ExtractedClaim{
		{Text: "OpenAI is burning cash unsustainably", Polarity: "assert", Contestable: true, SubjectNames: []string{"OpenAI"}},
	}}
	svc := NewService(store, ext)
	grounding := fakeGrounding{refs: []db.EntityRef{{ID: "e-openai", Name: "OpenAI"}}}

	n, err := NewAuthoredExtractor(svc, grounding).ForArtifact(
		context.Background(), "artifact-xyz", "owner-1", "OpenAI is burning cash unsustainably.",
	)
	if err != nil {
		t.Fatalf("ForArtifact: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 claim stored, got %d", n)
	}
	if len(store.mentions) != 1 {
		t.Fatalf("expected 1 claim mention, got %+v", store.mentions)
	}
	m := store.mentions[0]
	if m.SourceKind != "artifact" || m.SourceID != "artifact-xyz" {
		t.Fatalf("claim must be grounded in the artifact, got %+v", m)
	}
}

// TestAuthoredExtractor_NoEntitiesNoOp: a page with no tags/@mentions extracts nothing
// (the entity-grounded gate has nothing to ground on).
func TestAuthoredExtractor_NoEntitiesNoOp(t *testing.T) {
	store := newFakeClaimStore()
	ext := fakeExtractor{claims: []llm.ExtractedClaim{
		{Text: "should never be reached", Polarity: "assert", Contestable: true, SubjectNames: []string{"OpenAI"}},
	}}
	svc := NewService(store, ext)
	grounding := fakeGrounding{refs: nil}

	n, err := NewAuthoredExtractor(svc, grounding).ForArtifact(context.Background(), "artifact-empty", "owner-1", "text")
	if err != nil {
		t.Fatalf("ForArtifact: %v", err)
	}
	if n != 0 || store.created != 0 {
		t.Fatalf("expected no extraction without entities, got n=%d created=%d", n, store.created)
	}
}
