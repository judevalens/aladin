package mcpserver

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"aladin/backend_v2/internal/blocknote"
	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/dbtest"
	"aladin/backend_v2/internal/repo"
	"aladin/backend_v2/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestProjectPage_SyncsRefs proves the server-side projection: a page written via MCP (here,
// its live doc via the bridge) reconciles its `#` refs into artifact_refs — so agent-authored
// references become queryable without the frontend editor. Uses a fake bridge (no sidecar) +
// a real DB-backed ref service.
func TestProjectPage_SyncsRefs(t *testing.T) {
	dsn := dbtest.RequireTestDSN(t)
	ctx := context.Background()
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctxTO, dsn)
	if err != nil {
		t.Skipf("no test database: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctxTO); err != nil {
		t.Skipf("test database unreachable: %v", err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	userID := uuid.NewString()
	pageID := "proj-page-" + uuid.NewString()
	targetID := "proj-tgt-" + uuid.NewString()

	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, created_at) VALUES ($1::uuid,$2,now())`,
		userID, "u-"+uuid.NewString()[:8]+"@test.local"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO artifacts (id, user_id, type, title, content) VALUES
			($1,$3::uuid,'page','Holder',''),
			($2,$3::uuid,'page','Target note','')
	`, pageID, targetID, userID); err != nil {
		t.Fatalf("seed artifacts: %v", err)
	}
	// Every artifact is also a tree node (node id == artifact id); ReplaceRefs emits a node frame
	// for the page after syncing, which reads the light projection from tree_nodes.
	if _, err := pool.Exec(ctx, `
		INSERT INTO tree_nodes (id, user_id, kind, artifact_id, position, created_at, updated_at)
		VALUES ($1, $2::uuid, 'artifact', $1, 0, now(), now())
	`, pageID, userID); err != nil {
		t.Fatalf("seed tree node: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM outbox_events WHERE user_id = $1::uuid`, userID)
		_, _ = pool.Exec(bg, `DELETE FROM artifact_refs WHERE artifact_id = $1`, pageID)
		_, _ = pool.Exec(bg, `DELETE FROM tree_nodes WHERE id = $1`, pageID)
		_, _ = pool.Exec(bg, `DELETE FROM artifacts WHERE id IN ($1,$2)`, pageID, targetID)
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE id = $1::uuid`, userID)
	})

	// The live doc (as the bridge would return it) references the target page via a `#` ref.
	blocks := json.RawMessage(`[{"id":"b1","type":"paragraph","content":[
		{"type":"text","text":"see "},
		{"type":"artifactRef","props":{"kind":"page","targetId":"` + targetID + `","label":"Target note"}}
	]}]`)
	bridge := &fakeBridge{getFunc: func(string) (blocknote.BridgePage, error) {
		return blocknote.BridgePage{Blocks: blocks}, nil
	}}

	refSvc := service.NewArtifactRefService(repo.NewArtifactRefPostgres(pool))
	tools := toolServer{bridge: bridge, artifactRefs: refSvc}

	// projectPage → SyncRefs → ReplaceRefs needs the owning principal (user scope + LockUser), just
	// as the real MCP call has one; inject it (the read-path ListForArtifact needs none).
	authed := service.WithPrincipal(ctx, service.Principal{UserID: userID})
	tools.projectPage(authed, pageID)

	got, err := refSvc.ListForArtifact(ctx, pageID)
	if err != nil {
		t.Fatalf("list refs: %v", err)
	}
	if len(got) != 1 || got[0].Kind != service.RefKindPage || got[0].TargetID != targetID {
		t.Fatalf("expected one page ref to %s, got %+v", targetID, got)
	}
	if got[0].Label != "Target note" {
		t.Fatalf("ref label should resolve from the target artifact, got %q", got[0].Label)
	}

	// Idempotent: projecting the same doc again reconciles to the same single row.
	tools.projectPage(authed, pageID)
	got, _ = refSvc.ListForArtifact(ctx, pageID)
	if len(got) != 1 {
		t.Fatalf("re-projection should stay at 1 ref, got %+v", got)
	}
}
