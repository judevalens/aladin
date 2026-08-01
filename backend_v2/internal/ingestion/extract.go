// Package ingestion turns a stored file into readable text and navigable structure.
//
// The contract is deliberately narrow (design/INGESTION_PRD.md §2): it EXTRACTS, it does
// not interpret. Text and an outline are facts about the file. Entities, topics, claims
// and summaries are opinions about its meaning, and every one of them belongs to a
// separate later step that reads this output — not inside the extractor. Aladin already
// built the other version of this once; it was abandoned with the knowledge graph.
//
// The seam is Extract(path) -> Document. A second format is a new extractor behind that
// signature, not a new pipeline.
package ingestion

import (
	"fmt"
	"os"
	"strings"

	ledong "github.com/ledongthuc/pdf"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

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

// ExtractorPDF names the implementation that produced a Document, so a better extractor
// later can detect stale rows and re-run them.
const ExtractorPDF = "pdf/ledongthuc+pdfcpu@1"

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

// ExtractPDF pulls page text and the bookmark outline out of a PDF.
//
// It never returns an error for "this file cannot be read" — that is a Status, not a
// failure of the caller. An error here means the caller did something wrong (bad path).
func ExtractPDF(path string) Document {
	doc := Document{Status: StatusFailed, Extractor: ExtractorPDF}

	info, err := os.Stat(path)
	if err != nil {
		doc.Error = fmt.Sprintf("cannot read file: %v", err)
		return doc
	}
	if info.Size() == 0 {
		doc.Error = "file is empty"
		return doc
	}

	pages, err := extractPages(path)
	if err != nil {
		// Encrypted, truncated, or not really a PDF. The message is surfaced verbatim —
		// a vague "ingestion failed" sends you debugging the wrong thing.
		doc.Error = err.Error()
		return doc
	}
	doc.Pages = pages
	doc.PageCount = len(pages)

	if doc.PageCount == 0 {
		doc.Status = StatusFailed
		doc.Error = "no pages found"
		return doc
	}

	// §4: a scanned book is not a crash and not a success. Naming it tells you the next
	// action (OCR), which "0 characters extracted, status ready" would not.
	if doc.TextLen() < minCharsPerPage*doc.PageCount {
		doc.Status = StatusUnsupported
		doc.Error = "no extractable text layer (likely a scan — needs OCR first)"
		return doc
	}

	// The outline is a bonus, never a reason to fail: plenty of readable PDFs have no
	// bookmarks, and that is an empty outline, not an error.
	if sections, err := extractOutline(path); err == nil {
		doc.Sections = sections
	}

	doc.Status = StatusReady
	return doc
}

// extractPages reads per-page text. ledongthuc/pdf panics on some malformed files, so
// the recover is load-bearing rather than defensive: one bad upload must not take down
// the worker.
func extractPages(path string) (pages []Page, err error) {
	defer func() {
		if r := recover(); r != nil {
			pages, err = nil, fmt.Errorf("malformed PDF: %v", r)
		}
	}()

	file, reader, err := ledong.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open PDF: %w", err)
	}
	defer file.Close()

	count := reader.NumPage()
	out := make([]Page, 0, count)
	for index := 1; index <= count; index++ {
		page := reader.Page(index)
		if page.V.IsNull() {
			out = append(out, Page{Page: index})
			continue
		}
		text, perr := page.GetPlainText(nil)
		if perr != nil {
			// One unreadable page shouldn't lose the other 299.
			out = append(out, Page{Page: index})
			continue
		}
		out = append(out, Page{Page: index, Text: normalize(text)})
	}
	return out, nil
}

// extractOutline reads the PDF's own bookmarks, flattened into a levelled sequence.
func extractOutline(path string) (sections []Section, err error) {
	defer func() {
		if r := recover(); r != nil {
			sections, err = nil, fmt.Errorf("malformed outline: %v", r)
		}
	}()

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	conf := model.NewDefaultConfiguration()
	conf.ValidationMode = model.ValidationRelaxed
	bookmarks, err := api.Bookmarks(file, conf)
	if err != nil {
		return nil, err
	}

	out := []Section{}
	var walk func(items []pdfcpu.Bookmark, level int)
	walk = func(items []pdfcpu.Bookmark, level int) {
		for _, item := range items {
			title := strings.TrimSpace(item.Title)
			if title != "" && item.PageFrom > 0 {
				out = append(out, Section{Title: title, Level: level, Page: item.PageFrom})
			}
			if len(item.Kids) > 0 {
				walk(item.Kids, level+1)
			}
		}
	}
	walk(bookmarks, 0)
	return out, nil
}

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
