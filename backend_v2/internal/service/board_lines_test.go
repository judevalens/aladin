package service

import (
	"strings"
	"testing"
)

const boardSnapshotFixture = `{"document":{"store":{
	"shape:t1":{"typeName":"shape","type":"aladin-task","id":"shape:t1","props":{"text":"read §4.2"}},
	"shape:c1":{"typeName":"shape","type":"aladin-card","id":"shape:c1","props":{"front":"What bounds a collar?","back":"The strikes"}},
	"shape:e1":{"typeName":"shape","type":"aladin-excerpt","id":"shape:e1","props":{"text":"The payoff is bounded on both sides.","sourceArtifactId":"a_opt","sourceTitle":"Options","page":94}},
	"shape:l1":{"typeName":"shape","type":"aladin-link","id":"shape:l1","props":{"url":"https://ssrn.com/x","title":"Momentum Crashes","domain":"ssrn.com","description":"Crashes in panic states."}},
	"shape:d1":{"typeName":"shape","type":"aladin-doc","id":"shape:d1","props":{"title":"Option Strategies","artifactId":"a_opt","page":88}},
	"shape:ink1":{"typeName":"shape","type":"text","id":"shape:ink1","props":{"richText":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Greeks — margin notes"}]}]}}},
	"shape:draw1":{"typeName":"shape","type":"draw","id":"shape:draw1","props":{}},
	"binding:x":{"typeName":"binding","type":"arrow","id":"binding:x","props":{}}
}}}`

func TestParseBoardContentRendersEveryLegibleKind(t *testing.T) {
	parsed := ParseBoardContent(boardSnapshotFixture)

	if parsed.Counts["task"] != 1 || parsed.Counts["draw"] != 1 || parsed.Counts["text"] != 1 {
		t.Fatalf("counts wrong: %+v", parsed.Counts)
	}
	byID := map[string]string{}
	for _, line := range parsed.Lines {
		byID[line.ShapeID] = line.Text
	}
	cases := map[string]string{
		"shape:t1":   "task: read §4.2",
		"shape:c1":   "card: What bounds a collar? / The strikes",
		"shape:e1":   "excerpt: The payoff is bounded on both sides. [cite: Options p.94, artifact a_opt]",
		"shape:l1":   "link: Momentum Crashes — https://ssrn.com/x [ssrn.com] :: Crashes in panic states.",
		"shape:d1":   "live window: Option Strategies [artifact a_opt, open at p.88]",
		"shape:ink1": "label: Greeks — margin notes",
	}
	for id, want := range cases {
		if byID[id] != want {
			t.Fatalf("%s:\n got  %q\n want %q", id, byID[id], want)
		}
	}
	// The bare draw stroke is counted but produces no line — strokes aren't legible.
	if _, ok := byID["shape:draw1"]; ok {
		t.Fatal("draw stroke should not produce a line")
	}
}

func TestParseBoardContentToleratesJunk(t *testing.T) {
	for _, content := range []string{"", "   ", "not json", `{"document":{}}`} {
		parsed := ParseBoardContent(content)
		if len(parsed.Counts) != 0 || len(parsed.Lines) != 0 {
			t.Fatalf("junk %q should parse empty, got %+v", content, parsed)
		}
	}
}

func TestFlattenRichTextWalksTolerantly(t *testing.T) {
	got := FlattenRichText([]byte(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"one"},{"type":"text","text":"two"}]},{"type":"unknownNode","content":[{"type":"text","text":"three"}]}]}`))
	if got != "one two three" {
		t.Fatalf("got %q", got)
	}
	if FlattenRichText(nil) != "" || FlattenRichText([]byte("junk")) != "" {
		t.Fatal("junk rich text should flatten to empty")
	}
}

func TestBoardLinesFeedTheSummaryShape(t *testing.T) {
	// The MCP summary sorts lines; make sure nothing in the parser depends on map order.
	first := ParseBoardContent(boardSnapshotFixture)
	second := ParseBoardContent(boardSnapshotFixture)
	if len(first.Lines) != len(second.Lines) {
		t.Fatal("nondeterministic line count")
	}
	for i := range first.Lines {
		if first.Lines[i] != second.Lines[i] {
			t.Fatalf("nondeterministic order at %d", i)
		}
	}
	var joined []string
	for _, l := range first.Lines {
		joined = append(joined, l.Text)
	}
	if !strings.Contains(strings.Join(joined, "\n"), "label: Greeks") {
		t.Fatal("ink labels must reach the joined output")
	}
}
