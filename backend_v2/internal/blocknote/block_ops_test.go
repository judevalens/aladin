package blocknote

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func mustParse(t *testing.T, doc string) []Block {
	t.Helper()
	blocks, err := ParseBlocks(json.RawMessage(doc))
	if err != nil {
		t.Fatalf("ParseBlocks(%q): %v", doc, err)
	}
	return blocks
}

func mustSerialize(t *testing.T, blocks []Block) string {
	t.Helper()
	out, err := SerializeBlocks(blocks)
	if err != nil {
		t.Fatalf("SerializeBlocks: %v", err)
	}
	return string(out)
}

func TestParseBlocks_Empty(t *testing.T) {
	cases := []string{``, `[]`}
	for _, c := range cases {
		got, err := ParseBlocks(json.RawMessage(c))
		if err != nil {
			t.Fatalf("ParseBlocks(%q): %v", c, err)
		}
		if len(got) != 0 {
			t.Fatalf("ParseBlocks(%q) = %#v, want empty", c, got)
		}
	}
}

func TestParseBlocks_PreservesRawJSON(t *testing.T) {
	doc := `[{"id":"a","type":"paragraph","props":{"x":1},"content":[],"children":[]}]`
	blocks := mustParse(t, doc)
	if len(blocks) != 1 || blocks[0].ID != "a" {
		t.Fatalf("blocks = %#v", blocks)
	}
	round := mustSerialize(t, blocks)
	if round != doc {
		t.Fatalf("round-trip changed JSON: got %s, want %s", round, doc)
	}
}

func TestFindByID(t *testing.T) {
	blocks := mustParse(t, `[
		{"id":"a","type":"p"},
		{"id":"b","type":"p"},
		{"id":"c","type":"p"}
	]`)
	if FindByID(blocks, "b") != 1 {
		t.Fatalf("FindByID b = %d, want 1", FindByID(blocks, "b"))
	}
	if FindByID(blocks, "missing") != -1 {
		t.Fatalf("FindByID missing = %d, want -1", FindByID(blocks, "missing"))
	}
}

func TestReplaceByID_SingleBlock_KeepsID(t *testing.T) {
	blocks := mustParse(t, `[{"id":"a","type":"p","content":[]},{"id":"b","type":"p","content":[]}]`)
	replacement := mustParse(t, `[{"id":"new","type":"heading","content":[]}]`)
	out, n, err := ReplaceByID(blocks, "a", replacement)
	if err != nil {
		t.Fatalf("ReplaceByID: %v", err)
	}
	if n != 1 {
		t.Fatalf("count = %d, want 1", n)
	}
	if out[0].ID != "a" {
		t.Fatalf("first block id = %q, want %q (original id preserved)", out[0].ID, "a")
	}
	if !strings.Contains(string(out[0].Raw), `"type":"heading"`) {
		t.Fatalf("replacement raw = %s, want heading", string(out[0].Raw))
	}
	if out[1].ID != "b" {
		t.Fatalf("second block id = %q, want b", out[1].ID)
	}
}

func TestReplaceByID_MultipleBlocks_FirstKeepsIDOthersKeepTheirs(t *testing.T) {
	blocks := mustParse(t, `[{"id":"target","type":"p"}]`)
	replacement := mustParse(t, `[
		{"id":"new1","type":"heading"},
		{"id":"new2","type":"p"},
		{"id":"new3","type":"p"}
	]`)
	out, n, err := ReplaceByID(blocks, "target", replacement)
	if err != nil {
		t.Fatalf("ReplaceByID: %v", err)
	}
	if n != 3 {
		t.Fatalf("count = %d, want 3", n)
	}
	if out[0].ID != "target" {
		t.Fatalf("first new block id = %q, want %q", out[0].ID, "target")
	}
	if out[1].ID != "new2" || out[2].ID != "new3" {
		t.Fatalf("subsequent ids = %q,%q, want new2,new3", out[1].ID, out[2].ID)
	}
}

func TestReplaceByID_NotFound(t *testing.T) {
	blocks := mustParse(t, `[{"id":"a","type":"p"}]`)
	_, _, err := ReplaceByID(blocks, "missing", mustParse(t, `[{"id":"x","type":"p"}]`))
	if !errors.Is(err, ErrBlockNotFound) {
		t.Fatalf("error = %v, want ErrBlockNotFound", err)
	}
}

