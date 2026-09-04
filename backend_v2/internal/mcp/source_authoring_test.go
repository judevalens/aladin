package mcpserver

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"aladin/backend_v2/internal/artifact"
	"aladin/backend_v2/internal/docsurface"
	"aladin/backend_v2/internal/service"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func sourceInt(v int) *int { return &v }

func TestReadFileRanges(t *testing.T) {
	for _, tc := range []struct {
		name, content   string
		start, end      *int
		want            string
		total, from, to int
		bad             bool
	}{
		{name: "legacy whole file", content: "one\r\ntwo\nthree", want: "one\r\ntwo\nthree", total: 3, from: 1, to: 3},
		{name: "single CRLF line", content: "one\r\ntwo\r\n", start: sourceInt(2), end: sourceInt(2), want: "two\r\n", total: 2, from: 2, to: 2},
		{name: "open ended", content: "one\ntwo\nthree", start: sourceInt(2), want: "two\nthree", total: 3, from: 2, to: 3},
		{name: "end only", content: "one\ntwo\nthree", end: sourceInt(2), want: "one\ntwo\n", total: 3, from: 1, to: 2},
		{name: "clamped end", content: "one\ntwo", start: sourceInt(2), end: sourceInt(999), want: "two", total: 2, from: 2, to: 2},
		{name: "empty", content: "", want: ""},
		{name: "blank lines", content: "\n\n", start: sourceInt(2), end: sourceInt(2), want: "\n", total: 2, from: 2, to: 2},
		{name: "unicode", content: "α\n日本語\n", start: sourceInt(2), want: "日本語\n", total: 2, from: 2, to: 2},
		{name: "one line", content: "one", start: sourceInt(1), want: "one", total: 1, from: 1, to: 1},
		{name: "zero", content: "one", start: sourceInt(0), bad: true},
		{name: "negative", content: "one", end: sourceInt(-1), bad: true},
		{name: "reverse", content: "one\ntwo", start: sourceInt(2), end: sourceInt(1), bad: true},
		{name: "beyond EOF", content: "one\n", start: sourceInt(2), bad: true},
		{name: "range in empty file", start: sourceInt(1), bad: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts := docToolServer{artifacts: fakeArtifacts{}, store: fakeStore{files: map[string]string{"a.ts": tc.content}}}
			_, got, err := ts.readFile(context.Background(), nil, readFileInput{PageID: "p1", Path: "a.ts", StartLine: tc.start, EndLine: tc.end})
			if tc.bad {
				if err == nil {
					t.Fatal("expected invalid range")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Content != tc.want || got.TotalLines != tc.total || got.StartLine != tc.from || got.EndLine != tc.to {
				t.Fatalf("got %+v; want content=%q total=%d range=%d:%d", got, tc.want, tc.total, tc.from, tc.to)
			}
			if want := fmt.Sprintf("%x", sha256.Sum256([]byte(tc.content))); got.Hash != want {
				t.Fatalf("hash %s != %s", got.Hash, want)
			}
		})
	}
}

func TestSourceAuthoringFindReadEdit(t *testing.T) {
	ctx := service.WithPrincipal(context.Background(), service.Principal{UserID: "author"})
	store := docsurface.NewStore(t.TempDir())
	const original = "const title = 'before';\nconst stable = true;\n"
	if err := store.WriteFile(ctx, "p1", "index.tsx", []byte(original)); err != nil {
		t.Fatal(err)
	}
	builder := &fakeBuild{}
	ts := docToolServer{artifacts: fakeArtifacts{}, store: store, build: builder}
	_, found, err := ts.grepFiles(ctx, nil, grepFilesInput{PageID: "p1", Pattern: "title", Glob: "**/*.tsx", ContextLines: 1})
	if err != nil || len(found.Matches) != 1 || found.Matches[0].Line != 1 || len(found.Matches[0].After) != 1 {
		t.Fatalf("search: %+v %v", found, err)
	}
	_, read, err := ts.readFile(ctx, nil, readFileInput{PageID: "p1", Path: found.Matches[0].Path, StartLine: sourceInt(1), EndLine: sourceInt(1)})
	if err != nil {
		t.Fatal(err)
	}
	no := false
	_, edited, err := ts.editFile(ctx, nil, editFileInput{PageID: "p1", Path: "index.tsx", OldString: read.Content, NewString: "const title = 'after';\n", ExpectedHash: &read.Hash, Build: &no})
	if err != nil || !edited.OK || edited.Replacements != 1 || edited.Hash == read.Hash || edited.Build != nil || len(builder.built) != 0 {
		t.Fatalf("edit: %+v %v", edited, err)
	}
	data, _ := store.ReadFile(ctx, "p1", "index.tsx")
	if string(data) != strings.Replace(original, "before", "after", 1) {
		t.Fatalf("unexpected file %q", data)
	}
	entries, err := store.ListDir(ctx, "p1", ".history")
	if err != nil || len(entries) != 1 {
		t.Fatalf("history: %+v %v", entries, err)
	}
	history, _ := store.ReadFile(ctx, "p1", ".history/"+entries[0].Name)
	if string(history) != original {
		t.Fatalf("history=%q", history)
	}

	// The old text still exists, but a change elsewhere makes this hash stale.
	_, _, err = ts.editFile(ctx, nil, editFileInput{PageID: "p1", Path: "index.tsx", OldString: "stable", NewString: "changed", ExpectedHash: &read.Hash})
	if !errors.Is(err, service.ErrConflict) {
		t.Fatalf("want conflict, got %v", err)
	}
	after, _ := store.ReadFile(ctx, "p1", "index.tsx")
	historyAfter, _ := store.ListDir(ctx, "p1", ".history")
	if string(after) != string(data) || len(historyAfter) != 1 || len(builder.built) != 0 {
		t.Fatal("stale edit mutated, snapshotted or built")
	}

	// Existing callers can still omit the hash and retain automatic draft builds.
	_, edited, err = ts.editFile(ctx, nil, editFileInput{PageID: "p1", Path: "index.tsx", OldString: "stable", NewString: "stableName"})
	if err != nil || edited.Build == nil || !edited.Build.OK || len(builder.built) != 1 || builder.built[0] != service.ChannelDraft {
		t.Fatalf("legacy edit: %+v %v", edited, err)
	}
}

type deniedSourceArtifacts struct{ artifact.ArtifactService }

func (deniedSourceArtifacts) Get(context.Context, string) (artifact.ArtifactResponse, error) {
	return artifact.ArtifactResponse{}, service.ErrNotFound
}

func TestSourceToolsAuthorizeBeforeIO(t *testing.T) {
	// A nil store would panic if any IO happened before artifact authorization.
	ts := docToolServer{artifacts: deniedSourceArtifacts{}}
	ctx := context.Background()
	if _, _, err := ts.readFile(ctx, nil, readFileInput{PageID: "private", Path: "index.tsx"}); !errors.Is(err, service.ErrNotFound) {
		t.Fatal(err)
	}
	if _, _, err := ts.grepFiles(ctx, nil, grepFilesInput{PageID: "private", Pattern: "secret"}); !errors.Is(err, service.ErrNotFound) {
		t.Fatal(err)
	}
	if _, _, err := ts.editFile(ctx, nil, editFileInput{PageID: "private", Path: "index.tsx", OldString: "a", NewString: "b"}); !errors.Is(err, service.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestEditFileGuardsDoNotMutate(t *testing.T) {
	for _, tc := range []struct {
		name, old, next string
		all             bool
		count           int
		bad             bool
	}{
		{"absent", "absent", "next", false, 0, true},
		{"ambiguous", "same", "next", false, 0, true},
		{"empty", "", "next", false, 0, true},
		{"identical", "same", "same", false, 0, true},
		{"replace all", "same", "next", true, 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &writableStore{files: map[string]string{"a.ts": "same\nsame\n"}}
			builder := &fakeBuild{}
			ts := docToolServer{artifacts: fakeArtifacts{}, store: store, build: builder}
			_, out, err := ts.editFile(context.Background(), nil, editFileInput{PageID: "p1", Path: "a.ts", OldString: tc.old, NewString: tc.next, ReplaceAll: tc.all})
			if tc.bad {
				if err == nil || store.files["a.ts"] != "same\nsame\n" || len(builder.built) != 0 || len(historySnapshots(store)) != 0 {
					t.Fatalf("guard failed: %+v %v", out, err)
				}
			} else if err != nil || out.Replacements != tc.count {
				t.Fatalf("edit failed: %+v %v", out, err)
			}
		})
	}
}

func TestHashGuardFailsClosedWithoutAtomicStore(t *testing.T) {
	store := &writableStore{files: map[string]string{"a.ts": "before"}}
	ts := docToolServer{artifacts: fakeArtifacts{}, store: store}
	_, read, err := ts.readFile(context.Background(), nil, readFileInput{PageID: "p1", Path: "a.ts"})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = ts.editFile(context.Background(), nil, editFileInput{PageID: "p1", Path: "a.ts", OldString: "before", NewString: "after", ExpectedHash: &read.Hash})
	if err == nil || !strings.Contains(err.Error(), "unavailable") || store.files["a.ts"] != "before" || len(historySnapshots(store)) != 0 {
		t.Fatalf("unsupported store: %v", err)
	}
}

func TestSourceToolsOverMCP(t *testing.T) {
	ctx := service.WithPrincipal(context.Background(), service.Principal{UserID: "author"})
	store := docsurface.NewStore(t.TempDir())
	if err := store.WriteFile(ctx, "p1", "a.ts", []byte("first\nneedle\nlast\n")); err != nil {
		t.Fatal(err)
	}
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "source-tools", Version: "test"}, nil)
	registerDocSurfaceTools(server, fakeArtifacts{}, store, nil, nil, nil, nil)
	ct, st := sdkmcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "source-client", Version: "test"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	call := func(name string, args map[string]any, dest any) {
		t.Helper()
		result, err := cs.CallTool(ctx, &sdkmcp.CallToolParams{Name: name, Arguments: args})
		if err != nil || result.IsError {
			t.Fatalf("%s: %+v %v", name, result, err)
		}
		data, err := json.Marshal(result.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, dest); err != nil {
			t.Fatal(err)
		}
	}
	var found service.SourceSearchResult
	call("grep_files", map[string]any{"page_id": "p1", "pattern": "NEEDLE", "case_sensitive": false, "context_lines": 1, "glob": "**/*.ts"}, &found)
	if len(found.Matches) != 1 || found.Matches[0].Line != 2 {
		t.Fatalf("search %+v", found)
	}
	var read readFileOutput
	call("read_file", map[string]any{"page_id": "p1", "path": "a.ts", "start_line": 2, "end_line": 2}, &read)
	if read.Content != "needle\n" || read.TotalLines != 3 || read.StartLine != 2 || read.EndLine != 2 {
		t.Fatalf("read %+v", read)
	}
	var edited editFileOutput
	args := map[string]any{"page_id": "p1", "path": "a.ts", "old_string": "needle", "new_string": "updated", "expected_hash": read.Hash, "build": false}
	call("edit_file", args, &edited)
	if !edited.OK || edited.Hash == read.Hash || len(edited.Hash) != 64 || edited.Build != nil {
		t.Fatalf("edit %+v", edited)
	}
	args["old_string"], args["new_string"] = "last", "stale"
	conflict, err := cs.CallTool(ctx, &sdkmcp.CallToolParams{Name: "edit_file", Arguments: args})
	if err != nil || !conflict.IsError {
		t.Fatalf("expected tool-level conflict, got %+v %v", conflict, err)
	}
	// An old client still reads the whole file without supplying range options.
	call("read_file", map[string]any{"page_id": "p1", "path": "a.ts"}, &read)
	if read.Content != "first\nupdated\nlast\n" || read.Hash != edited.Hash {
		t.Fatalf("legacy read %+v", read)
	}
	call("grep_files", map[string]any{"page_id": "p1", "pattern": "not found"}, &found)
	if found.Matches == nil || len(found.Matches) != 0 || found.Truncated {
		t.Fatalf("empty search %+v", found)
	}
}

