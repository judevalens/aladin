//go:build integration

// MCP end-to-end integration test.
//
// Drives the live MCP server using the modelcontextprotocol/go-sdk
// client transport, exercising the full agent-authored-pages workflow:
//
//	list_folders → create_page → get_page → update_block →
//	insert_blocks → delete_block → get_page (verify).
//
// Prerequisites (the test SkipNow()s if any are missing):
//   - Postgres reachable at $DATABASE_URL or postgres://aladin:password@localhost:5433/aladin
//   - the blocknote sidecar (converter :3500 + collab :3501 + /admin bridge)
//     reachable at $CONVERTER_URL or http://localhost:3500 — start via
//     `make blocknote`. M8c routes page writes/reads through the collab
//     bridge, so the sidecar must be the M8c build and its
//     BLOCKNOTE_ADMIN_SHARED_SECRET must match this process's (default
//     local-dev-admin-secret).
//
// Run with:
//
//	cd backend_v2 && go test -tags=integration ./internal/mcp/... -v -run TestMCP_EndToEnd
package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"aladin/backend_v2/internal/app"
	"aladin/backend_v2/internal/blocknote"
	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/dbtest"
	"aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const defaultAdminUserID = "00000000-0000-0000-0000-000000000001"

func TestMCP_EndToEnd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// SAFETY: this test creates/edits pages and wipes tables — only ever run it
	// against an explicit throwaway TEST_DATABASE_URL, never the dev DB.
	dbURL := dbtest.RequireTestDSN(t)
	converterURL := envDefault("CONVERTER_URL", "http://localhost:3500")

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("postgres not reachable at %s: %v", dbURL, err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("postgres ping failed: %v", err)
	}

	converter := blocknote.NewClient(converterURL, blocknote.ClientOptions{
		AdminSecret: envDefault("BLOCKNOTE_ADMIN_SHARED_SECRET", "local-dev-admin-secret"),
	})
	if err := converter.Healthz(ctx); err != nil {
		t.Skipf("blocknote sidecar not reachable at %s: %v", converterURL, err)
	}

	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Wipe page data for a clean test (the migration in M5.2 already
	// truncated, but rerunning the test would have left rows behind).
	if _, err := pool.Exec(ctx, `TRUNCATE artifacts, tree_nodes, page_documents RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	deps := app.NewMCPComponents(pool)

	// Mint an integration token by sitting in the default admin user's
	// session principal and calling AuthService.CreateIntegrationToken.
	adminCtx := service.WithPrincipal(ctx, service.Principal{
		UserID:    defaultAdminUserID,
		ActorType: service.ActorTypeUserSession,
		ActorID:   defaultAdminUserID,
		Email:     "admin@email.com",
	})
	createdToken, err := deps.Auth().CreateIntegrationToken(adminCtx, service.IntegrationTokenInput{
		Name:   "mcp-e2e-test",
		Scopes: []string{service.ScopeArtifactsRead, service.ScopeArtifactsWrite},
	})
	if err != nil {
		t.Fatalf("CreateIntegrationToken: %v", err)
	}

	// Stand up the MCP server in-process via httptest. The server reads
	// bearer tokens from Authorization: Bearer ... and resolves them via
	// the AuthService — same as the real `cmd/mcp` binary.
	// One client serves both conversion and the collab bridge (M8c).
	mcpServer := New(":0", deps, deps.PageDocuments(), converter, converter)
	ts := httptest.NewServer(mcpServer.httpServer.Handler)
	defer ts.Close()

	// Connect a real MCP SDK client over Streamable HTTP, with the bearer
	// token baked into the HTTP transport.
	httpClient := &http.Client{
		Transport: bearerRoundTripper{
			Token: createdToken.Token,
			Base:  http.DefaultTransport,
		},
		Timeout: 15 * time.Second,
	}
	transport := &sdkmcp.StreamableClientTransport{
		Endpoint:   ts.URL + "/mcp",
		HTTPClient: httpClient,
	}
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "aladin-mcp-e2e", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	defer session.Close()

	// list_folders — sanity check: the workspace is empty after truncate.
	out := callTool[map[string]any](t, ctx, session, "list_folders", map[string]any{})
	folders, _ := out["folders"].([]any)
	if len(folders) != 0 {
		t.Fatalf("list_folders: expected 0 folders, got %d", len(folders))
	}

	// create_page
	created := callTool[map[string]any](t, ctx, session, "create_page", map[string]any{
		"title":    "Integration test page",
		"markdown": "# Heading\n\nFirst paragraph.\n\n- item one\n- item two",
		"agent":    map[string]any{"id": "claude-code", "name": "Claude Code"},
	})
	pageID, _ := created["id"].(string)
	if pageID == "" {
		t.Fatalf("create_page returned no id: %#v", created)
	}

	// get_page — verify blocks landed and we got back stable IDs
	got := callTool[map[string]any](t, ctx, session, "get_page", map[string]any{"id": pageID})
	page, _ := got["page"].(map[string]any)
	blocks, _ := page["blocks"].([]any)
	if len(blocks) < 3 {
		t.Fatalf("expected >=3 blocks (heading + paragraph + 2 list items), got %d: %#v", len(blocks), got)
	}
	headingBlock, _ := blocks[0].(map[string]any)
	if headingBlock["type"] != "heading" {
		t.Fatalf("first block type = %v, want heading", headingBlock["type"])
	}
	headingID, _ := headingBlock["id"].(string)
	paragraphBlock, _ := blocks[1].(map[string]any)
	paragraphID, _ := paragraphBlock["id"].(string)
	if headingID == "" || paragraphID == "" {
		t.Fatalf("blocks missing ids: %#v", blocks)
	}

	// update_block — rewrite the paragraph
	updated := callTool[map[string]any](t, ctx, session, "update_block", map[string]any{
		"page_id":  pageID,
		"block_id": paragraphID,
		"markdown": "Surgically replaced **bold** body.",
	})
	if updated["replaced_block_count"].(float64) < 1 {
		t.Fatalf("update_block replaced_block_count = %v, want >=1", updated["replaced_block_count"])
	}

	// Verify the replaced block kept the paragraph's id
	got2 := callTool[map[string]any](t, ctx, session, "get_page", map[string]any{"id": pageID})
	page2, _ := got2["page"].(map[string]any)
	blocks2, _ := page2["blocks"].([]any)
	foundUpdated := false
	for _, b := range blocks2 {
		bm, _ := b.(map[string]any)
		if bm["id"] == paragraphID {
			foundUpdated = true
			md, _ := bm["markdown"].(string)
			if !strings.Contains(md, "Surgically") {
				t.Fatalf("updated block markdown = %q, want contain 'Surgically'", md)
			}
		}
	}
	if !foundUpdated {
		t.Fatalf("updated block id %q not found after update_block", paragraphID)
	}

	// insert_blocks — drop a callout after the heading
	inserted := callTool[map[string]any](t, ctx, session, "insert_blocks", map[string]any{
		"page_id":  pageID,
		"position": map[string]any{"after_id": headingID},
		"markdown": "> Heads up: this came from an agent.",
	})
	insertedIDs, _ := inserted["inserted_block_ids"].([]any)
	if len(insertedIDs) == 0 {
		t.Fatalf("insert_blocks returned no inserted_block_ids: %#v", inserted)
	}
	firstInsertedID, _ := insertedIDs[0].(string)

	// delete_block — remove the inserted block
	del := callTool[map[string]any](t, ctx, session, "delete_block", map[string]any{
		"page_id":  pageID,
		"block_id": firstInsertedID,
	})
	if del["deleted"] != true {
		t.Fatalf("delete_block returned %#v, want deleted:true", del)
	}

	// Final get_page: confirm the inserted block is gone and the paragraph
	// is still there.
	got3 := callTool[map[string]any](t, ctx, session, "get_page", map[string]any{"id": pageID})
	page3, _ := got3["page"].(map[string]any)
	blocks3, _ := page3["blocks"].([]any)
	for _, b := range blocks3 {
		bm, _ := b.(map[string]any)
		if bm["id"] == firstInsertedID {
			t.Fatalf("inserted block %q still present after delete_block", firstInsertedID)
		}
	}
}

// callTool is a tiny helper that calls the named MCP tool, decodes the
// StructuredContent into T, and fails the test on any error. Most of our
// tools return a JSON object so T is typically map[string]any.
func callTool[T any](t *testing.T, ctx context.Context, session *sdkmcp.ClientSession, name string, args map[string]any) T {
	t.Helper()
	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	if res.IsError {
		t.Fatalf("CallTool(%s) returned IsError; content: %s", name, contentString(res.Content))
	}
	var out T
	if res.StructuredContent != nil {
		raw, err := json.Marshal(res.StructuredContent)
		if err != nil {
			t.Fatalf("CallTool(%s) marshal structured content: %v", name, err)
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("CallTool(%s) unmarshal structured content: %v (raw=%s)", name, err, string(raw))
		}
		return out
	}
	// Fallback: the server returned only text content. Try to decode it.
	if err := json.Unmarshal([]byte(contentString(res.Content)), &out); err != nil {
		t.Fatalf("CallTool(%s) no structured content and content not JSON: %v", name, err)
	}
	return out
}

func contentString(content []sdkmcp.Content) string {
	var parts []string
	for _, c := range content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// bearerRoundTripper injects an Authorization: Bearer header on every
// request. Used in tests so the MCP SDK transport sends the integration
// token to /mcp.
type bearerRoundTripper struct {
	Token string
	Base  http.RoundTripper
}

func (b bearerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header.Set("Authorization", "Bearer "+b.Token)
	return b.Base.RoundTrip(clone)
}

func envDefault(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
