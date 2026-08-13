package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// The grant is the whole point of the workspace plane: a shard reads only what
// its manifest declares, checked server-side on every call. These tests pin that
// (and the registry's ref-encoding rules) without a database.

type stubEntityService struct {
	kind       string
	observable bool
	entities   map[string]string // id -> title
	err        error
}

func (s stubEntityService) Kind() string     { return s.kind }
func (s stubEntityService) Observable() bool { return s.observable }

func (s stubEntityService) Exists(_ context.Context, id string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	_, ok := s.entities[id]
	return ok, nil
}

func (s stubEntityService) NodeView(_ context.Context, id string) (NodeView, error) {
	if s.err != nil {
		return NodeView{}, s.err
	}
	title, ok := s.entities[id]
	if !ok {
		return NodeView{}, ErrNotFound
	}
	data, _ := json.Marshal(map[string]string{"title": title})
	return NodeView{ID: id, Kind: s.kind, Title: title, Data: data}, nil
}

// stubShardStore serves one manifest; ReadFile of anything else misses.
type stubShardStore struct {
	DocSurfaceStore
	manifest string
	readErr  error
}

func (s stubShardStore) ReadFile(_ context.Context, _ string, path string) ([]byte, error) {
	if s.readErr != nil {
		return nil, s.readErr
	}
	if path != shardManifestFile {
		return nil, ErrNotFound
	}
	return []byte(s.manifest), nil
}

// bridgeStubArtifacts answers Get for one shard id.
type bridgeStubArtifacts struct {
	ArtifactService
	id      string
	typ     string
	missing bool
}

func (s bridgeStubArtifacts) Get(_ context.Context, id string) (ArtifactResponse, error) {
	if s.missing || id != s.id {
		return ArtifactResponse{}, ErrNotFound
	}
	return ArtifactResponse{ID: id, Type: s.typ, Title: "shard"}, nil
}

const grantManifest = `{
  "version": 1,
  "anchors": [
    {"id": "positions", "route": "#/", "meaning": "holdings", "refs": ["artifact-a", "watchlist:w-1"]},
    {"id": "notes", "route": "#/notes", "meaning": "notes", "refs": ["record-r1"]}
  ]
}`

func testBridge(t *testing.T, manifest string) ShardBridgeService {
	t.Helper()
	registry := NewEntityRegistry(
		stubEntityService{kind: "artifact", observable: true, entities: map[string]string{"artifact-a": "Memo"}},
		stubEntityService{kind: "record", entities: map[string]string{"record-r1": "Clipping"}},
		stubEntityService{kind: "watchlist", observable: true, entities: map[string]string{"w-1": "Semis"}},
	)
	return NewShardBridgeService(
		bridgeStubArtifacts{id: "artifact-shard", typ: "app"},
		stubShardStore{manifest: manifest},
		registry,
	)
}

func TestShardBridge_GrantAllowsDeclaredRefsOnly(t *testing.T) {
	ctx := context.Background()
	bridge := testBridge(t, grantManifest)

	nodes, missing, err := bridge.GetNodes(ctx, "artifact-shard", []string{"artifact-a", "watchlist:w-1"})
	if err != nil {
		t.Fatalf("GetNodes(declared): %v", err)
	}
	if len(nodes) != 2 || len(missing) != 0 {
		t.Fatalf("nodes=%d missing=%v, want 2/none", len(nodes), missing)
	}
	// Views come back keyed by the REF the shard asked with (so a qualified ref
	// like watchlist:w-1 lines up on the shard side).
	if nodes[1].ID != "watchlist:w-1" || nodes[1].Title != "Semis" {
		t.Fatalf("qualified ref view = %+v", nodes[1])
	}

	// An undeclared id fails the WHOLE call — a shard reaching outside its
	// manifest is a bug to surface, not a subset to quietly serve.
	_, _, err = bridge.GetNodes(ctx, "artifact-shard", []string{"artifact-a", "artifact-secret"})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("undeclared ref err = %v, want ErrForbidden", err)
	}
	if !strings.Contains(err.Error(), "artifact-secret") {
		t.Fatalf("denial should name the offending id: %v", err)
	}
}

