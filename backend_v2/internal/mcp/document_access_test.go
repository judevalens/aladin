package mcpserver

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aladin/backend_v2/internal/dbtest"
	"aladin/backend_v2/internal/document"
	documentpostgres "aladin/backend_v2/internal/document/postgres"
	"aladin/backend_v2/internal/ingestion"
	"aladin/backend_v2/internal/repo"
	"aladin/backend_v2/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestAgentCanReadAnIngestedPDF answers the question the whole feature exists for: after
// a PDF is ingested, can an agent actually read it?
//
// It drives the REAL chain — the Python script, Postgres, and the MCP tools the copilot
// calls — because every layer of it has been wrong at least once, and a fake anywhere
// would prove nothing about the thing being asked.
func TestAgentCanReadAnIngestedPDF(t *testing.T) {
	dsn := dbtest.RequireTestDSN(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	root := filepath.Join("..", "..", "..", "tools", "doclayout")
	segmenter := &ingestion.PythonSegmenter{
		Python:  filepath.Join(root, ".venv", "bin", "python"),
		Script:  filepath.Join(root, "segment.py"),
		Timeout: 2 * time.Minute,
	}
	if err := segmenter.Available(); err != nil {
		t.Skipf("layout tool not installed: %v", err)
	}

	tag := uuid.NewString()[:8]
	userID := uuid.NewString()
	artifactID := "artifact-" + uuid.NewString()
	ctx = service.WithPrincipal(ctx, service.Principal{
		UserID: userID,
		Scopes: []string{service.ScopeArtifactsRead, service.ScopeArtifactsWrite},
	})

	uploads := t.TempDir()
	files := repo.NewFilesystemArtifactStore(uploads, t.TempDir())
	source, err := os.ReadFile(filepath.Join("..", "ingestion", "testdata", "outlined.pdf"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(uploads, artifactID+".pdf"), source, 0o644); err != nil {
		t.Fatalf("stage upload: %v", err)
	}

	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, created_at) VALUES ($1::uuid, $2, now())`,
		userID, "agent-"+tag+"@test.local"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO artifacts (id, user_id, type, title, content, metadata)
		VALUES ($1, $2::uuid, 'file', $3, '', jsonb_build_object(
		    'storageKey', $4::text, 'mimeType', 'application/pdf', 'originalFilename', 'drift.pdf'))
	`, artifactID, userID, "The Shape of Drift", "file/"+artifactID+".pdf"); err != nil {
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

	docRepo := documentpostgres.NewDocumentPostgres(pool, files)
	sweeper := ingestion.NewSweeper(docRepo, segmenter,
		slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	if _, err := sweeper.Sweep(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	tools := workspaceToolServer{
		artifacts: service.NewArtifactService(repo.NewArtifactsPostgres(pool),
			repo.NewFilesystemArtifactStore(uploads, t.TempDir())),
		documents: document.NewDocumentService(docRepo),
	}

	// 1. get_artifact — the agent's first look. It must learn the document is readable
	//    and how to read it, WITHOUT being handed the text (that was the correction).
	_, meta, err := tools.getArtifact(ctx, nil, getArtifactInput{ArtifactID: artifactID})
	if err != nil {
		t.Fatalf("get_artifact: %v", err)
	}
	if meta.Text != "" {
		t.Fatalf("get_artifact leaked document text (%d chars) — looking something up must not cost the document", len(meta.Text))
	}
	if meta.PageCount != 10 {
		t.Fatalf("page count = %d, want 10", meta.PageCount)
	}
	if len(meta.Outline) == 0 {
		t.Fatal("no outline — the agent has no way to decide what to read")
	}
	if !strings.Contains(meta.More, "search_document") {
		t.Fatalf("get_artifact must point at retrieval, got %q", meta.More)
	}

	// 2. search_document — the entry point for a reader with a QUESTION rather than a
	//    page number, which is every reader that isn't already holding the outline.
	_, hits, err := tools.searchDocument(ctx, nil, searchDocumentInput{
		ArtifactID: artifactID, Query: "drift",
	})
	if err != nil {
		t.Fatalf("search_document: %v", err)
	}
	if len(hits.Hits) == 0 {
		t.Fatal("search found nothing in a document that contains the word")
	}
	for _, hit := range hits.Hits {
		if hit.Page < 1 || hit.Page > meta.PageCount {
			t.Fatalf("hit on page %d, outside the document", hit.Page)
		}
		if strings.TrimSpace(hit.Snippet) == "" {
			t.Fatal("a hit with no snippet tells the agent nothing")
		}
	}

	// 3. read_document — expand around a hit. This is the step that has to return real
	//    words, and only the range asked for.
	target := hits.Hits[0].Page
	_, read, err := tools.readDocument(ctx, nil, readDocumentInput{
		ArtifactID: artifactID, FromPage: target, ToPage: target,
	})
	if err != nil {
		t.Fatalf("read_document: %v", err)
	}
	if strings.TrimSpace(read.Text) == "" {
		t.Fatalf("read_document returned nothing for p%d", target)
	}
	if !strings.Contains(read.Text, "[p") {
		t.Fatalf("text carries no page markers, so nothing can be cited: %q", read.Text)
	}
	if read.FromPage != target || read.ToPage != target {
		t.Fatalf("range = %d–%d, asked for %d", read.FromPage, read.ToPage, target)
	}
	if len(read.Citations) == 0 {
		t.Fatal("a read must carry a citation back")
	}
}
