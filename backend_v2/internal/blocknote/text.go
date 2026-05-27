// Package blocknote provides utilities for working with BlockNote editor
// documents stored as JSON. It does not depend on any JavaScript runtime;
// callers that need true markdown<->blocks conversion talk to the
// blocknote-converter Node sidecar over HTTP.
package blocknote

import (
	"encoding/json"
	"strings"
)

// ExtractText walks a BlockNote document (a JSON array of blocks) and returns
// all inline text content concatenated. Blocks are separated by newlines so
// that the result is suitable for full-text indexing. Unknown block types and
// blocks without inline content (e.g. images) are skipped silently.
//
// The function is tolerant of partial/malformed input: any block whose shape
// it does not understand is skipped rather than failing the whole call.
// It returns an error only when the top-level JSON is itself not a valid
// array.
func ExtractText(blocks json.RawMessage) (string, error) {
	if len(blocks) == 0 {
		return "", nil
	}
	var nodes []blockNode
	if err := json.Unmarshal(blocks, &nodes); err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, n := range nodes {
		writeBlockText(&sb, n)
	}
	return strings.TrimRight(sb.String(), "\n"), nil
}

type blockNode struct {
	Type     string          `json:"type"`
	Content  json.RawMessage `json:"content"`
	Children []blockNode     `json:"children"`
}

type inlineNode struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func writeBlockText(sb *strings.Builder, n blockNode) {
	writeInlineContent(sb, n.Content)
	sb.WriteByte('\n')
	for _, child := range n.Children {
		writeBlockText(sb, child)
	}
}

// writeInlineContent handles BlockNote's inline content shape, which can be
// either a JSON array (the common case for paragraph/heading/list-item) or
// a string (some custom block types). Anything else is ignored.
func writeInlineContent(sb *strings.Builder, raw json.RawMessage) {
	if len(raw) == 0 {
		return
	}
	switch raw[0] {
	case '[':
		var items []inlineNode
		if err := json.Unmarshal(raw, &items); err != nil {
			return
		}
		for _, item := range items {
			if item.Text != "" {
				sb.WriteString(item.Text)
			}
		}
	case '"':
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return
		}
		sb.WriteString(s)
	}
}
