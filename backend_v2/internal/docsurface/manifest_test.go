package docsurface

import (
	"strings"
	"testing"
)

func TestValidateManifest(t *testing.T) {
	t.Run("valid manifest passes", func(t *testing.T) {
		data := []byte(`{
			"version": 1,
			"intent": "track positions",
			"anchors": [
				{"id": "positions:table", "kind": "collection", "route": "#/", "meaning": "rows", "refs": ["t-nvda"]},
				{"id": "nvda:metric", "kind": "metric", "route": "#/nvda", "meaning": "conviction"}
			]
		}`)
		if problems := ValidateManifestBytes(data); len(problems) != 0 {
			t.Fatalf("expected valid, got problems: %v", problems)
		}
	})

	t.Run("bad JSON is one problem", func(t *testing.T) {
		problems := ValidateManifestBytes([]byte(`{not json`))
		if len(problems) != 1 || !strings.Contains(problems[0], "not valid JSON") {
			t.Fatalf("got %v", problems)
		}
	})

	t.Run("wrong version", func(t *testing.T) {
		problems := ValidateManifest(Manifest{Version: 2})
		if !hasProblem(problems, "version must be 1") {
			t.Fatalf("got %v", problems)
		}
	})

	t.Run("missing required fields", func(t *testing.T) {
		problems := ValidateManifest(Manifest{
			Version: 1,
			Anchors: []ManifestAnchor{{Route: "#/"}}, // no id, no meaning
		})
		if !hasProblem(problems, "id is required") || !hasProblem(problems, "meaning is required") {
			t.Fatalf("got %v", problems)
		}
	})

	t.Run("unknown kind", func(t *testing.T) {
		problems := ValidateManifest(Manifest{
			Version: 1,
			Anchors: []ManifestAnchor{{ID: "a", Route: "#/", Meaning: "m", Kind: "bogus"}},
		})
		if !hasProblem(problems, `unknown kind "bogus"`) {
			t.Fatalf("got %v", problems)
		}
	})

	t.Run("duplicate id on same route", func(t *testing.T) {
		problems := ValidateManifest(Manifest{
			Version: 1,
			Anchors: []ManifestAnchor{
				{ID: "x", Route: "#/", Meaning: "m"},
				{ID: "x", Route: "#/", Meaning: "m"},
			},
		})
		if !hasProblem(problems, `duplicate anchor id "x"`) {
			t.Fatalf("got %v", problems)
		}
	})

	t.Run("same id on different routes is allowed", func(t *testing.T) {
		problems := ValidateManifest(Manifest{
			Version: 1,
			Anchors: []ManifestAnchor{
				{ID: "x", Route: "#/", Meaning: "m"},
				{ID: "x", Route: "#/other", Meaning: "m"},
			},
		})
		if len(problems) != 0 {
			t.Fatalf("expected valid, got %v", problems)
		}
	})
}

func hasProblem(problems []string, substr string) bool {
	for _, p := range problems {
		if strings.Contains(p, substr) {
			return true
		}
	}
	return false
}
