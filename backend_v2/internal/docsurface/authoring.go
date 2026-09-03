package docsurface

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"aladin/backend_v2/internal/service"
)

const (
	appArtifactType = "app"
	HistoryDir      = ".history"
	HistoryKeep     = 20
)

var (
	ErrEditNotFound  = errors.New("old_string not found")
	ErrEditAmbiguous = errors.New("old_string ambiguous")
)

// Authoring is the application boundary for page-scoped Doc Surface file and
// build commands. It owns authorization-before-IO, recoverable file mutation,
// and the draft-build policy; MCP and HTTP adapters only map transport shapes.
type Authoring struct {
	artifacts service.ArtifactService
	store     service.DocSurfaceStore
	build     service.ShardBuildService
}

type CreateCommand struct {
	Artifact service.ArtifactPayload
	Files    map[string][]byte
}

func (a *Authoring) Create(ctx context.Context, cmd CreateCommand) (service.ArtifactResponse, error) {
	if strings.TrimSpace(cmd.Artifact.Title) == "" {
		return service.ArtifactResponse{}, service.BadRequest("title is required")
	}
	cmd.Artifact.Type = appArtifactType
	created, err := a.artifacts.Create(ctx, cmd.Artifact)
	if err != nil {
		return service.ArtifactResponse{}, err
	}
	id := created.Artifact.ID
	rollback := func() { _, _ = a.artifacts.Delete(ctx, id) }
	if _, err := a.store.EnsurePageDir(ctx, id); err != nil {
		rollback()
		return service.ArtifactResponse{}, err
	}
	names := make([]string, 0, len(cmd.Files))
	for name := range cmd.Files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := a.store.WriteFile(ctx, id, name, cmd.Files[name]); err != nil {
			rollback()
			return service.ArtifactResponse{}, err
		}
	}
	return created.Artifact, nil
}

func NewAuthoring(artifacts service.ArtifactService, store service.DocSurfaceStore, build service.ShardBuildService) *Authoring {
	return &Authoring{artifacts: artifacts, store: store, build: build}
}

type WriteCommand struct {
	PageID    string
	Path      string
	Content   string
	Build     *bool
	Overwrite bool
}

type WriteResult struct {
	OK    bool
	Path  string
	Build *service.BuildResult
}

type EditCommand struct {
	PageID     string
	Path       string
	OldString  string
	NewString  string
	ReplaceAll bool
	Build      *bool
}

type EditResult struct {
	OK           bool
	Path         string
	Replacements int
	Build        *service.BuildResult
}

type DeleteCommand struct {
	PageID string
	Path   string
	Build  *bool
}

type DeleteResult struct {
	OK      bool
	Deleted string
	Build   *service.BuildResult
}

func (a *Authoring) RequireApp(ctx context.Context, pageID string) error {
	if strings.TrimSpace(pageID) == "" {
		return service.BadRequest("page_id is required")
	}
	rec, err := a.artifacts.Get(ctx, pageID)
	if err != nil {
		return err
	}
	if rec.Type != appArtifactType {
		return service.BadRequest("artifact is not a Doc Surface page")
	}
	return nil
}

func (a *Authoring) ListDir(ctx context.Context, pageID, path string) ([]service.FileEntry, error) {
	if err := a.RequireApp(ctx, pageID); err != nil {
		return nil, err
	}
	return a.store.ListDir(ctx, pageID, path)
}

