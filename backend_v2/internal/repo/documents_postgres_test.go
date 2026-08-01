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
	artifactservice "aladin/backend_v2/internal/service"

	"github.com/google/uuid"
)

// realSegmenter points at the repo's tool. Tests run from internal/repo/, so the paths
// are explicit rather than relying on the worker's relative default.
//
// This deliberately exercises the REAL script: the whole point of the test is that
// storageKey -> path -> Python -> rows -> frame lines up, and a fake segmenter would
// verify none of it. It skips where the venv isn't installed rather than failing, so a
// machine without the tool still runs the rest of the suite.
func realSegmenter(t *testing.T) *ingestion.PythonSegmenter {
	t.Helper()
	root := filepath.Join("..", "..", "..", "tools", "doclayout")
	seg := &ingestion.PythonSegmenter{
		Python:  filepath.Join(root, ".venv", "bin", "python"),
		Script:  filepath.Join(root, "segment.py"),
		Timeout: 5 * time.Minute,
	}
	if err := seg.Available(); err != nil {
		t.Skipf("layout tool not installed: %v", err)
	}
	return seg
}

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
	sweeper := ingestion.NewSweeper(docRepo, realSegmenter(t), slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))

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

	// Layout regions persisted — the substrate both surfaces sit on (§13e). Their bboxes
	// are in PDF points and must land inside the page, because an out-of-page box anchors
	// to nothing and §13d spends no error budget on anchors.
	regions, err := docRepo.GetRegions(ctx, artifactID, "")
	if err != nil {
		t.Fatalf("get regions: %v", err)
	}
	if len(regions) == 0 {
		t.Fatal("no layout regions persisted — the segmentation pass produced nothing")
	}
	for _, region := range regions {
		if region.Page < 1 || region.Page > doc.PageCount {
			t.Fatalf("region on page %d, outside 1..%d", region.Page, doc.PageCount)
		}
		if len(region.Bbox) != 4 {
			t.Fatalf("region %+v has no usable box", region)
		}
		if region.Bbox[2] <= region.Bbox[0] || region.Bbox[3] <= region.Bbox[1] {
			t.Fatalf("region %+v has an inverted box", region)
		}
		if region.Class == "" {
			t.Fatalf("region %+v has no class", region)
		}
	}
	// The fixture has headings, so at least one must have been recognised as such —
	// otherwise the model ran but found nothing useful.
	titles, err := docRepo.GetRegions(ctx, artifactID, "title")
	if err != nil {
		t.Fatalf("get title regions: %v", err)
	}
	if len(titles) == 0 {
		t.Fatalf("no title regions among %d — class filtering or the model is broken", len(regions))
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
	sweeper := ingestion.NewSweeper(docRepo, realSegmenter(t), slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
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

// The chunk tree (INGESTION_PRD §11) is what makes a document navigable when it shipped
// no bookmarks of its own. This asserts it survives the round trip through Postgres —
// including the self-referencing key, which only resolves because parents are written
// before their children.
func TestIngestion_ChunkTreeRoundTrip(t *testing.T) {
	ctx := context.Background()
	ctxTO, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	pool := mustTestPool(ctxTO, t)
	defer pool.Close()

	tag := uuid.NewString()[:8]
	userID := uuid.NewString()
	artifactID := "artifact-" + uuid.NewString()
	ctx = adminContext(userID)

	uploads := t.TempDir()
	files := NewFilesystemArtifactStore(uploads, t.TempDir())
	source, err := os.ReadFile(filepath.Join("..", "ingestion", "testdata", "outlined.pdf"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(uploads, artifactID+".pdf"), source, 0o644); err != nil {
		t.Fatalf("stage upload: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, created_at) VALUES ($1::uuid, $2, now())`,
		userID, "chk-"+tag+"@test.local"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO artifacts (id, user_id, type, title, content, metadata)
		VALUES ($1, $2::uuid, 'file', 'Chunked', '', jsonb_build_object(
		    'storageKey', $3::text, 'mimeType', 'application/pdf', 'originalFilename', 'book.pdf'))
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
	sweeper := ingestion.NewSweeper(docRepo, realSegmenter(t),
		slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	if _, err := sweeper.Sweep(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	tree, err := docRepo.GetChunkTree(ctx, artifactID, true)
	if err != nil {
		t.Fatalf("get chunk tree: %v", err)
	}
	if len(tree) == 0 {
		t.Fatal("no chunks produced — regions were persisted but nothing consumed them")
	}

	// Anchors again: every chunk must sit inside the document, and a child inside its
	// parent. A span that escapes is a citation pointing somewhere false.
	var pageCount int
	if err := pool.QueryRow(ctx, `SELECT page_count FROM artifact_documents WHERE artifact_id = $1`,
		artifactID).Scan(&pageCount); err != nil {
		t.Fatalf("page count: %v", err)
	}
	var walk func(chunk artifactservice.DocumentChunk, parent *artifactservice.DocumentChunk)
	seen := 0
	walk = func(chunk artifactservice.DocumentChunk, parent *artifactservice.DocumentChunk) {
		seen++
		if chunk.PageFrom < 1 || chunk.PageTo > pageCount {
			t.Fatalf("chunk %q spans %d–%d, outside 1..%d", chunk.Title, chunk.PageFrom, chunk.PageTo, pageCount)
		}
		if parent != nil && (chunk.PageFrom < parent.PageFrom || chunk.PageTo > parent.PageTo) {
			t.Fatalf("child %q (%d–%d) escapes parent %q (%d–%d)",
				chunk.Title, chunk.PageFrom, chunk.PageTo, parent.Title, parent.PageFrom, parent.PageTo)
		}
		if chunk.Kind != "section" && chunk.Kind != "block" {
			t.Fatalf("chunk %q has kind %q", chunk.Title, chunk.Kind)
		}
		for _, child := range chunk.Children {
			walk(child, &chunk)
		}
	}
	for _, chunk := range tree {
		walk(chunk, nil)
	}
	if seen < 2 {
		t.Fatalf("only %d chunk(s) — the fixture has headings and body", seen)
	}

	// The outline read omits text: navigating shouldn't cost what reading costs.
	light, err := docRepo.GetChunkTree(ctx, artifactID, false)
	if err != nil {
		t.Fatalf("get outline: %v", err)
	}
	for _, chunk := range light {
		if chunk.Text != "" {
			t.Fatalf("outline carried body text for %q", chunk.Title)
		}
	}
}
