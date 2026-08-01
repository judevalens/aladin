package ingestion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// segment.go — the seam between Go and the Python layout pass
// (design/INGESTION_PRD.md §13).
//
// Go owns persistence, status and the sync frames. Python is a function: a path in, one
// JSON document out. Deliberately a SUBPROCESS and not a service — ingestion is a batch
// job in a worker, nothing waits on it, so process startup is not on anyone's critical
// path, and a process that must be running is a process that can be forgotten.
//
// This file is the swap point. If the subprocess ever needs to become HTTP, Segmenter is
// the only thing that changes; nothing above it knows the difference.

// Region is one labelled area of a page. Bbox is in PDF POINTS, not pixels — the script
// converts before returning, because mixing coordinate spaces is exactly how a citation
// ends up pointing at the wrong place (§13d).
type Region struct {
	Class      string    `json:"class"`
	Confidence float64   `json:"confidence"`
	Bbox       []float64 `json:"bbox"` // x0, y0, x1, y1
	Text       string    `json:"text"`
}

// LayoutPage is a page's text plus the regions found on it.
type LayoutPage struct {
	Page    int      `json:"page"`
	Width   float64  `json:"width"`
	Height  float64  `json:"height"`
	Text    string   `json:"text"`
	Regions []Region `json:"regions"`
}

// Layout is one document as the script sees it: text, the document's own outline, and
// layout regions — all from a single pass, which is what makes a region's page anchor
// exact rather than inferred across two extractors.
type Layout struct {
	Extractor      string       `json:"extractor"`
	Device         string       `json:"device"`
	PageCount      int          `json:"page_count"`
	PagesProcessed int          `json:"pages_processed"`
	Outline        []Section    `json:"outline"`
	Pages          []LayoutPage `json:"pages"`
}

// Segmenter turns a stored file into a Layout.
type Segmenter interface {
	Segment(ctx context.Context, path string) (Layout, error)
}

// PythonSegmenter runs tools/doclayout/segment.py.
type PythonSegmenter struct {
	Python  string        // interpreter (the tool's venv)
	Script  string        // segment.py
	Timeout time.Duration // per document
}

// DefaultSegmentTimeout bounds one document. A 280-page book measures ~34s on the GPU and
// minutes on CPU, so this is generous — but it must exist: a subprocess that hangs
// silently is the failure the status model (§4) exists to prevent.
const DefaultSegmentTimeout = 20 * time.Minute

// NewPythonSegmenter locates the tool relative to the repo, overridable by env for
// deployments that put it elsewhere.
//
//	ALADIN_DOCLAYOUT_PYTHON  interpreter path
//	ALADIN_DOCLAYOUT_SCRIPT  segment.py path
func NewPythonSegmenter() *PythonSegmenter {
	python := os.Getenv("ALADIN_DOCLAYOUT_PYTHON")
	script := os.Getenv("ALADIN_DOCLAYOUT_SCRIPT")
	if python == "" || script == "" {
		// The worker runs from backend_v2/, so the tool is one level up.
		root := filepath.Join("..", "tools", "doclayout")
		if python == "" {
			python = filepath.Join(root, ".venv", "bin", "python")
		}
		if script == "" {
			script = filepath.Join(root, "segment.py")
		}
	}
	return &PythonSegmenter{Python: python, Script: script, Timeout: DefaultSegmentTimeout}
}

// Available reports whether the tool is installed, so a caller can say "the layout tool
// isn't set up" instead of surfacing an exec error nobody can act on.
func (s *PythonSegmenter) Available() error {
	if _, err := os.Stat(s.Python); err != nil {
		return fmt.Errorf("layout interpreter not found at %s (run: cd tools/doclayout && python3 -m venv .venv && .venv/bin/pip install -r requirements.txt)", s.Python)
	}
	if _, err := os.Stat(s.Script); err != nil {
		return fmt.Errorf("layout script not found at %s", s.Script)
	}
	return nil
}

// Segment runs the script over one document.
//
// Every failure mode lands as an error with a real message: a missing venv, a non-zero
// exit (stderr verbatim), a timeout, or JSON that doesn't parse. The caller maps those to
// status='failed', so a break is always visible and always says why — never a row stuck
// on 'ingesting', which is indistinguishable from a hang.
func (s *PythonSegmenter) Segment(ctx context.Context, path string) (Layout, error) {
	if err := s.Available(); err != nil {
		return Layout{}, err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return Layout{}, fmt.Errorf("resolve %s: %w", path, err)
	}
	if _, err := os.Stat(abs); err != nil {
		return Layout{}, fmt.Errorf("cannot read file: %w", err)
	}

	runCtx, cancel := context.WithTimeout(ctx, s.Timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, s.Python, s.Script, "--pdf", abs)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// WaitDelay is load-bearing, not belt-and-braces. CommandContext kills the process
	// on deadline, but Wait still blocks until the output pipes close — and a grandchild
	// (a shell's child, or a torch worker) keeps them open after its parent dies. Without
	// this, a "timeout" waits for the hung process anyway, which is the exact failure the
	// deadline is here to prevent. Measured: two tests sat for the full 30s until this
	// was added.
	cmd.WaitDelay = 5 * time.Second
	// Put the script in its own process group so the whole tree can be signalled, rather
	// than orphaning children that go on holding the pipes.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			// Negative pid = the group.
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return os.ErrProcessDone
	}

	if err := cmd.Run(); err != nil {
		if runCtx.Err() == context.DeadlineExceeded {
			return Layout{}, fmt.Errorf("layout timed out after %s", s.Timeout)
		}
		// The script's own message is far more useful than "exit status 1".
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return Layout{}, fmt.Errorf("layout failed: %s", lastLine(msg))
		}
		return Layout{}, fmt.Errorf("layout failed: %w", err)
	}

	var layout Layout
	if err := json.Unmarshal(stdout.Bytes(), &layout); err != nil {
		return Layout{}, fmt.Errorf("layout returned unreadable output: %w", err)
	}
	if layout.PageCount == 0 {
		return Layout{}, fmt.Errorf("layout returned no pages")
	}
	return layout, nil
}

// lastLine keeps the final stderr line — a Python traceback's useful part is its end, and
// the whole trace does not belong in a status field a human reads in a tooltip.
func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// ToDocument converts a Layout into the Document the rest of ingestion already speaks, so
// the persistence and status paths need no change (§4's statuses keep their meaning).
func (l Layout) ToDocument() Document {
	doc := Document{
		Status:    StatusReady,
		PageCount: l.PageCount,
		Extractor: l.Extractor,
		Sections:  l.Outline,
	}
	for _, page := range l.Pages {
		doc.Pages = append(doc.Pages, Page{Page: page.Page, Text: normalize(page.Text)})
	}
	// Same scan test as the text-only extractor: pages exist, words don't.
	if doc.TextLen() < minCharsPerPage*max(1, len(doc.Pages)) {
		doc.Status = StatusUnsupported
		doc.Error = "no extractable text layer (likely a scan — needs OCR first)"
	}
	return doc
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
