package authoring

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"aladin/backend_v2/internal/service"
)

type ReadCommand struct {
	PageID    string
	Path      string
	StartLine *int
	EndLine   *int
}

type ReadResult struct {
	Content    string `json:"content"`
	Hash       string `json:"hash"`
	TotalLines int    `json:"total_lines"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
}

func contentHash(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

// ReadRange preserves the exact bytes, including CRLF and trailing newlines.
// A terminal newline terminates a line; it does not create a phantom extra line.
func (a *Authoring) ReadRange(ctx context.Context, cmd ReadCommand) (ReadResult, error) {
	data, err := a.ReadFile(ctx, cmd.PageID, cmd.Path)
	if err != nil {
		return ReadResult{}, err
	}
	if (cmd.StartLine != nil && *cmd.StartLine < 1) || (cmd.EndLine != nil && *cmd.EndLine < 1) {
		return ReadResult{}, service.BadRequest("line numbers must be positive (one-based, inclusive)")
	}
	start := 1
	if cmd.StartLine != nil {
		start = *cmd.StartLine
	}
	if cmd.EndLine != nil && *cmd.EndLine < start {
		return ReadResult{}, service.BadRequest("end_line must be greater than or equal to start_line")
	}
	content := string(data)
	total := strings.Count(content, "\n")
	if len(data) > 0 && data[len(data)-1] != '\n' {
		total++
	}
	out := ReadResult{Content: content, Hash: contentHash(data), TotalLines: total}
	if cmd.StartLine == nil && cmd.EndLine == nil {
		if total > 0 {
			out.StartLine, out.EndLine = 1, total
		}
		return out, nil
	}
	if total == 0 || start > total {
		return ReadResult{}, service.BadRequest("start_line is beyond the end of the file")
	}
	end := total
	if cmd.EndLine != nil && *cmd.EndLine < end {
		end = *cmd.EndLine
	}
	// Scan offsets rather than allocate one string entry per line for large files.
	line, from, to := 1, 0, len(content)
	for i := 0; i < len(content); i++ {
		if content[i] != '\n' {
			continue
		}
		if line < start {
			from = i + 1
		}
		if line == end {
			to = i + 1
			break
		}
		line++
	}
	out.Content, out.StartLine, out.EndLine = content[from:to], start, end
	return out, nil
}

func (a *Authoring) GrepFiles(ctx context.Context, pageID string, opts service.SourceSearchOptions) (service.SourceSearchResult, error) {
	if err := a.RequireApp(ctx, pageID); err != nil {
		return service.SourceSearchResult{}, err
	}
	store, ok := a.store.(service.DocSurfaceSourceSearch)
	if !ok {
		return service.SourceSearchResult{}, service.BadRequest("source search is unavailable on this store")
	}
	return store.GrepFiles(ctx, pageID, opts)
}
