package ingestion

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The happy path is the least interesting thing about a subprocess. What matters is that
// every way it can break produces an error a human can act on — because the caller turns
// these into status='failed' with the message attached, and the one outcome the status
// model exists to prevent (INGESTION_PRD §4) is a row stuck on 'ingesting' forever.

// stubSegmenter writes a fake script so the failure modes can be exercised without the
// real venv, the real model, or a 34-second run.
func stubSegmenter(t *testing.T, body string) *PythonSegmenter {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "fake.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	return &PythonSegmenter{Python: "/bin/sh", Script: script, Timeout: 5 * time.Second}
}

func fixturePDF(t *testing.T) string {
	t.Helper()
	return filepath.Join("testdata", "plain.pdf")
}

func TestSegment_NonZeroExitSurfacesTheScriptsMessage(t *testing.T) {
	seg := stubSegmenter(t, `echo "PDF is password-protected" >&2; exit 1`)

	_, err := seg.Segment(context.Background(), fixturePDF(t))
	if err == nil {
		t.Fatal("a failing script must error")
	}
	// "exit status 1" is useless; the script's own words are the whole point.
	if !strings.Contains(err.Error(), "password-protected") {
		t.Fatalf("error = %q, want the script's stderr", err)
	}
}

func TestSegment_TracebackIsTrimmedToItsLastLine(t *testing.T) {
	seg := stubSegmenter(t, `
printf 'Traceback (most recent call last):\n  File "x", line 1\n    boom()\nValueError: bad page\n' >&2
exit 1`)

	_, err := seg.Segment(context.Background(), fixturePDF(t))
	if err == nil {
		t.Fatal("expected an error")
	}
	// A whole trace does not belong in a status a human reads in a tooltip.
	if !strings.Contains(err.Error(), "ValueError: bad page") {
		t.Fatalf("error = %q, want the traceback's last line", err)
	}
	if strings.Contains(err.Error(), "Traceback") {
		t.Fatalf("the full traceback leaked into the status: %q", err)
	}
}

func TestSegment_UnreadableOutputIsAnError(t *testing.T) {
	// A script that exits 0 but prints garbage is the sneakiest failure: without this
	// check it would look like a successful ingest of an empty document.
	seg := stubSegmenter(t, `echo "not json at all"; exit 0`)

	_, err := seg.Segment(context.Background(), fixturePDF(t))
	if err == nil {
		t.Fatal("garbage on stdout must not read as success")
	}
	if !strings.Contains(err.Error(), "unreadable") {
		t.Fatalf("error = %q", err)
	}
}

func TestSegment_EmptyResultIsAnError(t *testing.T) {
	seg := stubSegmenter(t, `echo '{"page_count":0,"pages":[]}'; exit 0`)

	if _, err := seg.Segment(context.Background(), fixturePDF(t)); err == nil {
		t.Fatal("zero pages must not read as success")
	}
}

func TestSegment_TimeoutIsReportedAsATimeout(t *testing.T) {
	seg := stubSegmenter(t, `sleep 30`)
	seg.Timeout = 300 * time.Millisecond

	start := time.Now()
	_, err := seg.Segment(context.Background(), fixturePDF(t))
	if err == nil {
		t.Fatal("a hung script must not hang the worker")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("error = %q, want a timeout", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("timeout took %s — the deadline isn't being enforced", elapsed)
	}
}

func TestSegment_MissingToolExplainsHowToInstallIt(t *testing.T) {
	seg := &PythonSegmenter{Python: "/nonexistent/python", Script: "/nonexistent/segment.py"}

	_, err := seg.Segment(context.Background(), fixturePDF(t))
	if err == nil {
		t.Fatal("expected an error")
	}
	// An exec error nobody can act on is worse than no feature.
	if !strings.Contains(err.Error(), "venv") {
		t.Fatalf("error = %q, want setup instructions", err)
	}
}

func TestSegment_MissingFileIsCaughtBeforeSpawning(t *testing.T) {
	seg := stubSegmenter(t, `echo '{"page_count":1,"pages":[{"page":1}]}'`)

	if _, err := seg.Segment(context.Background(), "testdata/does-not-exist.pdf"); err == nil {
		t.Fatal("a missing file must error")
	}
}

func TestSegment_CallerCancellationStops(t *testing.T) {
	seg := stubSegmenter(t, `sleep 30`)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	if _, err := seg.Segment(ctx, fixturePDF(t)); err == nil {
		t.Fatal("expected an error on cancellation")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("cancellation took %s — a shutting-down worker would hang", elapsed)
	}
}

// ToDocument keeps the status model's meaning, so the persistence path below it doesn't
// change when extraction moves to Python.
func TestLayoutToDocument(t *testing.T) {
	layout := Layout{
		Extractor: "pdf/pymupdf+doclayout-yolo@1",
		PageCount: 2,
		Outline:   []Section{{Title: "Chapter 1", Level: 0, Page: 1}},
		Pages: []LayoutPage{
			{Page: 1, Text: "Post-earnings drift persists in semiconductors and elsewhere."},
			{Page: 2, Text: "We sort by surprise decile and hold for sixty trading days."},
		},
	}

	doc := layout.ToDocument()
	if doc.Status != StatusReady {
		t.Fatalf("status = %q, want ready", doc.Status)
	}
	if doc.PageCount != 2 || len(doc.Pages) != 2 {
		t.Fatalf("pages = %d / %d", doc.PageCount, len(doc.Pages))
	}
	if len(doc.Sections) != 1 || doc.Sections[0].Title != "Chapter 1" {
		t.Fatalf("outline lost: %+v", doc.Sections)
	}
	if doc.Extractor == "" {
		t.Fatal("extractor stamp must survive — it's how stale rows get re-run")
	}
}

// A scan still lands 'unsupported' through the new path, not a silent empty 'ready'.
func TestLayoutToDocument_ScanStaysUnsupported(t *testing.T) {
	layout := Layout{
		PageCount: 3,
		Pages: []LayoutPage{
			{Page: 1, Text: ""},
			{Page: 2, Text: "  "},
			{Page: 3, Text: "1"},
		},
	}

	doc := layout.ToDocument()
	if doc.Status != StatusUnsupported {
		t.Fatalf("status = %q, want unsupported", doc.Status)
	}
	if !strings.Contains(strings.ToLower(doc.Error), "ocr") {
		t.Fatalf("error = %q, want it to name the next action", doc.Error)
	}
}
