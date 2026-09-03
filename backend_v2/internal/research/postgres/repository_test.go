package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/dbtest"
	"aladin/backend_v2/internal/file"
	"aladin/backend_v2/internal/repo"
	"aladin/backend_v2/internal/research"
	researchpostgres "aladin/backend_v2/internal/research/postgres"
	artifactservice "aladin/backend_v2/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func mustTestPool(ctx context.Context, t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := dbtest.RequireTestDSN(t)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("no test database: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("test database unreachable: %v", err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		pool.Close()
		t.Fatalf("migrate: %v", err)
	}
	return pool
}

func adminContext(userID string) context.Context {
	return artifactservice.WithPrincipal(context.Background(), artifactservice.Principal{
		UserID: userID, ActorType: artifactservice.ActorTypeUserSession, ActorID: userID,
		Scopes: []string{artifactservice.ScopeArtifactsRead, artifactservice.ScopeArtifactsWrite},
	})
}

// TestResearchFolder_CreateEmitsFrameWithExtension is the load-bearing test for the
// research bench's spine (RESEARCH_SURFACE_PRD §5 + §11): creating a research folder must
// write the tree node AND its 1:1 extension row AND the sync frame in one transaction,
// and the frame must carry the extension's light fields so a tree row can render run
// state without a second fetch.
func TestResearchFolder_CreateEmitsFrameWithExtension(t *testing.T) {
	ctx := context.Background()
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	pool := mustTestPool(ctxTO, t)
	defer pool.Close()

	tag := uuid.NewString()[:8]
	userID := uuid.NewString()
	nodeID := "research-" + uuid.NewString()
	ctx = adminContext(userID)

	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, created_at) VALUES ($1::uuid, $2, now())`,
		userID, "rs-"+tag+"@test.local"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM tree_nodes WHERE user_id = $1::uuid`, userID)
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE id = $1::uuid`, userID)
	})

	researchRepo := researchpostgres.NewResearchPostgres(pool)
	svc := research.NewResearchService(researchRepo)

	node, err := svc.Create(ctx, research.ResearchCreateInput{
		ID:         nodeID,
		Title:      "PEAD semis " + tag,
		Hypothesis: "post-earnings drift persists in semis",
	})
	if err != nil {
		t.Fatalf("create research folder: %v", err)
	}
	if node.Kind != "research" || node.Seq == 0 {
		t.Fatalf("light node = %+v, want kind=research and a non-zero seq", node)
	}

	// The extension row exists and carries the schema defaults (§8: event is the
	// primitive; §5: state from creation).
	got, err := svc.Get(ctx, nodeID)
	if err != nil {
		t.Fatalf("get research folder: %v", err)
	}
	if got.ExecMode != "event" || got.RunState != "idle" || got.SourceKind != "authored" {
		t.Fatalf("extension defaults = %+v, want event/idle/authored", got)
	}
	if got.Hypothesis != "post-earnings drift persists in semis" {
		t.Fatalf("hypothesis = %q", got.Hypothesis)
	}
	// Heavy fields stay empty on a sparse row — no code exists yet (§5).
	if got.CodeHash != "" || got.CommitSHA != "" {
		t.Fatalf("sparse row should have no code hash / sha, got %+v", got)
	}

	// The outbox frame committed in the same tx, and carries the extension's light
	// fields. This is what lets the tree render run state with no second fetch.
	var payload []byte
	if err := pool.QueryRow(ctx, `
		SELECT payload FROM outbox_events
		 WHERE user_id = $1::uuid AND type = 'data_event'
		 ORDER BY xid DESC, created_at DESC LIMIT 1
	`, userID).Scan(&payload); err != nil {
		t.Fatalf("read outbox frame: %v", err)
	}
	var frame struct {
		Entities []struct {
			EntityKind string `json:"entityKind"`
			EntityID   string `json:"entityId"`
			Op         string `json:"op"`
			Data       struct {
				Kind     string `json:"kind"`
				Title    string `json:"title"`
				Research *struct {
					RunState   string `json:"runState"`
					ExecMode   string `json:"execMode"`
					SourceKind string `json:"sourceKind"`
				} `json:"research"`
			} `json:"data"`
		} `json:"entities"`
	}
	if err := json.Unmarshal(payload, &frame); err != nil {
		t.Fatalf("decode frame %s: %v", payload, err)
	}
	if len(frame.Entities) != 1 {
		t.Fatalf("expected a single-entity frame, got %d: %s", len(frame.Entities), payload)
	}
	ent := frame.Entities[0]
	if ent.EntityKind != "research" || ent.EntityID != nodeID || ent.Op != "upsert" {
		t.Fatalf("frame entity = %+v", ent)
	}
	if ent.Data.Research == nil {
		t.Fatalf("research frame must carry the extension payload: %s", payload)
	}
	if ent.Data.Research.RunState != "idle" || ent.Data.Research.ExecMode != "event" {
		t.Fatalf("frame research payload = %+v", ent.Data.Research)
	}
	// The heavy fields must NOT be relayed — that's what keeps a manifest edit from
	// re-broadcasting to every tree row.
	if body := string(payload); strings.Contains(body, "post-earnings drift") {
		t.Fatalf("hypothesis must not ride the tree frame: %s", body)
	}
}

// TestResearchFolder_RejectsResearchParent covers §5's "comparing two strategies is a
// view ABOVE research folders, never a folder containing them".
func TestResearchFolder_RejectsResearchParent(t *testing.T) {
	ctx := context.Background()
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	pool := mustTestPool(ctxTO, t)
	defer pool.Close()

	tag := uuid.NewString()[:8]
	userID := uuid.NewString()
	parentID := "research-" + uuid.NewString()
	ctx = adminContext(userID)

	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, created_at) VALUES ($1::uuid, $2, now())`,
		userID, "rp-"+tag+"@test.local"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM tree_nodes WHERE user_id = $1::uuid`, userID)
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE id = $1::uuid`, userID)
	})

	svc := research.NewResearchService(researchpostgres.NewResearchPostgres(pool))
	if _, err := svc.Create(ctx, research.ResearchCreateInput{ID: parentID, Title: "Parent " + tag}); err != nil {
		t.Fatalf("create parent: %v", err)
	}

	_, err := svc.Create(ctx, research.ResearchCreateInput{Title: "Nested " + tag, ParentID: &parentID})
	if err == nil {
		t.Fatal("nesting research inside research must be rejected")
	}
	var badReq artifactservice.BadRequest
	if !errors.As(err, &badReq) {
		t.Fatalf("want a BadRequest, got %T: %v", err, err)
	}
}

