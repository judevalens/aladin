package ingestion

import (
	"strings"
	"testing"

	"aladin/backend_v2/internal/document"
)

func region(page, ordinal int, class, text string) document.DocumentRegion {
	return document.DocumentRegion{
		Page: page, Ordinal: ordinal, Class: class, Text: text,
		Bbox: []float64{0, 0, 100, 20}, Confidence: 0.9,
	}
}

// Depth comes from the heading's own numbering, because the layout model reports `title`
// with no level and the text layer has no usable typography (§9).
func TestHeadingDepth(t *testing.T) {
	cases := map[string]int{
		"1 Introduction":                 1,
		"3.2 Methods":                    2,
		"3.2.4 Trend-Based Regression":   3,
		"5.4  Summary of Empirical":      2,
		"2. HYPOTHESES":                  1,
		"Abstract":                       0, // unnumbered → top level, not guessed
		"References":                     0,
		"B.2 Performances of Voting":     0, // letter-numbered: not our scheme, stays flat
		"1990 was a good year for bonds": 0, // a year is not a section number
	}
	for title, want := range cases {
		if got := headingDepth(title); got != want {
			t.Errorf("headingDepth(%q) = %d, want %d", title, got, want)
		}
	}
}

func TestBuildChunks_NestsByHeadingNumber(t *testing.T) {
	chunks := BuildChunks([]document.DocumentRegion{
		region(1, 0, "title", "1 Introduction"),
		region(1, 1, "plain text", "Drift is persistent."),
		region(2, 0, "title", "1.1 Prior work"),
		region(2, 1, "plain text", "Others looked at this."),
		region(3, 0, "title", "1.2 Our approach"),
		region(3, 1, "plain text", "We sort by surprise."),
		region(4, 0, "title", "2 Methods"),
		region(4, 1, "plain text", "Sixty day holding period."),
	})

	if len(chunks) != 2 {
		t.Fatalf("top level = %d sections, want 2 (Introduction, Methods)", len(chunks))
	}
	intro := chunks[0]
	if intro.Title != "1 Introduction" || intro.Kind != ChunkSection {
		t.Fatalf("first section = %+v", intro)
	}

	// §11: the section keeps its own body AND its subsections — a tree, not a partition.
	var subsections int
	for _, child := range intro.Children {
		if child.Kind == ChunkSection {
			subsections++
		}
	}
	if subsections != 2 {
		t.Fatalf("Introduction has %d subsections, want 1.1 and 1.2: %+v", subsections, intro.Children)
	}
	if chunks[1].Title != "2 Methods" {
		t.Fatalf("second top-level section = %q", chunks[1].Title)
	}
}

// The anchor guarantee. §13d spends the whole error budget on boundaries and none here:
// a section that claims the wrong pages produces a confident false citation.
func TestBuildChunks_PageSpansAreObservedNotComputed(t *testing.T) {
	chunks := BuildChunks([]document.DocumentRegion{
		region(47, 0, "title", "3.2 Trend-Based Regression"),
		region(47, 1, "plain text", "Setup."),
		region(48, 0, "plain text", "Continued."),
		region(61, 0, "plain text", "Still in this section."),
		region(62, 0, "title", "3.3 Something else"),
		region(62, 1, "plain text", "New topic."),
	})

	if len(chunks) != 2 {
		t.Fatalf("sections = %d", len(chunks))
	}
	first := chunks[0]
	if first.PageFrom != 47 || first.PageTo != 61 {
		t.Fatalf("section spans pp%d–%d, want 47–61 (the union of its regions)", first.PageFrom, first.PageTo)
	}
	if chunks[1].PageFrom != 62 {
		t.Fatalf("second section starts on p%d, want 62", chunks[1].PageFrom)
	}
	// No chunk may claim a page nothing was seen on.
	var check func(c Chunk)
	check = func(c Chunk) {
		if c.PageFrom < 1 || c.PageTo < c.PageFrom {
			t.Fatalf("chunk %q has an impossible span %d–%d", c.Title, c.PageFrom, c.PageTo)
		}
		for _, child := range c.Children {
			if child.PageFrom < c.PageFrom || child.PageTo > c.PageTo {
				t.Fatalf("child %q (%d–%d) escapes parent %q (%d–%d)",
					child.Title, child.PageFrom, child.PageTo, c.Title, c.PageFrom, c.PageTo)
			}
			check(child)
		}
	}
	for _, chunk := range chunks {
		check(chunk)
	}
}

