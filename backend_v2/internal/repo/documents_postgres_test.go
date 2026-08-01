package repo

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aladin/backend_v2/internal/ingestion"

	"github.com/google/uuid"
)

// TestIngestion_SweepEndToEnd drives the whole backend loop the way the worker does:
// an uploaded PDF is claimed, extracted, persisted with its outline, and emits the
// artifact's node frame so open surfaces refresh through the syncer.
//
// The fixture is the same real PDF the extractor tests use, copied into the store's
// upload directory — this is the one test that proves the pieces line up (storageKey ->
// resolved path -> extractor -> rows -> frame), which no unit test can.
func TestIngestion_SweepEndToEnd(t *testing.T) {
	ctx := context.Background()
	ctxTO, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	pool := mustTestPool(ctxTO, t)
	defer pool.Close()

	tag := uuid.NewString()[:8]
	userID := uuid.NewString()
	artifactID := "artifact-" + uuid.NewString()
	ctx = adminContext(userID)

	// A real store rooted at a temp dir, holding a real PDF under a real storage key.
	uploads := t.TempDir()
	files := NewFilesystemArtifactStore(uploads, t.TempDir())
	storageKey := "file/" + artifactID + ".pdf"
	source, err := os.ReadFile(filepath.Join("..", "ingestion", "testdata", "outlined.pdf"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(uploads, artifactID+".pdf"), source, 0o644); err != nil {
		t.Fatalf("stage upload: %v", err)
	}

	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, created_at) VALUES ($1::uuid, $2, now())`,
		userID, "ing-"+tag+"@test.local"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO artifacts (id, user_id, type, title, content, metadata)
		VALUES ($1, $2::uuid, 'file', $3, '', jsonb_build_object(
		    'storageKey', $4::text, 'mimeType', 'application/pdf', 'originalFilename', 'book.pdf'))
	`, artifactID, userID, "Drift "+tag, storageKey); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	// The frame producer needs the artifact's tree_nodes row.
	if _, err := pool.Exec(ctx, `
		INSERT INTO tree_nodes (id, user_id, kind, artifact_id, position, created_at, updated_at)
		VALUES ($1, $2::uuid, 'artifact', $1, 0, now(), now())
	`, artifactID, userID); err != nil {
		t.Fatalf("seed node: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM tree_nodes WHERE user_id = $1::uuid`, userID)
		_, _ = pool.Exec(bg, `DELETE FROM artifacts WHERE user_id = $1::uuid`, userID)
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE id = $1::uuid`, userID)
	})

	docRepo := NewDocumentPostgres(pool, files)
	sweeper := ingestion.NewSweeper(docRepo, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))

	if _, err := sweeper.Sweep(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	doc, err := docRepo.GetDocument(ctx, artifactID, true)
	if err != nil {
		t.Fatalf("get document: %v", err)
	}
	if doc.Status != string(ingestion.StatusReady) {
		t.Fatalf("status = %q (%s), want ready", doc.Status, doc.Error)
	}
	if doc.PageCount != 10 {
		t.Fatalf("page count = %d, want 10", doc.PageCount)
	}
	if len(doc.Sections) != 7 {
		t.Fatalf("sections = %d, want the PDF's 7 bookmarks", len(doc.Sections))
	}
	// Outline order and nesting have to survive the round trip through Postgres.
	if doc.Sections[0].Title != "Preface" || doc.Sections[2].Level != 1 {
		t.Fatalf("outline lost its shape: %+v", doc.Sections[:3])
	}
	if len(doc.Pages) != 10 || !strings.Contains(doc.Pages[4].Text, "Beginnings") {
		t.Fatalf("page text missing or misaligned")
	}

	// Status-only reads must not drag the text along — that's the whole reason the API
	// makes it opt-in.
	light, err := docRepo.GetDocument(ctx, artifactID, false)
	if err != nil {
		t.Fatalf("get light: %v", err)
	}
	if len(light.Pages) != 0 {
		t.Fatalf("light read carried %d pages of text", len(light.Pages))
	}
	if len(light.Sections) != 7 {
		t.Fatal("the outline is light enough to always come along")
	}

	// A second sweep must be a no-op: the claim is the INSERT, so an already-ingested
	// artifact is never picked up twice.
	claimed, err := docRepo.ClaimPending(ctx, 10)
	if err != nil {
		t.Fatalf("second claim: %v", err)
	}
	for _, item := range claimed {
		if item.ArtifactID == artifactID {
			t.Fatal("an ingested artifact was claimed again")
		}
	}

	// The write emitted the artifact's node frame, so the tree learns about the status
	// change through the syncer rather than by polling.
	var payload string
	if err := pool.QueryRow(ctx, `
		SELECT payload::text FROM outbox_events
		 WHERE user_id = $1::uuid AND type = 'data_event'
		 ORDER BY xid DESC, created_at DESC LIMIT 1
	`, userID).Scan(&payload); err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	if !strings.Contains(payload, artifactID) {
		t.Fatalf("no node frame for the ingested artifact: %s", payload)
	}
}

// A file we can't extract must still leave a terminal status. A row stuck on 'ingesting'
// is indistinguishable from a hang, which is exactly what the status model exists to
// prevent (INGESTION_PRD §4).
func TestIngestion_UnreadableFileStillLandsTerminal(t *testing.T) {
	ctx := context.Background()
	ctxTO, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	pool := mustTestPool(ctxTO, t)
	defer pool.Close()

	tag := uuid.NewString()[:8]
	userID := uuid.NewString()
	artifactID := "artifact-" + uuid.NewString()
	ctx = adminContext(userID)

	uploads := t.TempDir()
	files := NewFilesystemArtifactStore(uploads, t.TempDir())
	if err := os.WriteFile(filepath.Join(uploads, artifactID+".pdf"), []byte("not a pdf at all"), 0o644); err != nil {
		t.Fatalf("stage upload: %v", err)
	}

	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, created_at) VALUES ($1::uuid, $2, now())`,
		userID, "bad-"+tag+"@test.local"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO artifacts (id, user_id, type, title, content, metadata)
		VALUES ($1, $2::uuid, 'file', 'Broken', '', jsonb_build_object(
		    'storageKey', $3::text, 'mimeType', 'application/pdf', 'originalFilename', 'broken.pdf'))
	`, artifactID, userID, "file/"+artifactID+".pdf"); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO tree_nodes (id, user_id, kind, artifact_id, position, created_at, updated_at)
		VALUES ($1, $2::uuid, 'artifact', $1, 0, now(), now())
	`, artifactID, userID); err != nil {
		t.Fatalf("seed node: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM tree_nodes WHERE user_id = $1::uuid`, userID)
		_, _ = pool.Exec(bg, `DELETE FROM artifacts WHERE user_id = $1::uuid`, userID)
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE id = $1::uuid`, userID)
	})

	docRepo := NewDocumentPostgres(pool, files)
	sweeper := ingestion.NewSweeper(docRepo, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	if _, err := sweeper.Sweep(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	doc, err := docRepo.GetDocument(ctx, artifactID, false)
	if err != nil {
		t.Fatalf("get document: %v", err)
	}
	if doc.Status == string(ingestion.StatusIngesting) || doc.Status == string(ingestion.StatusPending) {
		t.Fatalf("status = %q — a failure must be terminal, not a permanent spinner", doc.Status)
	}
	if doc.Status != string(ingestion.StatusFailed) {
		t.Fatalf("status = %q, want failed", doc.Status)
	}
	if doc.Error == "" {
		t.Fatal("a failure with no reason is undebuggable")
	}
}
