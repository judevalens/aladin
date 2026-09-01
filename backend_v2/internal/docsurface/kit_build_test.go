package docsurface

import (
	"strings"
	"testing"
)

// The embedded kit must compile (esbuild, react externalized) and expose the L3
// viz/color helpers + L4 generic-UI primitives alongside the core exports.
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
	for _, sym := range []string{
		"Region", "useRoute", // core
		"chartSeries", "chartAxis", "chartGrid", "chartTooltip", "tok", // L3 viz
		"Button", "Card", "Badge", "Input", "Textarea", "Field", "Callout", "Stat", "Tabs", "Dialog", "Divider", // L4 UI
		"DataTable", "KeyValue", "MetricRow", "Sparkline", "Delta", "ProgressBar", // L4b data display
		"AppShell", "SearchInput", "Select", "Checkbox", "RadioGroup", "EmptyState", "LoadingState", "useToast", "Toasts", // L4c chrome + forms
		"Quiz", "Flashcards", "Timer", "Checklist", "Stepper", // L4d interactive/stateful
		"useTheme", "kv", "useShardState", "useKV", // platform + local state
		"bridge", "useNode", "useNodes", // L5 bridge
		"useResource", "queryResource", "resourceRequestId", "executeGraphQL", "invokeLambda", // Shard v2 data/runtime client
	} {
		if !strings.Contains(js, sym) {
			t.Fatalf("kit bundle missing exported symbol %q", sym)
		}
	}
}
