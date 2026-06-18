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
