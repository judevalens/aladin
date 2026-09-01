package service

import (
	"context"
	"log/slog"
	"time"
)

// Shard build status — the live authoring signal. A build records 'building'
// then a terminal 'ok'|'failed' into shard_build_state, each transition emitting
// an "artifact.build-status" app_event over the outbox so it reaches the API
// process's websocket from the MCP build process. Ephemeral UI state, NOT part
// of the durable data-sync stream.

type ShardBuildStatus string

const (
	ShardBuildBuilding ShardBuildStatus = "building"
	ShardBuildOK       ShardBuildStatus = "ok"
	ShardBuildFailed   ShardBuildStatus = "failed"
)

// ShardBuildState is the current build status for one page + channel.
type ShardBuildState struct {
	PageID  string           `json:"page_id"`
	Channel BuildChannel     `json:"channel"`
	Status  ShardBuildStatus `json:"status"`
	Errors  string           `json:"errors,omitempty"`   // build log, on failure
	BuildID string           `json:"build_id,omitempty"` // content hash, on ok
	BuiltAt string           `json:"built_at,omitempty"` // RFC3339, on ok
}

// ShardBuildStore persists build state and emits the build-status app_event in
// the SAME transaction (transactional outbox). Implemented by the repo.
type ShardBuildStore interface {
	SetStatus(ctx context.Context, st ShardBuildState) error
	GetStatus(ctx context.Context, pageID string, channel BuildChannel) (ShardBuildState, error)
}

// ShardBuildService runs a build while recording its status transitions and
// emitting the build-status events the live view reacts to. It wraps the
// fs-only WorkspaceRuntime (which stays free of DB/event concerns).
type ShardBuildService interface {
	// Build records 'building', runs the underlying build, then records the
	// terminal status (with build_id on success / log on failure). The build
	// result is always returned faithfully; status recording is best-effort and
	// never turns a good build into an error.
	Build(ctx context.Context, pageID string, channel BuildChannel) (BuildResult, error)
	// State returns the current build state for a page + channel.
	State(ctx context.Context, pageID string, channel BuildChannel) (ShardBuildState, error)
}

type shardBuildService struct {
	runtime  WorkspaceRuntime
	store    ShardBuildStore
	releases ShardReleaseService
}

func NewShardBuildService(runtime WorkspaceRuntime, store ShardBuildStore, releases ...ShardReleaseService) ShardBuildService {
	s := &shardBuildService{runtime: runtime, store: store}
	if len(releases) > 0 {
		s.releases = releases[0]
	}
	return s
}

func (s *shardBuildService) Build(ctx context.Context, pageID string, channel BuildChannel) (BuildResult, error) {
	s.record(ctx, ShardBuildState{PageID: pageID, Channel: channel, Status: ShardBuildBuilding})

	res, err := s.runtime.Build(ctx, pageID, channel)
	if err == nil && res.OK && len(res.Contract) > 0 {
		if s.releases == nil {
			err = ResourceFailure("unsupported-capability", "V2 release activation is disabled")
		} else {
			err = s.releases.Stage(ctx, pageID, channel, res)
			if err == nil && channel == ChannelDraft {
				err = s.releases.Activate(ctx, pageID, channel, res.BuildID)
			}
		}
	}
	if err != nil {
		s.record(ctx, ShardBuildState{PageID: pageID, Channel: channel, Status: ShardBuildFailed, Errors: err.Error()})
		return res, err
	}

	st := ShardBuildState{PageID: pageID, Channel: channel}
	if res.OK {
		st.Status = ShardBuildOK
		st.BuildID = res.BuildID
		st.BuiltAt = time.Now().UTC().Format(time.RFC3339)
	} else {
		st.Status = ShardBuildFailed
		st.Errors = res.Log
	}
	s.record(ctx, st)
	return res, nil
}

func (s *shardBuildService) State(ctx context.Context, pageID string, channel BuildChannel) (ShardBuildState, error) {
	return s.store.GetStatus(ctx, pageID, channel)
}

// record writes status best-effort: a status/event write failure must not fail
// an otherwise-good build (the artifact is built regardless; the live view is a
// nicety).
func (s *shardBuildService) record(ctx context.Context, st ShardBuildState) {
	if err := s.store.SetStatus(ctx, st); err != nil {
		slog.Warn("shard build: record status", "page_id", st.PageID, "channel", st.Channel, "status", st.Status, "err", err)
	}
}
