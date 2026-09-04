// Package docsurface implements the Doc Surface ("app" artifact) file store and
// esbuild build runtime. v1 runs against the local filesystem ("backend is the
// central container"); the same interfaces (service.DocSurfaceStore /
// service.WorkspaceRuntime) admit a Docker-per-user impl later.
package docsurface

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"aladin/backend_v2/internal/service"
)

const (
	maxFileBytes   = 10 << 20 // 10 MiB per file
	installLibName = "install_lib.json"
	distDirName    = "dist"
)

// localStore stores pages under {root}/users/{userId}/pages/{pageId}/.
type localStore struct{ root string }

// NewStore returns a filesystem-backed DocSurfaceStore rooted at root.
func NewStore(root string) service.DocSurfaceStore { return &localStore{root: root} }

// validateSegment rejects any single path component that could traverse or
// escape the page directory.
func validateSegment(s string) error {
	if s == "" || s == "." || s == ".." ||
		strings.ContainsRune(s, '/') || strings.ContainsRune(s, '\\') || strings.ContainsRune(s, 0) {
		return service.BadRequest(fmt.Sprintf("invalid path segment %q", s))
	}
	return nil
}

// safePath resolves (userID-from-ctx, pageID, rel) to an absolute path that is
// provably inside the page directory. rel may contain subdirs but cannot escape.
func (l *localStore) safePath(ctx context.Context, pageID, rel string) (string, error) {
	p, err := service.RequirePrincipal(ctx)
	if err != nil {
		return "", err
	}
	userID := strings.TrimSpace(p.UserID)
	if err := validateSegment(userID); err != nil {
		return "", err
	}
	if err := validateSegment(strings.TrimSpace(pageID)); err != nil {
		return "", err
	}
	base := filepath.Join(l.root, "users", userID, "pages", strings.TrimSpace(pageID))
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return absBase, nil
	}
	// Reject explicit traversal / absolute paths outright (clear error for the
	// agent), then keep the prefix check below as defense-in-depth.
	cleanRel := filepath.Clean(rel)
	sep := string(filepath.Separator)
	if filepath.IsAbs(cleanRel) || cleanRel == ".." || strings.HasPrefix(cleanRel, ".."+sep) {
		return "", service.BadRequest("path traversal rejected")
	}
	full := filepath.Join(absBase, cleanRel)
	absFull, err := filepath.Abs(full)
	if err != nil {
		return "", err
	}
	if absFull != absBase && !strings.HasPrefix(absFull, absBase+sep) {
		return "", service.BadRequest("path traversal rejected")
	}
	// Best-effort symlink guard: if the target already exists, ensure its real
	// (symlink-resolved) location is still inside the page dir. Defends against a
	// symlink planted by something outside the agent's MCP tools.
	if realFull, err := filepath.EvalSymlinks(absFull); err == nil {
		if realBase, err := filepath.EvalSymlinks(absBase); err == nil {
			if realFull != realBase && !strings.HasPrefix(realFull, realBase+sep) {
				return "", service.BadRequest("path traversal rejected")
			}
		}
	}
	return absFull, nil
}

