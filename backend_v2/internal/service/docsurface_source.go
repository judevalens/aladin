package service

import "context"

// DocSurfaceSourceSearch is an optional source-inspection capability. Keeping it
// separate preserves compatibility with other DocSurfaceStore implementations.
type DocSurfaceSourceSearch interface {
	GrepFiles(context.Context, string, SourceSearchOptions) (SourceSearchResult, error)
}

// DocSurfaceFileCAS must serialize comparison and replacement with every other
// file mutation on the same storage namespace, including across processes.
type DocSurfaceFileCAS interface {
	CompareAndSwapFile(ctx context.Context, pageID, path string, previous, next []byte) error
}

type SourceSearchOptions struct {
	Pattern       string
	Regex         bool
	CaseSensitive *bool
	Glob          string
	ContextLines  int
	MaxMatches    int
}

type SourceLine struct {
	Line      int    `json:"line"`
	Text      string `json:"text"`
	Truncated bool   `json:"truncated,omitempty"`
}

type SourceMatch struct {
	Path      string       `json:"path"`
	Line      int          `json:"line"`
	Text      string       `json:"text"`
	Truncated bool         `json:"truncated,omitempty"`
	Before    []SourceLine `json:"before,omitempty"`
	After     []SourceLine `json:"after,omitempty"`
}

type SourceSearchResult struct {
	Matches          []SourceMatch `json:"matches"`
	FilesSearched    int           `json:"files_searched"`
	FilesSkipped     int           `json:"files_skipped"`
	Truncated        bool          `json:"truncated"`
	TruncationReason string        `json:"truncation_reason,omitempty"`
}
