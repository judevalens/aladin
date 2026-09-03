package board

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Board snapshot → addressable text lines. ONE parser for the two consumers that must
// never drift: the MCP board summary (what a copilot turn reads) and the content-index
// projector (what search retrieves). Every line carries its shape id, so a hit can point
// back onto the board ("shape:<id>" locator).

// BoardLine is one legible object on a board.
type BoardLine struct {
	ShapeID string
	// Kind is the shape type without the "aladin-" prefix ("task", "card", "excerpt",
	// "link", "doc", "ink-label").
	Kind string
	// Text is the rendered line — content plus its cite, when the object carries one.
	Text string
}

// BoardContent is the parsed, legible view of a board snapshot.
type BoardContent struct {
	// Counts by rendered kind (every shape counts, legible or not).
	Counts map[string]int
	Lines  []BoardLine
}

type boardRecord struct {
	TypeName string `json:"typeName"`
	Type     string `json:"type"`
	ID       string `json:"id"`
	Props    struct {
		Text             string          `json:"text"`
		Front            string          `json:"front"`
		Back             string          `json:"back"`
		Title            string          `json:"title"`
		ArtifactID       string          `json:"artifactId"`
		SourceArtifactID string          `json:"sourceArtifactId"`
		SourceTitle      string          `json:"sourceTitle"`
		Page             int             `json:"page"`
		URL              string          `json:"url"`
		Description      string          `json:"description"`
		Domain           string          `json:"domain"`
		RichText         json.RawMessage `json:"richText"`
	} `json:"props"`
}

// ParseContent reads a board's projected snapshot (artifacts.content). An empty or
// unreadable snapshot returns zero counts — callers decide how to say "empty".
func ParseContent(content string) BoardContent {
	out := BoardContent{Counts: map[string]int{}}
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return out
	}
	var snapshot struct {
		Document struct {
			Store map[string]boardRecord `json:"store"`
		} `json:"document"`
	}
	if err := json.Unmarshal([]byte(trimmed), &snapshot); err != nil {
		return out
	}
	// Store maps are unordered — sort by record key so output is deterministic.
	keys := make([]string, 0, len(snapshot.Document.Store))
	for key := range snapshot.Document.Store {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		record := snapshot.Document.Store[key]
		if record.TypeName != "shape" {
			continue
		}
		out.Counts[strings.TrimPrefix(record.Type, "aladin-")]++
		if line, ok := boardLineFor(record); ok {
			out.Lines = append(out.Lines, line)
		}
	}
	return out
}

func boardLineFor(record boardRecord) (BoardLine, bool) {
	p := record.Props
	switch record.Type {
	case "aladin-task":
		if p.Text == "" {
			return BoardLine{}, false
		}
		return BoardLine{ShapeID: record.ID, Kind: "task", Text: "task: " + p.Text}, true
	case "aladin-card":
		if p.Front == "" {
			return BoardLine{}, false
		}
		return BoardLine{ShapeID: record.ID, Kind: "card", Text: "card: " + p.Front + " / " + p.Back}, true
	case "aladin-excerpt":
		if p.Text == "" {
			return BoardLine{}, false
		}
		line := "excerpt: " + p.Text
		// The cite is what makes the excerpt quizzable: the copilot reads the source
		// around it (read_document) and checks answers against the text.
		if p.SourceArtifactID != "" && p.Page > 0 {
			line += fmt.Sprintf(" [cite: %s p.%d, artifact %s]", p.SourceTitle, p.Page, p.SourceArtifactID)
		}
		return BoardLine{ShapeID: record.ID, Kind: "excerpt", Text: line}, true
	case "aladin-link":
		if p.URL == "" {
			return BoardLine{}, false
		}
		// Gathered sources are what lets an agent draft a study/research plan from a
		// board — surface everything the unfurl learned, not just the URL.
		line := "link: " + p.Title
		if p.Title == "" {
			line = "link: " + p.URL
		} else {
			line += " — " + p.URL
		}
		if p.Domain != "" {
			line += " [" + p.Domain + "]"
		}
		if p.Description != "" {
			line += " :: " + p.Description
		}
		return BoardLine{ShapeID: record.ID, Kind: "link", Text: line}, true
	case "aladin-doc":
		if p.Title == "" {
			return BoardLine{}, false
		}
		line := "live window: " + p.Title
		if p.ArtifactID != "" {
			line += fmt.Sprintf(" [artifact %s, open at p.%d]", p.ArtifactID, p.Page)
		}
		return BoardLine{ShapeID: record.ID, Kind: "doc", Text: line}, true
	case "text":
		// Ink labels — handwriting-adjacent text the user placed as headings/margins.
		// "Your handwriting is the legend": these name the board's regions, so a board
		// without them reads as unstructured when it isn't.
		label := FlattenRichText(p.RichText)
		if label == "" {
			return BoardLine{}, false
		}
		return BoardLine{ShapeID: record.ID, Kind: "ink-label", Text: "label: " + label}, true
	default:
		return BoardLine{}, false
	}
}

// FlattenRichText walks tldraw's tiptap document tolerantly, collecting text nodes —
// unknown node shapes just contribute nothing (the same stance as the client's
// flattenBlocks).
func FlattenRichText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var node any
	if err := json.Unmarshal(raw, &node); err != nil {
		return ""
	}
	var out []string
	var walk func(any)
	walk = func(n any) {
		switch v := n.(type) {
		case []any:
			for _, item := range v {
				walk(item)
			}
		case map[string]any:
			if text, ok := v["text"].(string); ok && text != "" {
				out = append(out, text)
			}
			if content, ok := v["content"]; ok {
				walk(content)
			}
		}
	}
	walk(node)
	return strings.Join(strings.Fields(strings.Join(out, " ")), " ")
}