func (l *localStore) EnsurePageDir(ctx context.Context, pageID string) (string, error) {
	dir, err := l.safePath(ctx, pageID, "")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func (l *localStore) PageDir(ctx context.Context, pageID string) (string, error) {
	return l.safePath(ctx, pageID, "")
}

func (l *localStore) ListDir(ctx context.Context, pageID, relPath string) ([]service.FileEntry, error) {
	dir, err := l.safePath(ctx, pageID, relPath)
	if err != nil {
		return nil, err
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []service.FileEntry{}, nil
		}
		return nil, err
	}
	out := make([]service.FileEntry, 0, len(ents))
	for _, e := range ents {
		fe := service.FileEntry{Name: e.Name(), IsDir: e.IsDir()}
		if info, err := e.Info(); err == nil && !e.IsDir() {
			fe.Size = info.Size()
		}
		out = append(out, fe)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (l *localStore) ReadFile(ctx context.Context, pageID, relPath string) ([]byte, error) {
	p, err := l.safePath(ctx, pageID, relPath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, service.ErrNotFound
		}
		return nil, err
	}
	if info.IsDir() {
		return nil, service.BadRequest("path is a directory")
	}
	if info.Size() > maxFileBytes {
		return nil, service.BadRequest("file too large")
	}
	return os.ReadFile(p)
}

func (l *localStore) WriteFile(ctx context.Context, pageID, relPath string, data []byte) error {
	if _, err := l.safePath(ctx, pageID, relPath); err != nil {
		return err
	}
	unlock, err := l.lockMutation(ctx, pageID)
	if err != nil {
		return err
	}
	defer unlock()
	return l.writeFile(ctx, pageID, relPath, data)
}

func (l *localStore) writeFile(ctx context.Context, pageID, relPath string, data []byte) error {
	if len(data) > maxFileBytes {
		return service.BadRequest("file too large")
	}
	p, err := l.safePath(ctx, pageID, relPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return atomicWrite(p, data)
}

func (l *localStore) DeleteFile(ctx context.Context, pageID, relPath string) error {
	p, err := l.safePath(ctx, pageID, relPath)
	if err != nil {
		return err
	}
	unlock, err := l.lockMutation(ctx, pageID)
	if err != nil {
		return err
	}
	defer unlock()
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (l *localStore) SearchFiles(ctx context.Context, pageID, query string, limit int) ([]string, error) {
	base, err := l.safePath(ctx, pageID, "")
	if err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, service.BadRequest("query is required")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var hits []string
	err = filepath.WalkDir(base, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			if d.Name() == distDirName || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(base, path)
		matched := strings.Contains(rel, query)
		if !matched {
			if info, err := d.Info(); err == nil && info.Size() <= maxFileBytes {
				if b, err := os.ReadFile(path); err == nil && isTextLike(b) && strings.Contains(string(b), query) {
					matched = true
				}
			}
		}
		if matched {
			hits = append(hits, rel)
			if len(hits) >= limit {
				return filepath.SkipAll
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if hits == nil {
		hits = []string{}
	}
	return hits, nil
}

type libManifest struct {
	Libs []service.LibEntry `json:"libs"`
}

func (l *localStore) Libs(ctx context.Context, pageID string) ([]service.LibEntry, error) {
	p, err := l.safePath(ctx, pageID, installLibName)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []service.LibEntry{}, nil
		}
		return nil, err
	}
	var m libManifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, service.BadRequest("install_lib.json is corrupt: " + err.Error())
	}
	if m.Libs == nil {
		m.Libs = []service.LibEntry{}
	}
	return m.Libs, nil
}

func (l *localStore) InstallLib(ctx context.Context, pageID string, lib service.LibEntry) ([]service.LibEntry, error) {
	lib.Name = strings.TrimSpace(lib.Name)
	lib.URL = strings.TrimSpace(lib.URL)
	if lib.Name == "" || lib.URL == "" {
		return nil, service.BadRequest("name and url are required")
	}
	libs, err := l.Libs(ctx, pageID)
	if err != nil {
		return nil, err
	}
	replaced := false
	for i := range libs {
		if libs[i].Name == lib.Name {
			libs[i] = lib
			replaced = true
			break
		}
	}
	if !replaced {
		libs = append(libs, lib)
	}
	out, err := json.MarshalIndent(libManifest{Libs: libs}, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := l.WriteFile(ctx, pageID, installLibName, out); err != nil {
		return nil, err
	}
	return libs, nil
}

// atomicWrite writes data to a temp file in the same dir then renames over the
// target, so a reader never sees a partial file.
func atomicWrite(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".write-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := f.Chmod(0o644); err != nil {
		_ = f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

// isTextLike reports whether b looks like text (no NUL in the first 8KB).
func isTextLike(b []byte) bool {
	n := len(b)
	if n > 8192 {
		n = 8192
	}
	for i := 0; i < n; i++ {
		if b[i] == 0 {
			return false
		}
	}
	return true
}
