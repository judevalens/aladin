package docsurface

import (
	"strings"
	"testing"
)

// EntryHTML must wire runtime Tailwind into every shard: the theme in a
// type="text/tailwindcss" block (so the engine compiles the @theme into
// utilities) + the engine script + the embedded design tokens.
func TestEntryHTMLInjectsRuntimeTailwind(t *testing.T) {
	out := EntryHTML("Shard", TokensCSS, "", "console.log(0)", ImportMap{Imports: map[string]string{}}, "")
	for _, want := range []string{
		`<style type="text/tailwindcss">`, // the Tailwind input block
		`@import "tailwindcss"`,            // pulls in utilities/preflight
		"--panel:",                         // embedded canonical theme present
		"@tailwindcss/browser",            // the engine bundle (jsdelivr header marker)
	} {
		if !strings.Contains(out, want) {
			t.Errorf("EntryHTML output missing %q", want)
		}
	}
}
