package docsurface

import (
	"strings"
	"testing"
)

// The embedded kit must compile (esbuild, react externalized) and expose the new
// L3 viz/color helpers alongside the core exports.
func TestBuildKitCompilesWithVizHelpers(t *testing.T) {
	b := &builder{}
	out, err := b.buildKit()
	if err != nil {
		t.Fatalf("buildKit: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("empty kit bundle")
	}
	js := string(out)
	for _, sym := range []string{"chartSeries", "chartAxis", "chartGrid", "chartTooltip", "tok", "Region", "useRoute"} {
		if !strings.Contains(js, sym) {
			t.Fatalf("kit bundle missing exported symbol %q", sym)
		}
	}
}
