package ingestion

import (
	"strings"
	"testing"
)

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

// A NUL in extracted text is valid UTF-8 but illegal in a Postgres text value:
// it fails the page INSERT with SQLSTATE 22021 and wedges the whole document at
// status='ingesting', which ClaimPending can never re-claim. Real PDFs emit these.
func TestNormalizeStripsBytesPostgresRejects(t *testing.T) {
	got := normalize("page \x00one\nsecond\x07 line\n\x00\n")
	want := "page one\nsecond line"
	if got != want {
		t.Fatalf("normalize() = %q, want %q", got, want)
	}
	if strings.ContainsRune(got, 0) {
		t.Fatal("normalize() left a NUL in the text")
	}
}
