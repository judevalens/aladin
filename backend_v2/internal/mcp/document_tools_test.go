package mcpserver

import (
	"context"
	"strings"
	"testing"

	"aladin/backend_v2/internal/service"
)

// These cover the gap that made ingestion useless to the copilot: get_artifact on a file
// returned a filename, so an agent concluded the document was empty. RESEARCH_SURFACE_PRD
// §3 #9 wants captured material to be something agents can run over.

type fakeDocumentService struct{ doc service.Document }

func (f fakeDocumentService) Get(_ context.Context, _ string, _ bool) (service.Document, error) {
	return f.doc, nil
}

// Only Get is exercised here; embedding the interface leaves every other method nil so
// an accidental call fails loudly rather than silently returning a zero value.
type fakeArtifactGetter struct {
	service.ArtifactService
	art service.ArtifactResponse
}

func (f fakeArtifactGetter) Get(_ context.Context, _ string) (service.ArtifactResponse, error) {
	return f.art, nil
}

func readyDoc() service.Document {
	return service.Document{
		Status:    "ready",
		PageCount: 3,
		Sections: []service.DocumentSection{
			{Title: "Introduction", Level: 0, Page: 1},
			{Title: "Method", Level: 1, Page: 2},
		},
		Pages: []service.DocumentPage{
			{Page: 1, Text: "Post-earnings drift persists in semiconductors."},
			{Page: 2, Text: "We sort by surprise decile and hold sixty days."},
			{Page: 3, Text: "Results are robust to transaction costs."},
		},
	}
}

func TestJoinPages_MarksPagesAndRespectsRange(t *testing.T) {
	doc := readyDoc()

	all, truncated := joinPages(doc.Pages, 0, 0, 0)
	if truncated {
		t.Fatal("an unbounded join should not truncate")
	}
	// Page markers are what let a model cite "p. 2" rather than gesture at the document.
	for _, want := range []string{"[p1]", "[p2]", "[p3]", "sixty days"} {
		if !strings.Contains(all, want) {
			t.Fatalf("joined text missing %q:\n%s", want, all)
		}
	}

	ranged, _ := joinPages(doc.Pages, 2, 2, 0)
	if !strings.Contains(ranged, "sixty days") {
		t.Fatalf("range 2-2 lost its page: %q", ranged)
	}
	if strings.Contains(ranged, "semiconductors") || strings.Contains(ranged, "transaction costs") {
		t.Fatalf("range 2-2 leaked neighbouring pages: %q", ranged)
	}
}

func TestJoinPages_BudgetTruncates(t *testing.T) {
	doc := readyDoc()
	text, truncated := joinPages(doc.Pages, 0, 0, 60)
	if !truncated {
		t.Fatal("a tiny budget must report truncation, or the model will think it read everything")
	}
	if len(text) > 60 {
		t.Fatalf("budget exceeded: %d chars", len(text))
	}
}

func TestGetArtifact_ReturnsDocumentTextAndOutline(t *testing.T) {
	tools := workspaceToolServer{
		artifacts: fakeArtifactGetter{art: service.ArtifactResponse{ID: "a1", Title: "PEAD paper", Type: "file"}},
		documents: fakeDocumentService{doc: readyDoc()},
	}

	_, out, err := tools.getArtifact(context.Background(), nil, getArtifactInput{ArtifactID: "a1"})
	if err != nil {
		t.Fatalf("getArtifact: %v", err)
	}
	if !strings.Contains(out.Text, "semiconductors") {
		t.Fatalf("the document's text must come back, got %q", out.Text)
	}
	if out.PageCount != 3 {
		t.Fatalf("page count = %d, want 3", out.PageCount)
	}
	if len(out.Outline) != 2 || out.Outline[1].Level != 1 {
		t.Fatalf("outline = %+v, want both entries with nesting preserved", out.Outline)
	}
	if out.Unreadable != "" {
		t.Fatalf("a readable document must not be flagged unreadable: %q", out.Unreadable)
	}
}

// The case where being wrong is worse than being empty: a scan yields no text, and an
// agent that treats that as "nothing to say" will answer confidently about nothing.
func TestGetArtifact_ScanTellsTheAgentItCannotRead(t *testing.T) {
	tools := workspaceToolServer{
		artifacts: fakeArtifactGetter{art: service.ArtifactResponse{ID: "a2", Title: "Scanned book", Type: "file"}},
		documents: fakeDocumentService{doc: service.Document{
			Status:    "unsupported",
			Error:     "no extractable text layer (likely a scan — needs OCR first)",
			PageCount: 400,
		}},
	}

	_, out, err := tools.getArtifact(context.Background(), nil, getArtifactInput{ArtifactID: "a2"})
	if err != nil {
		t.Fatalf("getArtifact: %v", err)
	}
	if out.Unreadable == "" {
		t.Fatal("an unreadable document must say so rather than looking empty")
	}
	if !strings.Contains(strings.ToLower(out.Unreadable), "ocr") {
		t.Fatalf("unreadable = %q, want the actionable reason", out.Unreadable)
	}
}

func TestGetArtifact_StillWorksWithoutDocuments(t *testing.T) {
	// A note or a link has no document, and the whole path must stay inert for them.
	tools := workspaceToolServer{
		artifacts: fakeArtifactGetter{art: service.ArtifactResponse{ID: "a3", Title: "A note", Type: "page", Content: "plain body"}},
	}
	_, out, err := tools.getArtifact(context.Background(), nil, getArtifactInput{ArtifactID: "a3"})
	if err != nil {
		t.Fatalf("getArtifact: %v", err)
	}
	if out.Text != "plain body" || out.PageCount != 0 || out.Unreadable != "" {
		t.Fatalf("non-file artifact was altered: %+v", out)
	}
}

func TestReadDocument_ReadsARange(t *testing.T) {
	tools := workspaceToolServer{
		artifacts: fakeArtifactGetter{art: service.ArtifactResponse{ID: "a1", Title: "PEAD paper", Type: "file"}},
		documents: fakeDocumentService{doc: readyDoc()},
	}

	_, out, err := tools.readDocument(context.Background(), nil, readDocumentInput{ArtifactID: "a1", FromPage: 2, ToPage: 3})
	if err != nil {
		t.Fatalf("readDocument: %v", err)
	}
	if out.FromPage != 2 || out.ToPage != 3 || out.PageCount != 3 {
		t.Fatalf("range = %d-%d of %d", out.FromPage, out.ToPage, out.PageCount)
	}
	if strings.Contains(out.Text, "semiconductors") {
		t.Fatalf("page 1 leaked into a 2-3 read: %q", out.Text)
	}
	if !strings.Contains(out.Text, "transaction costs") {
		t.Fatalf("page 3 missing from a 2-3 read: %q", out.Text)
	}
	if len(out.Citations) == 0 {
		t.Fatal("a document read must carry a citation back")
	}
}

func TestReadDocument_RefusesAnUnreadableDocument(t *testing.T) {
	tools := workspaceToolServer{
		artifacts: fakeArtifactGetter{art: service.ArtifactResponse{ID: "a2", Title: "Scan", Type: "file"}},
		documents: fakeDocumentService{doc: service.Document{Status: "unsupported", Error: "needs OCR first"}},
	}
	if _, _, err := tools.readDocument(context.Background(), nil, readDocumentInput{ArtifactID: "a2"}); err == nil {
		t.Fatal("reading an unreadable document must error, not return empty text")
	}
}
