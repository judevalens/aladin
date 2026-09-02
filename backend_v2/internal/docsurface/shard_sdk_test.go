package docsurface

import (
	"strings"
	"testing"
)

// The public shard package is deliberately nonvisual. It carries the host data
// clients needed by authored code while the document receives design tokens as CSS.
func TestBuildShardSDKExportsPlatformAPIsWithoutUIComponents(t *testing.T) {
	b := &builder{}
	out, err := b.buildShardSDK()
	if err != nil {
		t.Fatalf("buildShardSDK: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("empty shard SDK bundle")
	}
	js := string(out)
	for _, symbol := range []string{
		"useResource", "queryResource", "resourceRequestId", "executeGraphQL", "invokeLambda",
		"useTheme", "kv", "useShardState", "useKV", "bridge", "useNode", "useNodes",
	} {
		if !strings.Contains(js, symbol) {
			t.Fatalf("shard SDK bundle missing exported symbol %q", symbol)
		}
	}
	for _, removed := range []string{
		"AppShell", "DataTable", "MetricRow", "Button", "Card", "Dialog", "Quiz", "Flashcards", "Stepper", "Region",
	} {
		if strings.Contains(js, removed) {
			t.Fatalf("shard SDK bundle still contains removed UI symbol %q", removed)
		}
	}
}
