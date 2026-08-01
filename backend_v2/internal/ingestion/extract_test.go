package ingestion

import "testing"

func TestNormalizeCollapsesExtractionWhitespace(t *testing.T) {
	// PDF text comes out positionally, so runs of spaces and blank lines are artefacts of
	// glyph placement rather than meaning. The fixtures that used to exercise the Go
	// extractor now drive the Python one through segment_test.go and the repo's
	// end-to-end test.
	got := normalize("  Hello   world  \n\n\n   second   line \n  ")
	if got != "Hello world\nsecond line" {
		t.Fatalf("normalize = %q", got)
	}
}
