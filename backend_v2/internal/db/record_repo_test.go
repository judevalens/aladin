package db

import (
	"context"
	"testing"

	"aladin/backend_v2/internal/dbtest"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRecordRepositorySaveCompleteIgnoresStaleRevision(t *testing.T) {
	t.Parallel()

	dsn := dbtest.RequireTestDSN(t)

	ctx := context.Background()
	pool, err := Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	externalID := "t3_revision-test-" + uuid.NewString()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM records WHERE external_id = $1`, externalID)
	})

	repo := NewRecordRepository(pool)
	if err := repo.SaveComplete(ctx, completedRevisionRecord("record-new", externalID, 10, "newer content")); err != nil {
		t.Fatalf("save newer revision: %v", err)
	}
	if err := repo.SaveComplete(ctx, completedRevisionRecord("record-new", externalID, 9, "stale content")); err != nil {
		t.Fatalf("save stale revision: %v", err)
	}
	assertStoredRecordRevision(t, pool, "record-new", 10, "newer content")

	if err := repo.SaveComplete(ctx, completedRevisionRecord("record-new", externalID, 11, "latest content")); err != nil {
		t.Fatalf("save latest revision: %v", err)
	}
	assertStoredRecordRevision(t, pool, "record-new", 11, "latest content")
}

func completedRevisionRecord(id string, externalID string, revision int64, content string) *CompletedRecord {
	return &CompletedRecord{
		ID:             id,
		ExternalID:     externalID,
		SourceRevision: revision,
		Type:           "post",
		Label:          "Revision test",
		Content:        content,
		SourceURL:      "https://example.test/post",
		Metadata:       map[string]any{"platform": "reddit"},
		Enrichment:     []byte(`{"summary":"ok"}`),
		Embedding:      zeroEmbedding(),
	}
}

func assertStoredRecordRevision(t *testing.T, pool *pgxpool.Pool, recordID string, wantRevision int64, wantContent string) {
	t.Helper()

	var gotRevision int64
	var gotContent string
	err := pool.QueryRow(context.Background(), `
		SELECT source_revision, content
		  FROM records
		 WHERE id = $1
	`, recordID).Scan(&gotRevision, &gotContent)
	if err != nil {
		t.Fatalf("query stored record: %v", err)
	}
	if gotRevision != wantRevision || gotContent != wantContent {
		t.Fatalf("stored record = revision %d content %q, want revision %d content %q", gotRevision, gotContent, wantRevision, wantContent)
	}
}

func zeroEmbedding() []float32 {
	embedding := make([]float32, 1536)
	return embedding
}