// `abandon` earns its keep here: running heads and library stamps must not end up in text
// an agent will later quote.
func TestBuildChunks_DropsPageFurniture(t *testing.T) {
	chunks := BuildChunks([]document.DocumentRegion{
		region(1, 0, "abandon", "MASSACHUSETTS NS E OF TECHNOLOGY JUN 252008 LIBRARIES"),
		region(1, 1, "title", "1 Introduction"),
		region(1, 2, "plain text", "The real content."),
		region(1, 3, "abandon", "47"),
	})

	all := collectText(chunks)
	if strings.Contains(all, "LIBRARIES") || strings.Contains(all, "MASSACHUSETTS") {
		t.Fatalf("page furniture leaked into the text: %q", all)
	}
	if !strings.Contains(all, "The real content") {
		t.Fatalf("body was dropped: %q", all)
	}
}

// A document with no headings must still chunk — most PDFs have no outline, and a flat
// tree is navigable where a failure is not.
func TestBuildChunks_HeadinglessDocumentStillChunks(t *testing.T) {
	var regions []document.DocumentRegion
	for page := 1; page <= 6; page++ {
		regions = append(regions, region(page, 0, "plain text", strings.Repeat("prose ", 300)))
	}

	chunks := BuildChunks(regions)
	if len(chunks) == 0 {
		t.Fatal("a headingless document produced no chunks at all")
	}
	for _, chunk := range chunks {
		if chunk.Kind != ChunkBlock {
			t.Fatalf("expected flat blocks, got %+v", chunk)
		}
	}
	// It must also SPLIT: 6 pages of prose in one chunk defeats retrieval.
	if len(chunks) < 2 {
		t.Fatalf("long prose was not split: %d chunk(s)", len(chunks))
	}
}

func TestBuildChunks_SplitsOnRegionBoundaries(t *testing.T) {
	// Each region is well under the target, but together they exceed it.
	var regions []document.DocumentRegion
	for i := 0; i < 8; i++ {
		regions = append(regions, region(1, i, "plain text", strings.Repeat("x", 800)))
	}

	chunks := BuildChunks(regions)
	if len(chunks) < 2 {
		t.Fatalf("expected a split, got %d chunk(s)", len(chunks))
	}
	for _, chunk := range chunks {
		// A boundary must fall between regions, so every chunk is a whole number of them.
		if len(chunk.Text)%800 != 0 && !strings.Contains(chunk.Text, "\n\n") {
			t.Fatalf("chunk looks split mid-region: %d chars", len(chunk.Text))
		}
	}
}

// Figures have no text but are part of the section they sit in.
func TestBuildChunks_FigureContributesItsPageNotItsWords(t *testing.T) {
	chunks := BuildChunks([]document.DocumentRegion{
		region(10, 0, "title", "5 Results"),
		region(10, 1, "plain text", "See the chart."),
		region(11, 0, "figure", ""),
		region(11, 1, "figure_caption", "Figure 3: equity curve."),
	})

	if len(chunks) != 1 {
		t.Fatalf("sections = %d", len(chunks))
	}
	if chunks[0].PageTo != 11 {
		t.Fatalf("the section stops at p%d, but its figure is on p11", chunks[0].PageTo)
	}
	// The caption belongs with the body, not stranded alone.
	if !strings.Contains(collectText(chunks), "Figure 3") {
		t.Fatal("the caption was lost")
	}
}

// Persistence writes parents before children so the self-referencing key resolves.
func TestFlattenChunks_ParentsComeFirst(t *testing.T) {
	chunks := BuildChunks([]document.DocumentRegion{
		region(1, 0, "title", "1 One"),
		region(1, 1, "plain text", "body"),
		region(2, 0, "title", "1.1 Nested"),
		region(2, 1, "plain text", "more"),
	})

	seen := map[int]bool{}
	index := 0
	FlattenChunks(chunks, func(_ Chunk, parent int) int {
		if parent >= 0 && !seen[parent] {
			t.Fatalf("child written before its parent (%d)", parent)
		}
		seen[index] = true
		index++
		return index - 1
	})
	if index == 0 {
		t.Fatal("nothing was visited")
	}
}

func collectText(chunks []Chunk) string {
	var parts []string
	var walk func([]Chunk)
	walk = func(nodes []Chunk) {
		for _, node := range nodes {
			parts = append(parts, node.Title, node.Text)
			walk(node.Children)
		}
	}
	walk(chunks)
	return strings.Join(parts, " ")
}
