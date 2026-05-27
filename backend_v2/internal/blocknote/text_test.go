package blocknote

import (
	"encoding/json"
	"testing"
)

func TestExtractText(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty input",
			in:   ``,
			want: "",
		},
		{
			name: "empty array",
			in:   `[]`,
			want: "",
		},
		{
			name: "single paragraph",
			in: `[{
				"id":"a","type":"paragraph",
				"content":[{"type":"text","text":"Hello world","styles":{}}],
				"children":[]
			}]`,
			want: "Hello world",
		},
		{
			name: "heading + list",
			in: `[
				{"id":"h","type":"heading","props":{"level":1},
				 "content":[{"type":"text","text":"Title","styles":{}}],"children":[]},
				{"id":"l1","type":"bulletListItem",
				 "content":[{"type":"text","text":"first","styles":{}}],"children":[]},
				{"id":"l2","type":"bulletListItem",
				 "content":[{"type":"text","text":"second","styles":{}}],"children":[]}
			]`,
			want: "Title\nfirst\nsecond",
		},
		{
			name: "multiple text runs in one block (e.g. bold mid-sentence)",
			in: `[{
				"id":"a","type":"paragraph",
				"content":[
					{"type":"text","text":"Hello ","styles":{}},
					{"type":"text","text":"bold","styles":{"bold":true}},
					{"type":"text","text":" world","styles":{}}
				],
				"children":[]
			}]`,
			want: "Hello bold world",
		},
		{
			name: "nested children (toggle list)",
			in: `[{
				"id":"a","type":"toggleListItem",
				"content":[{"type":"text","text":"Parent","styles":{}}],
				"children":[
					{"id":"b","type":"paragraph",
					 "content":[{"type":"text","text":"Child","styles":{}}],
					 "children":[]}
				]
			}]`,
			want: "Parent\nChild",
		},
		{
			name: "block with no inline content (image-like)",
			in: `[
				{"id":"img","type":"image","props":{"url":"x.png"},"content":[],"children":[]},
				{"id":"p","type":"paragraph",
				 "content":[{"type":"text","text":"after","styles":{}}],"children":[]}
			]`,
			want: "\nafter",
		},
		{
			name: "block with null content",
			in: `[
				{"id":"img","type":"image","props":{"url":"x.png"},"content":null,"children":[]},
				{"id":"p","type":"paragraph",
				 "content":[{"type":"text","text":"after","styles":{}}],"children":[]}
			]`,
			want: "\nafter",
		},
		{
			name: "string content shape",
			in: `[{
				"id":"c","type":"codeBlock","props":{"language":"go"},
				"content":"package main",
				"children":[]
			}]`,
			want: "package main",
		},
		{
			name: "non-text inline (link with text node mixed)",
			in: `[{
				"id":"a","type":"paragraph",
				"content":[
					{"type":"text","text":"see ","styles":{}},
					{"type":"link","href":"https://x","content":[{"type":"text","text":"here","styles":{}}]}
				],
				"children":[]
			}]`,
			// link inline content is not unwrapped — we only collect the top-level
			// `text` field. That's acceptable; the link's anchor text is lost from
			// the search index. Document the constraint in the test name.
			want: "see ",
		},
		{
			name: "unknown block type with text content is still picked up",
			in: `[{
				"id":"x","type":"customMystery",
				"content":[{"type":"text","text":"mystery","styles":{}}],
				"children":[]
			}]`,
			want: "mystery",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var raw json.RawMessage
			if tc.in != "" {
				raw = json.RawMessage(tc.in)
			}
			got, err := ExtractText(raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestExtractText_InvalidJSON(t *testing.T) {
	_, err := ExtractText(json.RawMessage(`{"not":"an array"}`))
	if err == nil {
		t.Fatalf("expected error for non-array root, got nil")
	}
}