func TestReplaceByID_EmptyReplacementDeletes(t *testing.T) {
	blocks := mustParse(t, `[{"id":"a","type":"p"},{"id":"b","type":"p"}]`)
	out, n, err := ReplaceByID(blocks, "a", nil)
	if err != nil {
		t.Fatalf("ReplaceByID: %v", err)
	}
	if n != 0 {
		t.Fatalf("count = %d, want 0", n)
	}
	if len(out) != 1 || out[0].ID != "b" {
		t.Fatalf("out = %#v, want only b", out)
	}
}

func TestInsertAfter(t *testing.T) {
	blocks := mustParse(t, `[{"id":"a","type":"p"},{"id":"b","type":"p"}]`)
	newBlocks := mustParse(t, `[{"id":"x","type":"p"}]`)
	out, err := InsertAfter(blocks, "a", newBlocks)
	if err != nil {
		t.Fatalf("InsertAfter: %v", err)
	}
	ids := CollectIDs(out)
	want := []string{"a", "x", "b"}
	if !equalSlices(ids, want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
}

func TestInsertAfter_NotFound(t *testing.T) {
	blocks := mustParse(t, `[{"id":"a","type":"p"}]`)
	_, err := InsertAfter(blocks, "missing", mustParse(t, `[{"id":"x","type":"p"}]`))
	if !errors.Is(err, ErrBlockNotFound) {
		t.Fatalf("error = %v, want ErrBlockNotFound", err)
	}
}

func TestInsertBefore(t *testing.T) {
	blocks := mustParse(t, `[{"id":"a","type":"p"},{"id":"b","type":"p"}]`)
	out, err := InsertBefore(blocks, "b", mustParse(t, `[{"id":"x","type":"p"}]`))
	if err != nil {
		t.Fatalf("InsertBefore: %v", err)
	}
	if !equalSlices(CollectIDs(out), []string{"a", "x", "b"}) {
		t.Fatalf("ids = %v", CollectIDs(out))
	}
}

func TestInsertAtStart(t *testing.T) {
	blocks := mustParse(t, `[{"id":"a","type":"p"}]`)
	out := InsertAtStart(blocks, mustParse(t, `[{"id":"x","type":"p"},{"id":"y","type":"p"}]`))
	if !equalSlices(CollectIDs(out), []string{"x", "y", "a"}) {
		t.Fatalf("ids = %v", CollectIDs(out))
	}
}

func TestInsertAtStart_EmptyDoc(t *testing.T) {
	out := InsertAtStart(nil, mustParse(t, `[{"id":"x","type":"p"}]`))
	if !equalSlices(CollectIDs(out), []string{"x"}) {
		t.Fatalf("ids = %v", CollectIDs(out))
	}
}

func TestInsertAtEnd(t *testing.T) {
	blocks := mustParse(t, `[{"id":"a","type":"p"}]`)
	out := InsertAtEnd(blocks, mustParse(t, `[{"id":"x","type":"p"},{"id":"y","type":"p"}]`))
	if !equalSlices(CollectIDs(out), []string{"a", "x", "y"}) {
		t.Fatalf("ids = %v", CollectIDs(out))
	}
}

func TestDeleteByID(t *testing.T) {
	blocks := mustParse(t, `[{"id":"a","type":"p"},{"id":"b","type":"p"},{"id":"c","type":"p"}]`)
	out, err := DeleteByID(blocks, "b")
	if err != nil {
		t.Fatalf("DeleteByID: %v", err)
	}
	if !equalSlices(CollectIDs(out), []string{"a", "c"}) {
		t.Fatalf("ids = %v", CollectIDs(out))
	}
}

func TestDeleteByID_NotFound(t *testing.T) {
	blocks := mustParse(t, `[{"id":"a","type":"p"}]`)
	_, err := DeleteByID(blocks, "missing")
	if !errors.Is(err, ErrBlockNotFound) {
		t.Fatalf("error = %v, want ErrBlockNotFound", err)
	}
}

func TestWithID_OverwritesID(t *testing.T) {
	blocks := mustParse(t, `[{"id":"old","type":"p","props":{"x":1}}]`)
	updated, err := WithID(blocks[0], "new-id")
	if err != nil {
		t.Fatalf("WithID: %v", err)
	}
	if updated.ID != "new-id" {
		t.Fatalf("ID = %q, want new-id", updated.ID)
	}
	if !strings.Contains(string(updated.Raw), `"new-id"`) {
		t.Fatalf("raw = %s, want new-id embedded", string(updated.Raw))
	}
	if !strings.Contains(string(updated.Raw), `"props":{"x":1}`) {
		t.Fatalf("raw = %s, want props preserved", string(updated.Raw))
	}
}

func equalSlices[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
