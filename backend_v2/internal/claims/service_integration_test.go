package claims

import (
	"context"
	"strings"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/dbtest"
	"aladin/backend_v2/internal/llm"

	"github.com/google/uuid"
)

// TestService_AgainstPostgres exercises C0 end-to-end against the sandbox: the 00011
// migration applies, a contestable claim grounded on a real entity is stored with its
// subject + asserting mention, and a re-run is idempotent.
func TestService_AgainstPostgres(t *testing.T) {
	dsn := dbtest.RequireTestDSN(t)
	ctx := context.Background()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate (00011 claim layer must apply): %v", err)
	}

	tag := strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
	entName := "novacorp" + tag
	claimText := "the approach of " + entName + " will win"
	recordID := "claim-test-" + uuid.NewString()

	entityRepo := db.NewEntityRepository(pool)
	entityID, err := entityRepo.CreateSharedEntity(ctx, db.CreateEntityParams{Kind: "unknown", CanonicalName: entName, NormalizedKey: entName})
	if err != nil {
		t.Fatalf("create entity: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM claims WHERE canonical_text = $1`, claimText)
		_, _ = pool.Exec(context.Background(), `DELETE FROM entities WHERE id = $1::uuid`, entityID)
	})

	ext := fakeExtractor{claims: []llm.ExtractedClaim{
		{Text: claimText, Polarity: "assert", Contestable: true, SubjectNames: []string{entName}},
	}}
	svc := NewService(db.NewClaimRepository(pool), ext)
	in := RecordInput{
		RecordID:  recordID,
		Summary:   "a post",
		KeyClaims: []string{"raw point"},
		Entities:  []db.EntityRef{{ID: entityID, Name: entName}},
	}

	n, err := svc.ExtractFromRecord(ctx, in)
	if err != nil || n != 1 {
		t.Fatalf("expected 1 claim stored, got n=%d err=%v", n, err)
	}

	var claimID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM claims WHERE canonical_text = $1`, claimText).Scan(&claimID); err != nil {
		t.Fatalf("claim not stored: %v", err)
	}
	var subjects, mentions int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM claim_subjects WHERE claim_id = $1::uuid AND entity_id = $2::uuid`, claimID, entityID).Scan(&subjects); err != nil {
		t.Fatalf("count subjects: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM claim_mentions WHERE claim_id = $1::uuid AND source_id = $2 AND stance = 'assert'`, claimID, recordID).Scan(&mentions); err != nil {
		t.Fatalf("count mentions: %v", err)
	}
	if subjects != 1 {
		t.Fatalf("expected the claim grounded on the entity, got %d subjects", subjects)
	}
	if mentions != 1 {
		t.Fatalf("expected 1 asserting mention from the record, got %d", mentions)
	}

	// Idempotent re-run: same record + same text → no duplicate claim or mention.
	if _, err := svc.ExtractFromRecord(ctx, in); err != nil {
		t.Fatalf("re-run: %v", err)
	}
	var claimCount, mentionCount int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM claims WHERE canonical_text = $1`, claimText).Scan(&claimCount)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM claim_mentions WHERE claim_id = $1::uuid AND source_id = $2`, claimID, recordID).Scan(&mentionCount)
	if claimCount != 1 || mentionCount != 1 {
		t.Fatalf("re-run must be idempotent, got claims=%d mentions=%d", claimCount, mentionCount)
	}
}

// fixedEmbedder gives every claim the same 1536-d vector, so proposition-matching is
// decided by the (faked) adjudicator, not by cosine — the deterministic way to test the
// contradiction surface against real pgvector.
type fixedEmbedder struct{}

func (fixedEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	v := make([]float32, 1536)
	v[0] = 1
	return v, nil
}

// TestService_ContradictionSurface (C2–C4, the hero): a tenant verified thesis, then
// discovered claims that assert / negate / paraphrase it, and the surface tallies
// support vs contradict from the discovered sources.
func TestService_ContradictionSurface(t *testing.T) {
	dsn := dbtest.RequireTestDSN(t)
	ctx := context.Background()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	tag := strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
	entName := "acme" + tag
	owner := uuid.NewString()
	repo := db.NewClaimRepository(pool)
	entityRepo := db.NewEntityRepository(pool)
	entityID, err := entityRepo.CreateSharedEntity(ctx, db.CreateEntityParams{Kind: "unknown", CanonicalName: entName, NormalizedKey: entName})
	if err != nil {
		t.Fatalf("create entity: %v", err)
	}
	grounding := []db.EntityRef{{ID: entityID, Name: entName}}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM claims WHERE id IN (SELECT claim_id FROM claim_subjects WHERE entity_id = $1::uuid)`, entityID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM entities WHERE id = $1::uuid`, entityID)
	})

	// helper: a service that extracts exactly one claim with the given text/polarity and
	// the given adjudicator verdict.
	svc := func(text, polarity, verdict string) *Service {
		ext := fakeExtractor{claims: []llm.ExtractedClaim{
			{Text: text, Polarity: polarity, Contestable: true, SubjectNames: []string{entName}},
		}}
		return NewService(repo, ext).WithEmbedder(fixedEmbedder{}).WithAdjudicator(fakeClaimAdj{rel: verdict})
	}

	// The tenant's thesis (verified).
	if _, err := svc(entName+" will win", "assert", "same").ExtractFromAuthored(ctx, AuthoredInput{
		OwnerID: owner, ArtifactID: "note-1", Summary: "my note", KeyClaims: []string{"x"}, Entities: grounding,
	}); err != nil {
		t.Fatalf("authored: %v", err)
	}
	var thesisID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM claims WHERE scope='tenant' AND owner_user_id=$1::uuid`, owner).Scan(&thesisID); err != nil {
		t.Fatalf("thesis not stored: %v", err)
	}

	// Discovered evidence: two sources assert it, one denies it (negation).
	if _, err := svc(entName+" will win", "assert", "same").ExtractFromRecord(ctx, RecordInput{RecordID: "rec-a", Summary: "a", KeyClaims: []string{"x"}, Entities: grounding}); err != nil {
		t.Fatalf("rec-a: %v", err)
	}
	if _, err := svc(entName+" will triumph", "assert", "same").ExtractFromRecord(ctx, RecordInput{RecordID: "rec-b", Summary: "b", KeyClaims: []string{"x"}, Entities: grounding}); err != nil {
		t.Fatalf("rec-b: %v", err)
	}
	if _, err := svc(entName+" will not win", "deny", "negation").ExtractFromRecord(ctx, RecordInput{RecordID: "rec-c", Summary: "c", KeyClaims: []string{"x"}, Entities: grounding}); err != nil {
		t.Fatalf("rec-c: %v", err)
	}

	counts, err := NewService(repo, nil).GetContradictions(ctx, thesisID)
	if err != nil {
		t.Fatalf("contradictions: %v", err)
	}
	if counts.Support != 2 || counts.Contradict != 1 {
		t.Fatalf("expected support=2 contradict=1, got support=%d contradict=%d", counts.Support, counts.Contradict)
	}
}
