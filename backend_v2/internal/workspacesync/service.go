// Package workspacesync owns client pull and snapshot recovery policy.
package workspacesync

import "context"

type Service struct {
	outbox  OutboxReader
	sources []SyncSource
}

func New(outbox OutboxReader, sources ...SyncSource) *Service {
	return &Service{outbox: outbox, sources: sources}
}

func (s *Service) Pull(ctx context.Context, userID string, cursor uint64) (PullResult, error) {
	if cursor == 0 {
		return s.snapshot(ctx, userID)
	}
	minXid, ok, err := s.outbox.MinXid(ctx, userID)
	if err != nil {
		return PullResult{}, err
	}
	if ok && cursor < minXid {
		return s.snapshot(ctx, userID)
	}
	current, err := s.outbox.Horizon(ctx)
	if err != nil {
		return PullResult{}, err
	}
	if cursor > current {
		return s.snapshot(ctx, userID)
	}
	frames, horizon, err := s.outbox.PullSince(ctx, userID, cursor)
	if err != nil {
		return PullResult{}, err
	}
	return PullResult{Frames: frames, Cursor: horizon, Mode: PullModeDelta}, nil
}

func (s *Service) snapshot(ctx context.Context, userID string) (PullResult, error) {
	var entities []FrameEntity
	for _, src := range s.sources {
		es, err := src.Snapshot(ctx, userID)
		if err != nil {
			return PullResult{}, err
		}
		entities = append(entities, es...)
	}
	horizon, err := s.outbox.Horizon(ctx)
	if err != nil {
		return PullResult{}, err
	}
	frames := []Frame{}
	if len(entities) > 0 {
		frames = append(frames, Frame{Entities: entities})
	}
	return PullResult{Frames: frames, Cursor: horizon, Mode: PullModeSnapshot}, nil
}