func (a *Authoring) ReadFile(ctx context.Context, pageID, path string) ([]byte, error) {
	if err := a.RequireApp(ctx, pageID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(path) == "" {
		return nil, service.BadRequest("path is required")
	}
	return a.store.ReadFile(ctx, pageID, path)
}

func (a *Authoring) WriteFile(ctx context.Context, cmd WriteCommand) (WriteResult, error) {
	if err := a.RequireApp(ctx, cmd.PageID); err != nil {
		return WriteResult{}, err
	}
	if strings.TrimSpace(cmd.Path) == "" {
		return WriteResult{}, service.BadRequest("path is required")
	}
	if existing, err := a.store.ReadFile(ctx, cmd.PageID, cmd.Path); err == nil {
		if !cmd.Overwrite {
			return WriteResult{}, service.BadRequest(fmt.Sprintf(
				"%s already exists (%d bytes) — read it first, then use edit_file for a targeted change, or pass overwrite:true to replace it wholesale.",
				cmd.Path, len(existing)))
		}
		a.snapshotFile(ctx, cmd.PageID, cmd.Path, existing, "write")
	}
	if err := a.store.WriteFile(ctx, cmd.PageID, cmd.Path, []byte(cmd.Content)); err != nil {
		return WriteResult{}, err
	}
	return WriteResult{OK: true, Path: cmd.Path, Build: a.autoBuild(ctx, cmd.PageID, cmd.Build)}, nil
}

func (a *Authoring) EditFile(ctx context.Context, cmd EditCommand) (EditResult, error) {
	if err := a.RequireApp(ctx, cmd.PageID); err != nil {
		return EditResult{}, err
	}
	if strings.TrimSpace(cmd.Path) == "" {
		return EditResult{}, service.BadRequest("path is required")
	}
	if cmd.OldString == "" {
		return EditResult{}, service.BadRequest("old_string is required")
	}
	if cmd.OldString == cmd.NewString {
		return EditResult{}, service.BadRequest("old_string and new_string are identical")
	}
	data, err := a.store.ReadFile(ctx, cmd.PageID, cmd.Path)
	if err != nil {
		return EditResult{}, err
	}
	updated, count, err := ApplyStringEdit(string(data), cmd.OldString, cmd.NewString, cmd.ReplaceAll)
	switch {
	case errors.Is(err, ErrEditNotFound):
		return EditResult{}, service.BadRequest("old_string not found in " + cmd.Path)
	case errors.Is(err, ErrEditAmbiguous):
		return EditResult{}, service.BadRequest(fmt.Sprintf(
			"old_string matches %d times in %s; add surrounding context to make it unique, or set replace_all", count, cmd.Path))
	case err != nil:
		return EditResult{}, err
	}
	if err := a.store.WriteFile(ctx, cmd.PageID, cmd.Path, []byte(updated)); err != nil {
		return EditResult{}, err
	}
	return EditResult{OK: true, Path: cmd.Path, Replacements: count, Build: a.autoBuild(ctx, cmd.PageID, cmd.Build)}, nil
}

func (a *Authoring) DeleteFile(ctx context.Context, cmd DeleteCommand) (DeleteResult, error) {
	if err := a.RequireApp(ctx, cmd.PageID); err != nil {
		return DeleteResult{}, err
	}
	if strings.TrimSpace(cmd.Path) == "" {
		return DeleteResult{}, service.BadRequest("path is required")
	}
	if existing, err := a.store.ReadFile(ctx, cmd.PageID, cmd.Path); err == nil {
		a.snapshotFile(ctx, cmd.PageID, cmd.Path, existing, "delete")
	}
	if err := a.store.DeleteFile(ctx, cmd.PageID, cmd.Path); err != nil {
		return DeleteResult{}, err
	}
	return DeleteResult{OK: true, Deleted: cmd.Path, Build: a.autoBuild(ctx, cmd.PageID, cmd.Build)}, nil
}

func (a *Authoring) InstallLib(ctx context.Context, pageID, name, rawURL string) ([]service.LibEntry, error) {
	if err := a.RequireApp(ctx, pageID); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, service.BadRequest("name is required")
	}
	url := strings.TrimSpace(rawURL)
	if url == "" {
		url = "https://esm.sh/" + name
	} else if !strings.HasPrefix(url, "https://esm.sh/") {
		return nil, service.BadRequest("url must be an https://esm.sh/ URL")
	}
	key := name
	if at := strings.LastIndex(name, "@"); at > 0 {
		key = name[:at]
	}
	return a.store.InstallLib(ctx, pageID, service.LibEntry{Name: key, URL: url})
}

func (a *Authoring) Build(ctx context.Context, pageID string, channel service.BuildChannel) (service.BuildResult, error) {
	if err := a.RequireApp(ctx, pageID); err != nil {
		return service.BuildResult{}, err
	}
	return a.build.Build(ctx, pageID, channel)
}

func ApplyStringEdit(content, oldString, newString string, replaceAll bool) (string, int, error) {
	count := strings.Count(content, oldString)
	switch {
	case count == 0:
		return "", 0, ErrEditNotFound
	case count > 1 && !replaceAll:
		return "", count, ErrEditAmbiguous
	case replaceAll:
		return strings.ReplaceAll(content, oldString, newString), count, nil
	default:
		return strings.Replace(content, oldString, newString, 1), 1, nil
	}
}

func (a *Authoring) autoBuild(ctx context.Context, pageID string, build *bool) *service.BuildResult {
	if build != nil && !*build {
		return nil
	}
	res, err := a.build.Build(ctx, pageID, service.ChannelDraft)
	if err != nil {
		return &service.BuildResult{OK: false, Log: err.Error()}
	}
	return &res
}

func (a *Authoring) snapshotFile(ctx context.Context, pageID, path string, content []byte, operation string) {
	stamp := time.Now().UTC().Format("20060102T150405.000000000Z")
	dest := fmt.Sprintf("%s/%s-%s-%s", HistoryDir, stamp, operation, strings.ReplaceAll(path, "/", "__"))
	if err := a.store.WriteFile(ctx, pageID, dest, content); err != nil {
		return
	}
	a.pruneHistory(ctx, pageID)
}

func (a *Authoring) pruneHistory(ctx context.Context, pageID string) {
	entries, err := a.store.ListDir(ctx, pageID, HistoryDir)
	if err != nil || len(entries) <= HistoryKeep {
		return
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir {
			names = append(names, entry.Name)
		}
	}
	if len(names) <= HistoryKeep {
		return
	}
	sort.Strings(names)
	for _, name := range names[:len(names)-HistoryKeep] {
		_ = a.store.DeleteFile(ctx, pageID, HistoryDir+"/"+name)
	}
}
