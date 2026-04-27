package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

type fixtureSpec struct {
	Platform string           `json:"platform"`
	Pages    []redditPageSpec `json:"pages"`
}

type redditPageSpec struct {
	Cursor     string             `json:"cursor"`
	NextCursor string             `json:"next_cursor"`
	Children   []redditRecordSpec `json:"children"`
}

type redditRecordSpec struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Permalink  string  `json:"permalink"`
	Score      int     `json:"score"`
	CreatedUTC float64 `json:"created_utc"`
}

type redditFixture struct {
	Pages map[string]redditFixturePage `json:"pages"`
}

type redditFixturePage struct {
	After    string             `json:"after"`
	Children []redditRecordSpec `json:"children"`
}

func main() {
	specPath := flag.String("spec", "", "path to fixture spec JSON")
	outPath := flag.String("out", "", "path to generated fixture JSON")
	flag.Parse()

	if *specPath == "" || *outPath == "" {
		exitf("usage: sync-fixture-gen -spec <spec.json> -out <fixture.json>")
	}

	raw, err := os.ReadFile(*specPath)
	if err != nil {
		exitf("read spec: %v", err)
	}

	var spec fixtureSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		exitf("decode spec: %v", err)
	}

	fixture, err := buildFixture(spec)
	if err != nil {
		exitf("build fixture: %v", err)
	}

	out, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		exitf("encode fixture: %v", err)
	}
	out = append(out, '\n')

	if err := os.WriteFile(*outPath, out, 0o644); err != nil {
		exitf("write fixture: %v", err)
	}
}

func buildFixture(spec fixtureSpec) (*redditFixture, error) {
	if spec.Platform != "reddit" {
		return nil, fmt.Errorf("unsupported platform %q", spec.Platform)
	}

	fixture := &redditFixture{
		Pages: make(map[string]redditFixturePage, len(spec.Pages)),
	}
	for i, page := range spec.Pages {
		if page.Cursor == "" && i != 0 {
			return nil, fmt.Errorf("page %d: empty cursor is only allowed for the first page", i)
		}
		if _, exists := fixture.Pages[page.Cursor]; exists {
			return nil, fmt.Errorf("duplicate cursor %q", page.Cursor)
		}
		fixture.Pages[page.Cursor] = redditFixturePage{
			After:    page.NextCursor,
			Children: append([]redditRecordSpec(nil), page.Children...),
		}
	}
	return fixture, nil
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
