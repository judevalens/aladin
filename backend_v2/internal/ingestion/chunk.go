package ingestion

import (
	"regexp"
	"strconv"
	"strings"

	"aladin/backend_v2/internal/document"
)

// chunk.go — regions become a navigable tree (design/INGESTION_PRD.md §11).
//
// §11 is explicit that this is a TREE and not a partition: a chapter is a chunk *and*
// contains chunks. Flattening to leaves throws away the structure that was expensive to
// recover, and it costs multi-resolution retrieval — matching coarse for "what is this
// about" and fine for "what exactly does it claim".
//
// Everything here is deterministic. The LLM's turn comes later (§10 stage 4), over
// regions that are already the right size *because* of this pass.

// ChunkKind separates the structure from the content hanging off it.
type ChunkKind string

const (
	// ChunkSection is a heading and everything under it — an internal node.
	ChunkSection ChunkKind = "section"
	// ChunkBlock is a leaf: contiguous body regions, sized to be retrievable.
	ChunkBlock ChunkKind = "block"
)

// Chunk is one node of the tree. Page spans come from the REGIONS that formed it, never
// from arithmetic — §13d spends no error budget on anchors.
type Chunk struct {
	Ordinal  int
	Depth    int
	Kind     ChunkKind
	Title    string
	PageFrom int
	PageTo   int
	Text     string
	Children []Chunk
}

// TargetChunkChars is the size a leaf aims for. Big enough to hold an argument, small
// enough that a retrieval hit is worth reading — and split only at region boundaries, so
// a chunk never begins mid-paragraph.
const TargetChunkChars = 2400

// skipClasses never reach a chunk. `abandon` is running heads, page numbers and library
// stamps — the class earns its keep here, by keeping furniture out of the text an agent
// will later quote.
var skipClasses = map[string]bool{"abandon": true}

// Captions (figure_caption, table_caption, formula_caption, table_footnote) are NOT
// special-cased: they are body regions, so they accumulate into the same block as the
// figure or table they describe. A caption stranded in its own chunk is a sentence with
// no referent.

// numberedHeading pulls the depth out of a heading's own numbering: "3.2.4 Trend-Based
// Regression" is three levels deep, "1 Introduction" is one.
//
// This is how depth is inferred, because the layout model reports `title` without a level
// and the text layer carries no usable typography (§9). It degrades honestly: a document
// whose headings aren't numbered comes out flat rather than wrongly nested, and a flat
// tree is still navigable.
var numberedHeading = regexp.MustCompile(`^(\d+(?:\.\d+)*)[.)]?\s+\S`)

// maxSectionNumber separates a section number from a year. "1990 was a good year for
// bonds" is a heading in a finance paper, not section 1990 — and a corpus of trading
// research is full of them. Multi-part numbers ("3.2") are unambiguous and exempt.
const maxSectionNumber = 100

func headingDepth(title string) int {
	match := numberedHeading.FindStringSubmatch(strings.TrimSpace(title))
	if match == nil {
		return 0
	}
	parts := strings.Split(match[1], ".")
	if len(parts) == 1 {
		n, err := strconv.Atoi(parts[0])
		if err != nil || n >= maxSectionNumber {
			return 0 // a year, or a figure reference — not a section number
		}
	}
	return len(parts)
}

// BuildChunks turns a document's regions into a tree.
//
// Regions must arrive in reading order (page asc, then ordinal), which is how they are
// stored. Anything without text contributes its page span but no words — a figure is part
// of the section it sits in even though it has nothing to quote.
func BuildChunks(regions []document.DocumentRegion) []Chunk {
	root := &Chunk{Kind: ChunkSection, Depth: -1}
	stack := []*Chunk{root}
	var body []document.DocumentRegion

	// flush turns the accumulated body regions into leaves under the open section,
	// splitting on region boundaries once a leaf would exceed the target.
	flush := func() {
		if len(body) == 0 {
			return
		}
		parent := stack[len(stack)-1]
		var current []document.DocumentRegion
		size := 0
		emit := func() {
			if len(current) == 0 {
				return
			}
			parent.Children = append(parent.Children, leafFrom(current, parent.Depth+1, len(parent.Children)))
			current = nil
			size = 0
		}
		for _, region := range body {
			length := len(strings.TrimSpace(region.Text))
			// Split BEFORE adding, so a chunk never starts mid-region — the boundary is
			// always something the layout model actually saw.
			if size > 0 && size+length > TargetChunkChars {
				emit()
			}
			current = append(current, region)
			size += length
		}
		emit()
		body = nil
	}

	for _, region := range regions {
		if skipClasses[region.Class] {
			continue
		}
		if region.Class == "title" && strings.TrimSpace(region.Text) != "" {
			flush()
			title := strings.Join(strings.Fields(region.Text), " ")
			depth := headingDepth(title)

			// Pop back to this heading's parent. A depth-0 heading (unnumbered) always
			// attaches to the root, which keeps an unnumbered document flat rather than
			// nesting it by accident of ordering.
			for len(stack) > 1 && stack[len(stack)-1].Depth >= depth {
				stack = stack[:len(stack)-1]
			}
			parent := stack[len(stack)-1]
			section := Chunk{
				Ordinal:  len(parent.Children),
				Depth:    depth,
				Kind:     ChunkSection,
				Title:    title,
				PageFrom: region.Page,
				PageTo:   region.Page,
			}
			parent.Children = append(parent.Children, section)
			stack = append(stack, &parent.Children[len(parent.Children)-1])
			continue
		}
		body = append(body, region)
	}
	flush()

	propagateSpans(root)
	return root.Children
}

// leafFrom builds a block from contiguous regions. Its span is the min/max page of what
// went into it — measured, not computed.
func leafFrom(regions []document.DocumentRegion, depth, ordinal int) Chunk {
	chunk := Chunk{Ordinal: ordinal, Depth: depth, Kind: ChunkBlock, PageFrom: regions[0].Page, PageTo: regions[0].Page}
	var parts []string
	for _, region := range regions {
		if region.Page < chunk.PageFrom {
			chunk.PageFrom = region.Page
		}
		if region.Page > chunk.PageTo {
			chunk.PageTo = region.Page
		}
		if text := strings.TrimSpace(region.Text); text != "" {
			// Captions ride with the body they describe rather than being separated out.
			parts = append(parts, text)
		}
	}
	chunk.Text = strings.Join(parts, "\n\n")
	return chunk
}

// propagateSpans widens each section to cover its children. A section's pages are the
// union of what it contains — again taken from regions, so "section 3 spans pp. 47–61" is
// something we observed rather than estimated.
func propagateSpans(chunk *Chunk) {
	for index := range chunk.Children {
		child := &chunk.Children[index]
		propagateSpans(child)
		if child.PageFrom == 0 {
			continue
		}
		if chunk.PageFrom == 0 || child.PageFrom < chunk.PageFrom {
			chunk.PageFrom = child.PageFrom
		}
		if child.PageTo > chunk.PageTo {
			chunk.PageTo = child.PageTo
		}
	}
}

// FlattenChunks walks the tree depth-first, which is the order it persists in — a parent
// is always written before its children so the self-referencing key resolves.
func FlattenChunks(chunks []Chunk, visit func(chunk Chunk, parentIndex int) int) {
	var walk func(nodes []Chunk, parent int)
	walk = func(nodes []Chunk, parent int) {
		for _, node := range nodes {
			index := visit(node, parent)
			walk(node.Children, index)
		}
	}
	walk(chunks, -1)
}
