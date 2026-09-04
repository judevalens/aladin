package docsurface

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"aladin/backend_v2/internal/service"
)

func searchStore(t *testing.T, files map[string]string) *localStore {
	t.Helper()
	store := NewStore(t.TempDir()).(*localStore)
	if _, err := store.EnsurePageDir(testCtx(), "p1"); err != nil {
		t.Fatal(err)
	}
	for name, data := range files {
		if err := store.WriteFile(testCtx(), "p1", name, []byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	return store
}

func TestSourceSearchLiteralRegexContextAndGlobs(t *testing.T) {
	store := searchStore(t, map[string]string{
		"index.tsx":              "before\r\nuseState(0)\r\nafter\r\n",
		"components/Card.tsx":    "useState(1)\nUSESTATE(2)\n",
		"components/nested/a.ts": "useState(3)\n",
		"style.css":              "useState(4)\n",
	})
	no := false
	for _, tc := range []struct {
		name  string
		opts  service.SourceSearchOptions
		paths []string
	}{
		{"literal", service.SourceSearchOptions{Pattern: "useState(0)"}, []string{"index.tsx"}},
		{"regex", service.SourceSearchOptions{Pattern: `^useState\([0-9]\)$`, Regex: true, Glob: "**/*.tsx"}, []string{"components/Card.tsx", "index.tsx"}},
		{"basename glob", service.SourceSearchOptions{Pattern: "useState", Glob: "*.tsx"}, []string{"components/Card.tsx", "index.tsx"}},
		{"subtree", service.SourceSearchOptions{Pattern: "useState", Glob: "components/**"}, []string{"components/Card.tsx", "components/nested/a.ts"}},
		{"case insensitive", service.SourceSearchOptions{Pattern: "usestate", CaseSensitive: &no, Glob: "components/*.tsx"}, []string{"components/Card.tsx", "components/Card.tsx"}},
		{"case sensitive default", service.SourceSearchOptions{Pattern: "usestate"}, nil},
		{"filename is not source", service.SourceSearchOptions{Pattern: "Card.tsx"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := store.GrepFiles(testCtx(), "p1", tc.opts)
			if err != nil {
				t.Fatal(err)
			}
			var paths []string
			for _, hit := range got.Matches {
				paths = append(paths, hit.Path)
			}
			if !reflect.DeepEqual(paths, tc.paths) || got.Truncated || got.Matches == nil {
				t.Fatalf("got %+v, want %v", got, tc.paths)
			}
			again, err := store.GrepFiles(testCtx(), "p1", tc.opts)
			if err != nil || !reflect.DeepEqual(got, again) {
				t.Fatal("search is not deterministic")
			}
		})
	}
	got, err := store.GrepFiles(testCtx(), "p1", service.SourceSearchOptions{Pattern: "useState(0)", ContextLines: 5})
	if err != nil || len(got.Matches) != 1 {
		t.Fatalf("%+v %v", got, err)
	}
	hit := got.Matches[0]
	if hit.Line != 2 || hit.Text != "useState(0)" || !reflect.DeepEqual(hit.Before, []service.SourceLine{{Line: 1, Text: "before"}}) || !reflect.DeepEqual(hit.After, []service.SourceLine{{Line: 3, Text: "after"}}) {
		t.Fatalf("context %+v", hit)
	}
}

func TestSourceSearchExclusionsAndIsolation(t *testing.T) {
	store := searchStore(t, map[string]string{
		"a.ts":               "needle",
		"dist/bundle.js":     "needle",
		"node_modules/x.js":  "needle",
		"vendor/x.js":        "needle",
		".history/a.ts":      "needle",
		".env":               "needle",
		"nested/.git/config": "needle",
		"binary.bin":         "needle\x00data",
		"invalid.bin":        "needle\xff",
		"large.txt":          strings.Repeat("x", searchFileBytes+1) + "needle",
	})
	ctx := testCtx()
	if err := store.WriteFile(ctx, "p2", "secret.ts", []byte("needle")); err != nil {
		t.Fatal(err)
	}
	other := service.WithPrincipal(context.Background(), service.Principal{UserID: "other"})
	if err := store.WriteFile(other, "p1", "secret.ts", []byte("needle")); err != nil {
		t.Fatal(err)
	}
	base, _ := store.PageDir(ctx, "p1")
	otherBase, _ := store.PageDir(ctx, "p2")
	for link, target := range map[string]string{"link.ts": filepath.Join(otherBase, "secret.ts"), "linked": otherBase, "internal.ts": filepath.Join(base, "a.ts")} {
		if err := os.Symlink(target, filepath.Join(base, link)); err != nil {
			t.Fatal(err)
		}
	}
	got, err := store.GrepFiles(ctx, "p1", service.SourceSearchOptions{Pattern: "needle"})
	if err != nil || len(got.Matches) != 1 || got.Matches[0].Path != "a.ts" || got.FilesSkipped < 6 {
		t.Fatalf("isolation: %+v %v", got, err)
	}
	if _, err := store.GrepFiles(context.Background(), "p1", service.SourceSearchOptions{Pattern: "needle"}); err == nil {
		t.Fatal("unauthenticated search succeeded")
	}
	if _, err := store.GrepFiles(ctx, "../p2", service.SourceSearchOptions{Pattern: "needle"}); err == nil {
		t.Fatal("page traversal succeeded")
	}
}

func TestSourceSearchValidation(t *testing.T) {
	store := searchStore(t, nil)
	for _, opts := range []service.SourceSearchOptions{
		{}, {Pattern: strings.Repeat("a", 4097)}, {Pattern: "a\nb"},
		{Pattern: "[", Regex: true}, {Pattern: "x", Glob: "["},
		{Pattern: "x", Glob: "../secret"}, {Pattern: "x", Glob: "/secret"},
		{Pattern: "x", Glob: "a\\b"}, {Pattern: "x", Glob: "a//b"},
		{Pattern: "x", ContextLines: -1}, {Pattern: "x", ContextLines: 6},
		{Pattern: "x", MaxMatches: -1}, {Pattern: "x", MaxMatches: 201},
	} {
		if _, err := store.GrepFiles(testCtx(), "p1", opts); err == nil {
			t.Fatalf("accepted %+v", opts)
		}
	}
	ctx, cancel := context.WithCancel(testCtx())
	cancel()
	if _, err := store.GrepFiles(ctx, "p1", service.SourceSearchOptions{Pattern: "x"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestSourceSearchLimits(t *testing.T) {
	t.Run("match limit is exact", func(t *testing.T) {
		store := searchStore(t, map[string]string{"a.ts": "hit\nhit\nhit\n"})
		got, err := store.GrepFiles(testCtx(), "p1", service.SourceSearchOptions{Pattern: "hit", MaxMatches: 2})
		if err != nil || len(got.Matches) != 2 || !got.Truncated || got.TruncationReason != "max_matches" {
			t.Fatalf("%+v %v", got, err)
		}
		got, err = store.GrepFiles(testCtx(), "p1", service.SourceSearchOptions{Pattern: "hit", MaxMatches: 3})
		if err != nil || got.Truncated || len(got.Matches) != 3 {
			t.Fatalf("exact complete: %+v %v", got, err)
		}
	})
	t.Run("long lines preserve UTF8", func(t *testing.T) {
		store := searchStore(t, map[string]string{"a.ts": strings.Repeat("界", 1000) + "needle\n"})
		got, err := store.GrepFiles(testCtx(), "p1", service.SourceSearchOptions{Pattern: "needle"})
		if err != nil || len(got.Matches) != 1 || !got.Matches[0].Truncated || !utf8.ValidString(got.Matches[0].Text) || len(got.Matches[0].Text) > searchLineBytes {
			t.Fatalf("%+v %v", got, err)
		}
	})
	t.Run("JSON output bounded including escaping", func(t *testing.T) {
		line := strings.Repeat("\t", searchLineBytes-3) + "hit\n"
		store := searchStore(t, map[string]string{"a.ts": strings.Repeat(line, 200)})
		got, err := store.GrepFiles(testCtx(), "p1", service.SourceSearchOptions{Pattern: "hit", MaxMatches: 200, ContextLines: 5})
		data, _ := json.Marshal(got.Matches)
		if err != nil || !got.Truncated || got.TruncationReason != "output_limit" || len(data) > searchOutputBytes+2 {
			t.Fatalf("size=%d truncated=%v reason=%s err=%v", len(data), got.Truncated, got.TruncationReason, err)
		}
	})
	t.Run("scan bytes bounded", func(t *testing.T) {
		files := map[string]string{}
		for i := 0; i < 17; i++ {
			files[fmt.Sprintf("%02d.ts", i)] = strings.Repeat("x", searchFileBytes)
		}
		store := searchStore(t, files)
		got, err := store.GrepFiles(testCtx(), "p1", service.SourceSearchOptions{Pattern: "absent"})
		if err != nil || !got.Truncated || got.TruncationReason != "byte_limit" || got.FilesSearched != 16 {
			t.Fatalf("%+v %v", got, err)
		}
	})
	t.Run("file count bounded", func(t *testing.T) {
		files := map[string]string{}
		for i := 0; i < 1001; i++ {
			files[fmt.Sprintf("%04d.ts", i)] = "x"
		}
		store := searchStore(t, files)
		got, err := store.GrepFiles(testCtx(), "p1", service.SourceSearchOptions{Pattern: "absent"})
		if err != nil || !got.Truncated || got.TruncationReason != "file_limit" || got.FilesSearched != 1000 {
			t.Fatalf("%+v %v", got, err)
		}
	})
}

func TestSourceGlob(t *testing.T) {
	for _, tc := range []struct {
		pattern, name string
		want          bool
	}{
		{"**/*.tsx", "index.tsx", true}, {"**/*.tsx", "a/b/index.tsx", true},
		{"*.tsx", "a/index.tsx", true}, {"a/*.tsx", "a/b/index.tsx", false},
		{"a/**/b.ts", "a/b.ts", true}, {"a/**/b.ts", "a/x/y/b.ts", true},
		{"**/**/b.ts", "a/x/b.ts", true}, {"a/**", "a/b/c.ts", true},
		{"a/[ab]?.ts", "a/ab.ts", true}, {"a/[ab]?.ts", "a/cb.ts", false},
	} {
		match, err := sourceGlob(tc.pattern)
		if err != nil || match(tc.name) != tc.want {
			t.Fatalf("%q %q want %v: %v", tc.pattern, tc.name, tc.want, err)
		}
	}
}
