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

func (f fakeDocumentService) Pages(_ context.Context, _ string, from, to int) ([]service.DocumentPage, error) {
	out := []service.DocumentPage{}
	for _, page := range f.doc.Pages {
		if (from == 0 || page.Page >= from) && (to == 0 || page.Page <= to) {
			out = append(out, page)
		}
	}
	return out, nil
}

func (f fakeDocumentService) Search(_ context.Context, _, query string, _ int) ([]service.DocumentHit, error) {
	out := []service.DocumentHit{}
	for _, page := range f.doc.Pages {
		if strings.Contains(strings.ToLower(page.Text), strings.ToLower(query)) {
			out = append(out, service.DocumentHit{Page: page.Page, Snippet: page.Text, Score: 1})
		}
	}
	return out, nil
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

func TestJoinPages_MarksPages(t *testing.T) {
	all, truncated := joinPages(readyDoc().Pages, 0)
	if truncated {
		t.Fatal("an unbounded join should not truncate")
	}
	// Page markers are what let a model cite "p. 2" rather than gesture at the document.
	for _, want := range []string{"[p1]", "[p2]", "[p3]", "sixty days"} {
		if !strings.Contains(all, want) {
			t.Fatalf("joined text missing %q:\n%s", want, all)
		}
	}
}

func TestJoinPages_BudgetTruncates(t *testing.T) {
	text, truncated := joinPages(readyDoc().Pages, 60)
	if !truncated {
		t.Fatal("a tiny budget must report truncation, or the model will think it read everything")
	}
	if len(text) > 60 {
		t.Fatalf("budget exceeded: %d chars", len(text))
	}
}

func TestGetArtifact_ReturnsOutlineButNeverText(t *testing.T) {
	tools := workspaceToolServer{
		artifacts: fakeArtifactGetter{art: service.ArtifactResponse{ID: "a1", Title: "PEAD paper", Type: "file"}},
		documents: fakeDocumentService{doc: readyDoc()},
	}

	_, out, err := tools.getArtifact(context.Background(), nil, getArtifactInput{ArtifactID: "a1"})
	if err != nil {
		t.Fatalf("getArtifact: %v", err)
	}
	// The whole point of the correction: looking an artifact UP must not cost you the
	// document. Text is only ever returned by a deliberate read.
	if out.Text != "" {
		t.Fatalf("get_artifact must not return document text, got %q", out.Text)
	}
	if !strings.Contains(out.More, "search_document") {
		t.Fatalf("get_artifact must point at search_document, got %q", out.More)
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

// The outline is bounded too. A 400-page book can carry hundreds of bookmarks, and an
// outline that fills the context is the same mistake as text that does — just quieter.
func TestCapOutline_KeepsTheTopAndSaysWhenItTrims(t *testing.T) {
	small := []service.DocumentSection{{Title: "One", Page: 1}, {Title: "Two", Page: 9}}
	got, note := capOutline(small)
	if len(got) != 2 || note != "" {
		t.Fatalf("a small outline must pass through untouched: %d entries, note %q", len(got), note)
	}

	// 20 chapters, each with 20 subsections: 420 entries, far past the cap.
	var big []service.DocumentSection
	for chapter := 1; chapter <= 20; chapter++ {
		big = append(big, service.DocumentSection{Title: "Chapter", Level: 0, Page: chapter * 10})
		for sub := 0; sub < 20; sub++ {
			big = append(big, service.DocumentSection{Title: "Section", Level: 1, Page: chapter*10 + sub})
		}
	}
	got, note = capOutline(big)
	if len(got) > maxOutlineEntries {
		t.Fatalf("outline not capped: %d entries", len(got))
	}
	if note == "" {
		t.Fatal("a truncated outline must say so — a partial contents page presented as complete misleads")
	}
	// It should keep chapters, not an arbitrary prefix of subsections.
	for _, entry := range got {
		if entry.Level != 0 {
			t.Fatalf("capping should prefer top-level entries, got level %d", entry.Level)
		}
	}
}

func TestSearchDocument_ReturnsSnippetsNotTheDocument(t *testing.T) {
	tools := workspaceToolServer{
		artifacts: fakeArtifactGetter{art: service.ArtifactResponse{ID: "a1", Title: "PEAD paper", Type: "file"}},
		documents: fakeDocumentService{doc: readyDoc()},
	}

	_, out, err := tools.searchDocument(context.Background(), nil, searchDocumentInput{ArtifactID: "a1", Query: "decile"})
	if err != nil {
		t.Fatalf("searchDocument: %v", err)
	}
	if len(out.Hits) != 1 || out.Hits[0].Page != 2 {
		t.Fatalf("hits = %+v, want one on page 2", out.Hits)
	}
	// A hit must carry its page, or it can't be cited or expanded.
	if out.Hits[0].Snippet == "" {
		t.Fatal("a hit without a snippet is useless")
	}
	if !strings.Contains(out.Note, "read_document") {
		t.Fatalf("search should point at the follow-up read, got %q", out.Note)
	}
}

func TestSearchDocument_EmptyResultSaysWhy(t *testing.T) {
	tools := workspaceToolServer{
		artifacts: fakeArtifactGetter{art: service.ArtifactResponse{ID: "a1", Title: "PEAD paper", Type: "file"}},
		documents: fakeDocumentService{doc: readyDoc()},
	}
	_, out, err := tools.searchDocument(context.Background(), nil, searchDocumentInput{ArtifactID: "a1", Query: "cryptocurrency"})
	if err != nil {
		t.Fatalf("searchDocument: %v", err)
	}
	if len(out.Hits) != 0 {
		t.Fatalf("unexpected hits: %+v", out.Hits)
	}
	// "No results" from keyword search means something different from "not in the
	// document", and an agent that can't tell will give up too early.
	if !strings.Contains(out.Note, "semantic") {
		t.Fatalf("an empty result must explain the search's limits, got %q", out.Note)
	}
}

// A range read must never be a way to pull a whole book through in one call.
func TestReadDocument_CapsTheSpan(t *testing.T) {
	doc := readyDoc()
	doc.PageCount = 500
	for page := 4; page <= 500; page++ {
		doc.Pages = append(doc.Pages, service.DocumentPage{Page: page, Text: "filler"})
	}
	tools := workspaceToolServer{
		artifacts: fakeArtifactGetter{art: service.ArtifactResponse{ID: "a1", Title: "Book", Type: "file"}},
		documents: fakeDocumentService{doc: doc},
	}

	_, out, err := tools.readDocument(context.Background(), nil, readDocumentInput{ArtifactID: "a1"})
	if err != nil {
		t.Fatalf("readDocument: %v", err)
	}
	if out.ToPage-out.FromPage+1 > maxReadPages {
		t.Fatalf("read spanned %d pages, cap is %d", out.ToPage-out.FromPage+1, maxReadPages)
	}
	if out.More == "" {
		t.Fatal("a capped read must say where to continue from")
	}
}
