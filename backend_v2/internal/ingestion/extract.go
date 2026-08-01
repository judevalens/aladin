// Package ingestion turns a stored file into readable text and navigable structure.
//
// The contract is deliberately narrow (design/INGESTION_PRD.md §2): it EXTRACTS, and any
// interpretation on top must stay derived, disposable, anchored to the source, and never
// the thing you query instead of the text. Aladin built the unbounded version of this
// once; it was abandoned with the knowledge graph.
//
// Extraction itself runs in Python (segment.go): text, outline and layout regions from
// ONE pass, so a region's box resolves to the page under it exactly rather than across
// two coordinate models. This file holds the shared vocabulary — the status model, the
// document/page/section types, and the scan test they share.
package ingestion

import "strings"

// Status is the artifact's ingestion state (§4). Every one of these is visible in the
// UI — a spinner that never resolves is the worst outcome, so extraction always lands
// somewhere terminal and says why.
type Status string

const (
	StatusPending     Status = "pending"
	StatusIngesting   Status = "ingesting"
	StatusReady       Status = "ready"
	StatusUnsupported Status = "unsupported"
	StatusFailed      Status = "failed"
)

// Section is one outline entry, read from the document's own bookmarks.
type Section struct {
	Title string `json:"title"`
	Level int    `json:"level"`
	Page  int    `json:"page"` // 1-based
}

// Page is one page's extracted text. Kept per-page rather than as one blob so a section
// (which knows only its page) can resolve to the words under it.
type Page struct {
	Page int    `json:"page"`
	Text string `json:"text"`
}

// Document is everything extraction recovered.
type Document struct {
	Status    Status
	Error     string
	PageCount int
	Pages     []Page
	Sections  []Section
	Extractor string
}

// TextLen is the total extracted character count — the signal that separates a real
// document from a scan.
func (d Document) TextLen() int {
	total := 0
	for _, page := range d.Pages {
		total += len(strings.TrimSpace(page.Text))
	}
	return total
}

// minCharsPerPage is the threshold below which we call a PDF a scan rather than a
// document. Real prose runs hundreds of characters per page; a scanned page yields zero,
// and a page of pure figures yields a handful of axis labels. Averaging over the whole
// document keeps a title page or a plate section from tripping it.
const minCharsPerPage = 24

// normalize squeezes the whitespace PDF text extraction produces. Extractors emit text
// positionally, so line breaks are wherever the glyphs happened to sit; collapsing runs
// keeps the stored text readable and searchable without pretending to reflow paragraphs.
func normalize(text string) string {
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if trimmed := strings.Join(strings.Fields(line), " "); trimmed != "" {
			kept = append(kept, trimmed)
		}
	}
	return strings.Join(kept, "\n")
}