func TestShardBridge_NoManifestMeansEmptyGrant(t *testing.T) {
	ctx := context.Background()
	bridge := NewShardBridgeService(
		bridgeStubArtifacts{id: "artifact-shard", typ: "app"},
		stubShardStore{readErr: ErrNotFound},
		NewEntityRegistry(stubEntityService{kind: "artifact", entities: map[string]string{"artifact-a": "Memo"}}),
	)
	if _, _, err := bridge.GetNodes(ctx, "artifact-shard", []string{"artifact-a"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("no manifest should deny everything, got %v", err)
	}
}

func TestShardBridge_DeclaredButUnresolvedIsMissingNotError(t *testing.T) {
	ctx := context.Background()
	bridge := testBridge(t, `{"version":1,"anchors":[{"id":"a","route":"#/","meaning":"m","refs":["artifact-gone"]}]}`)
	nodes, missing, err := bridge.GetNodes(ctx, "artifact-shard", []string{"artifact-gone"})
	if err != nil {
		t.Fatalf("GetNodes: %v", err)
	}
	if len(nodes) != 0 || len(missing) != 1 || missing[0] != "artifact-gone" {
		t.Fatalf("nodes=%v missing=%v, want the ref reported missing", nodes, missing)
	}
}

func TestShardBridge_CheckRefsClassifiesEveryRef(t *testing.T) {
	ctx := context.Background()
	bridge := testBridge(t, `{
      "version": 1,
      "anchors": [{"id":"a","route":"#/","meaning":"m",
        "refs":["artifact-a","record-r1","artifact-gone","sparkle-9"]}]
    }`)
	report, err := bridge.CheckRefs(ctx, "artifact-shard")
	if err != nil {
		t.Fatalf("CheckRefs: %v", err)
	}
	if report.OK {
		t.Fatalf("report should not be OK: %+v", report)
	}
	if report.Total != 4 {
		t.Errorf("Total = %d, want 4", report.Total)
	}
	if len(report.Missing) != 1 || report.Missing[0] != "artifact-gone" {
		t.Errorf("Missing = %v", report.Missing)
	}
	if len(report.UnknownKind) != 1 || report.UnknownKind[0] != "sparkle-9" {
		t.Errorf("UnknownKind = %v", report.UnknownKind)
	}
	// record exists but its kind emits no frames: renderable, never live.
	if len(report.Unobservable) != 1 || report.Unobservable[0] != "record-r1" {
		t.Errorf("Unobservable = %v", report.Unobservable)
	}
}

func TestShardBridge_NonAppArtifactIsNotFound(t *testing.T) {
	ctx := context.Background()
	bridge := NewShardBridgeService(
		bridgeStubArtifacts{id: "artifact-page", typ: "page"},
		stubShardStore{manifest: grantManifest},
		NewEntityRegistry(),
	)
	if _, _, err := bridge.GetNodes(ctx, "artifact-page", nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("non-app artifact err = %v, want ErrNotFound", err)
	}
}

func TestEntityRegistry_ResolveEncodings(t *testing.T) {
	registry := NewEntityRegistry(
		stubEntityService{kind: "artifact"},
		stubEntityService{kind: "research"},
		stubEntityService{kind: "watchlist"},
	)
	cases := []struct {
		ref      string
		wantKind string
		wantID   string
	}{
		{"artifact-123", "artifact", "artifact-123"}, // prefix keeps the whole id
		{"research-9", "research", "research-9"},
		{"watchlist:uuid-1", "watchlist", "uuid-1"}, // qualifier is stripped
	}
	for _, tc := range cases {
		kind, id, _, err := registry.Resolve(tc.ref)
		if err != nil || kind != tc.wantKind || id != tc.wantID {
			t.Errorf("Resolve(%q) = (%q,%q,%v), want (%q,%q,nil)", tc.ref, kind, id, err, tc.wantKind, tc.wantID)
		}
	}
	for _, bad := range []string{"", "nope-1", "unknownkind:1", "bare-uuid-without-prefix-or-kind"} {
		if _, _, _, err := registry.Resolve(bad); err == nil {
			t.Errorf("Resolve(%q) should fail", bad)
		}
	}
}