type interveningSourceStore struct{ service.DocSurfaceStore }

func (s interveningSourceStore) CompareAndSwapFile(ctx context.Context, pageID, path string, previous, next []byte) error {
	if err := s.DocSurfaceStore.WriteFile(ctx, pageID, path, []byte("before\nconcurrent\n")); err != nil {
		return err
	}
	return s.DocSurfaceStore.(service.DocSurfaceFileCAS).CompareAndSwapFile(ctx, pageID, path, previous, next)
}

func TestSourceEditRejectsChangeBetweenReadAndWrite(t *testing.T) {
	ctx := service.WithPrincipal(context.Background(), service.Principal{UserID: "author"})
	store := docsurface.NewStore(t.TempDir())
	if err := store.WriteFile(ctx, "p1", "a.ts", []byte("before\n")); err != nil {
		t.Fatal(err)
	}
	builder := &fakeBuild{}
	ts := docToolServer{artifacts: fakeArtifacts{}, store: interveningSourceStore{store}, build: builder}
	_, read, err := ts.readFile(ctx, nil, readFileInput{PageID: "p1", Path: "a.ts"})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = ts.editFile(ctx, nil, editFileInput{PageID: "p1", Path: "a.ts", OldString: "before", NewString: "after", ExpectedHash: &read.Hash})
	if !errors.Is(err, service.ErrConflict) {
		t.Fatalf("expected concurrent-write conflict: %v", err)
	}
	data, _ := store.ReadFile(ctx, "p1", "a.ts")
	history, _ := store.ListDir(ctx, "p1", ".history")
	if string(data) != "before\nconcurrent\n" || len(history) != 0 || len(builder.built) != 0 {
		t.Fatalf("lost concurrent change or had side effects: %q", data)
	}
}