// TestResearchFolder_NestingRules pins the containment rules (RESEARCH_SURFACE_PRD §5 +
// §21):
//
//	research inside a folder    → allowed
//	folder   inside a research  → allowed
//	artifact inside a research  → allowed  (§21: research artifacts live in the folder)
//	research inside a research  → REJECTED (§5: comparison is a view ABOVE research)
//
// The first three all flow through container validation, which was folder-only before
// the research kind existed — so a research folder could hold nothing at all.
func TestResearchFolder_NestingRules(t *testing.T) {
	ctx := context.Background()
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	pool := mustTestPool(ctxTO, t)
	defer pool.Close()

	tag := uuid.NewString()[:8]
	userID := uuid.NewString()
	folderID := "folder-" + uuid.NewString()
	researchID := "research-" + uuid.NewString()
	ctx = adminContext(userID)

	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, created_at) VALUES ($1::uuid, $2, now())`,
		userID, "rn-"+tag+"@test.local"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM tree_nodes WHERE user_id = $1::uuid`, userID)
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE id = $1::uuid`, userID)
	})

	artifacts := repo.NewArtifactsPostgres(pool)
	researchService := research.NewResearchService(researchpostgres.NewResearchPostgres(pool))

	// A plain folder to nest into.
	if err := artifacts.CreateTreeNode(ctx, artifactservice.TreeNodeRecord{
		ID: folderID, Kind: "folder", Title: strPtr("Ideas " + tag), Position: 1,
	}); err != nil {
		t.Fatalf("seed folder: %v", err)
	}

	// research inside folder → allowed.
	if _, err := researchService.Create(ctx, research.ResearchCreateInput{
		ID: researchID, Title: "PEAD " + tag, ParentID: &folderID,
	}); err != nil {
		t.Fatalf("research inside a folder must be allowed: %v", err)
	}

	// folder inside research → allowed (container validation, not folder-only).
	if _, err := artifacts.GetContainer(ctx, researchID); err != nil {
		t.Fatalf("a research folder must resolve as a container: %v", err)
	}

	// research inside research → rejected.
	_, err := researchService.Create(ctx, research.ResearchCreateInput{
		Title: "Nested " + tag, ParentID: &researchID,
	})
	if err == nil {
		t.Fatal("research nested inside research must be rejected")
	}
	var badReq artifactservice.BadRequest
	if !errors.As(err, &badReq) {
		t.Fatalf("want BadRequest, got %T: %v", err, err)
	}

	// And a plain folder must still NOT resolve through the research parent check,
	// which is what keeps the rule one-directional.
	ok, err := researchpostgres.NewResearchPostgres(pool).ParentIsFolder(ctx, researchID)
	if err != nil {
		t.Fatalf("ParentIsFolder: %v", err)
	}
	if ok {
		t.Fatal("ParentIsFolder must not accept a research node")
	}
	if ok, err = researchpostgres.NewResearchPostgres(pool).ParentIsFolder(ctx, folderID); err != nil || !ok {
		t.Fatalf("ParentIsFolder(folder) = %v, %v; want true, nil", ok, err)
	}
}

func strPtr(s string) *string { return &s }

// TestResearchFolder_HoldsFoldersAndArtifacts drives the REAL create paths the browser
// pane uses — CreateBrowserNode for a folder and for a note — with a research folder as
// the parent. §21: research artifacts live in the research folder via the existing
// capture surface, so both must land. Testing GetContainer alone was not enough: this is
// the path the UI actually takes.
func TestResearchFolder_HoldsFoldersAndArtifacts(t *testing.T) {
	ctx := context.Background()
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	pool := mustTestPool(ctxTO, t)
	defer pool.Close()

	tag := uuid.NewString()[:8]
	userID := uuid.NewString()
	researchID := "research-" + uuid.NewString()
	ctx = adminContext(userID)

	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, created_at) VALUES ($1::uuid, $2, now())`,
		userID, "rh-"+tag+"@test.local"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM tree_nodes WHERE user_id = $1::uuid`, userID)
		_, _ = pool.Exec(bg, `DELETE FROM artifacts WHERE user_id = $1::uuid`, userID)
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE id = $1::uuid`, userID)
	})

	researchService := research.NewResearchService(researchpostgres.NewResearchPostgres(pool))
	if _, err := researchService.Create(ctx, research.ResearchCreateInput{
		ID: researchID, Title: "PEAD " + tag,
	}); err != nil {
		t.Fatalf("create research: %v", err)
	}

	artifactSvc := artifactservice.NewArtifactService(
		repo.NewArtifactsPostgres(pool),
		file.NewFilesystemArtifactStore(t.TempDir(), t.TempDir()),
	)

	// "New folder here" inside a research folder.
	folderRes, err := artifactSvc.CreateBrowserNode(ctx, artifactservice.BrowserNodeCreateInput{
		Kind: "folder", Title: "Data " + tag, ParentID: &researchID,
	})
	if err != nil {
		t.Fatalf("create folder inside research: %v", err)
	}
	if folderRes.Node.ParentID == nil || *folderRes.Node.ParentID != researchID {
		t.Fatalf("folder parent = %+v, want %s", folderRes.Node.ParentID, researchID)
	}

	// "New note here" inside a research folder.
	noteRes, err := artifactSvc.CreateBrowserNode(ctx, artifactservice.BrowserNodeCreateInput{
		Kind:     "artifact",
		Title:    "Notes " + tag,
		ParentID: &researchID,
		Artifact: &artifactservice.BrowserArtifactPayload{Type: "page"},
	})
	if err != nil {
		t.Fatalf("create note inside research: %v", err)
	}
	if noteRes.Node.ParentID == nil || *noteRes.Node.ParentID != researchID {
		t.Fatalf("note parent = %+v, want %s", noteRes.Node.ParentID, researchID)
	}

	// Both are children of the research node in the canonical tree.
	var children int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM tree_nodes WHERE parent_id = $1 AND user_id = $2::uuid AND is_deleted = false`,
		researchID, userID).Scan(&children); err != nil {
		t.Fatalf("count children: %v", err)
	}
	if children != 2 {
		t.Fatalf("research folder children = %d, want 2", children)
	}
}

// TestResearchFolder_Rename covers the rename endpoint: it retitles, emits the frame, and
// is scoped to kind='research' so it can't retitle a plain folder (the mirror of the
// folder API being scoped to kind='folder').
func TestResearchFolder_Update(t *testing.T) {
	ctx := context.Background()
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	pool := mustTestPool(ctxTO, t)
	defer pool.Close()

	tag := uuid.NewString()[:8]
	userID := uuid.NewString()
	researchID := "research-" + uuid.NewString()
	folderID := "folder-" + uuid.NewString()
	ctx = adminContext(userID)

	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, created_at) VALUES ($1::uuid, $2, now())`,
		userID, "rr-"+tag+"@test.local"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM tree_nodes WHERE user_id = $1::uuid`, userID)
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE id = $1::uuid`, userID)
	})

	svc := research.NewResearchService(researchpostgres.NewResearchPostgres(pool))
	if _, err := svc.Create(ctx, research.ResearchCreateInput{ID: researchID, Title: "Before " + tag}); err != nil {
		t.Fatalf("create: %v", err)
	}

	node, err := svc.Update(ctx, researchID, research.ResearchPatch{Title: strPtr("  After " + tag + "  ")})
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if node.Title != "After "+tag {
		t.Fatalf("title = %q, want trimmed %q", node.Title, "After "+tag)
	}
	if node.Kind != "research" || node.Seq == 0 {
		t.Fatalf("renamed node = %+v", node)
	}

	got, err := svc.Get(ctx, researchID)
	if err != nil || got.Title != "After "+tag {
		t.Fatalf("read back = %+v, err %v", got, err)
	}

	// The rename emitted a frame, so the tree updates through the syncer.
	var payload []byte
	if err := pool.QueryRow(ctx, `
		SELECT payload FROM outbox_events
		 WHERE user_id = $1::uuid AND type = 'data_event'
		 ORDER BY xid DESC, created_at DESC LIMIT 1
	`, userID).Scan(&payload); err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	if !strings.Contains(string(payload), "After "+tag) {
		t.Fatalf("rename frame missing the new title: %s", payload)
	}

	// An empty title is rejected; an empty patch is too.
	if _, err := svc.Update(ctx, researchID, research.ResearchPatch{Title: strPtr("   ")}); err == nil {
		t.Fatal("empty title must be rejected")
	}
	if _, err := svc.Update(ctx, researchID, research.ResearchPatch{}); err == nil {
		t.Fatal("an empty patch must be rejected")
	}

	// A hypothesis edit must NOT blank the title (the COALESCE guard).
	if _, err := svc.Update(ctx, researchID, research.ResearchPatch{Hypothesis: strPtr("drift persists")}); err != nil {
		t.Fatalf("patch hypothesis: %v", err)
	}
	after, err := svc.Get(ctx, researchID)
	if err != nil {
		t.Fatalf("get after hypothesis patch: %v", err)
	}
	if after.Title != "After "+tag {
		t.Fatalf("hypothesis patch clobbered the title: %+v", after)
	}
	if after.Hypothesis != "drift persists" {
		t.Fatalf("hypothesis = %q", after.Hypothesis)
	}

	// A plain folder must NOT be renameable through the research endpoint.
	if err := repo.NewArtifactsPostgres(pool).CreateTreeNode(ctx, artifactservice.TreeNodeRecord{
		ID: folderID, Kind: "folder", Title: strPtr("Plain " + tag), Position: 2,
	}); err != nil {
		t.Fatalf("seed folder: %v", err)
	}
	if _, err := svc.Update(ctx, folderID, research.ResearchPatch{Title: strPtr("hijacked")}); err == nil {
		t.Fatal("the research endpoint must not rename a plain folder")
	}

}
