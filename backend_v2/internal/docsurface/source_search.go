package docsurface

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"aladin/backend_v2/internal/service"
)

const (
	searchFileBytes   = 1 << 20
	searchTotalBytes  = 16 << 20
	searchOutputBytes = 256 << 10
	searchLineBytes   = 2048
	searchMaxFiles    = 1000
	searchMaxEntries  = 10000
)

// GrepFiles searches source text, not filenames or workspace documents. Traversal
// is deterministic and never follows directory symlinks. OpenRoot confines reads
// even if an entry is replaced with a symlink between listing and opening it.
func (l *localStore) GrepFiles(ctx context.Context, pageID string, opts service.SourceSearchOptions) (service.SourceSearchResult, error) {
	out := service.SourceSearchResult{Matches: []service.SourceMatch{}}
	base, err := l.safePath(ctx, pageID, "")
	if err != nil {
		return out, err
	}
	if opts.Pattern == "" || len(opts.Pattern) > 4096 {
		return out, service.BadRequest("pattern must contain between 1 and 4096 bytes")
	}
	if strings.ContainsAny(opts.Pattern, "\r\n") {
		return out, service.BadRequest("grep_files matches one line at a time; pattern must not contain line breaks")
	}
	if opts.ContextLines < 0 || opts.ContextLines > 5 {
		return out, service.BadRequest("context_lines must be between 0 and 5")
	}
	if opts.MaxMatches == 0 {
		opts.MaxMatches = 50
	}
	if opts.MaxMatches < 1 || opts.MaxMatches > 200 {
		return out, service.BadRequest("max_matches must be between 1 and 200")
	}
	glob, err := sourceGlob(opts.Glob)
	if err != nil {
		return out, err
	}
	pattern := opts.Pattern
	if !opts.Regex {
		pattern = regexp.QuoteMeta(pattern)
	}
	if opts.CaseSensitive != nil && !*opts.CaseSensitive {
		pattern = "(?i)" + pattern
	}
	matcher, err := regexp.Compile(pattern)
	if err != nil {
		return out, service.BadRequest("invalid regular expression: " + err.Error())
	}
	root, err := os.OpenRoot(base)
	if err != nil {
		return out, err
	}
	defer root.Close()
	entries, scannedBytes, outputBytes := 0, 0, 0
	stop := func(reason string) error {
		out.Truncated, out.TruncationReason = true, reason
		return filepath.SkipAll
	}
	err = filepath.WalkDir(base, func(full string, d fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		} // do not present an unreadable tree as complete
		if full == base {
			return nil
		}
		entries++
		if entries > searchMaxEntries {
			return stop("entry_limit")
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") || d.Name() == "dist" || d.Name() == "node_modules" || d.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 || strings.HasPrefix(d.Name(), ".") {
			out.FilesSkipped++
			return nil
		}
		rel, err := filepath.Rel(base, full)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !glob(rel) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Size() > searchFileBytes {
			out.FilesSkipped++
			return nil
		}
		if out.FilesSearched+out.FilesSkipped >= searchMaxFiles {
			return stop("file_limit")
		}
		if scannedBytes+int(info.Size()) > searchTotalBytes {
			return stop("byte_limit")
		}
		f, err := root.Open(rel)
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(io.LimitReader(f, searchFileBytes+1))
		closeErr := f.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		scannedBytes += len(data)
		if scannedBytes > searchTotalBytes {
			return stop("byte_limit")
		}
		if len(data) > searchFileBytes || !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
			out.FilesSkipped++
			return nil
		}
		out.FilesSearched++
		if len(data) == 0 {
			return nil
		}
		lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
		for i, line := range lines {
			if err := ctx.Err(); err != nil {
				return err
			}
			line = strings.TrimSuffix(line, "\r")
			if !matcher.MatchString(line) {
				continue
			}
			if len(out.Matches) == opts.MaxMatches {
				return stop("max_matches")
			}
			matched := searchLine(i+1, line)
			hit := service.SourceMatch{Path: rel, Line: matched.Line, Text: matched.Text, Truncated: matched.Truncated}
			for n := max(0, i-opts.ContextLines); n < i; n++ {
				hit.Before = append(hit.Before, searchLine(n+1, lines[n]))
			}
			for n := i + 1; n < min(len(lines), i+1+opts.ContextLines); n++ {
				hit.After = append(hit.After, searchLine(n+1, lines[n]))
			}
			encoded, err := json.Marshal(hit)
			if err != nil {
				return err
			}
			// Includes JSON escaping, not just source bytes, to bound actual output.
			if outputBytes+len(encoded)+1 > searchOutputBytes {
				return stop("output_limit")
			}
			outputBytes += len(encoded) + 1
			out.Matches = append(out.Matches, hit)
		}
		return nil
	})
	return out, err
}

func searchLine(line int, text string) service.SourceLine {
	text = strings.TrimSuffix(text, "\r")
	out := service.SourceLine{Line: line, Text: text}
	if len(text) > searchLineBytes {
		n := searchLineBytes
		for !utf8.RuneStart(text[n]) {
			n--
		}
		out.Text, out.Truncated = text[:n], true
	}
	return out
}

// Globs without a slash match basenames at any depth. A whole ** segment matches
// zero or more directories; other segments use Go path.Match (*, ?, [class]).
func sourceGlob(pattern string) (func(string) bool, error) {
	if pattern == "" {
		return func(string) bool { return true }, nil
	}
	if len(pattern) > 1024 || strings.HasPrefix(pattern, "/") || strings.Contains(pattern, "\\") {
		return nil, service.BadRequest("glob must be a relative slash-separated pattern of at most 1024 bytes")
	}
	parts := strings.Split(pattern, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return nil, service.BadRequest("glob contains an invalid path segment")
		}
		if _, err := path.Match(part, ""); err != nil {
			return nil, service.BadRequest(fmt.Sprintf("invalid glob: %v", err))
		}
	}
	return func(name string) bool {
		if len(parts) == 1 && pattern != "**" {
			matched, _ := path.Match(pattern, path.Base(name))
			return matched
		}
		names := strings.Split(name, "/")
		// Dynamic programming bounds work even for repeated ** segments.
		previous := make([]bool, len(names)+1)
		previous[0] = true
		for _, part := range parts {
			next := make([]bool, len(names)+1)
			if part == "**" {
				next[0] = previous[0]
			}
			for i, name := range names {
				if part == "**" {
					next[i+1] = previous[i+1] || next[i]
				} else {
					matched, _ := path.Match(part, name)
					next[i+1] = previous[i] && matched
				}
			}
			previous = next
		}
		return previous[len(names)]
	}, nil
}
