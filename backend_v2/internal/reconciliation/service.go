// Package reconciliation owns client pull and snapshot recovery policy.
package reconciliation

import (
	"context"

	coreservice "aladin/backend_v2/internal/service"
)

type Service struct {
	outbox  coreservice.OutboxReader
	sources []coreservice.SyncSource
}

func New(outbox coreservice.OutboxReader, sources ...coreservice.SyncSource) *Service {
	return &Service{outbox: outbox, sources: sources}
}

func (s *Service) Pull(ctx context.Context, userID string, cursor uint64) (coreservice.PullResult, error) {
	if cursor == 0 {
		return s.snapshot(ctx, userID)
	}
	minXid, ok, err := s.outbox.MinXid(ctx, userID)
	if err != nil {
		return coreservice.PullResult{}, err
	}
	if ok && cursor < minXid {
		return s.snapshot(ctx, userID)
	}
	current, err := s.outbox.Horizon(ctx)
	if err != nil {
		return coreservice.PullResult{}, err
	}
	if cursor > current {
		return s.snapshot(ctx, userID)
	}
	frames, horizon, err := s.outbox.PullSince(ctx, userID, cursor)
	if err != nil {
		return coreservice.PullResult{}, err
	}
	return coreservice.PullResult{Frames: frames, Cursor: horizon, Mode: coreservice.PullModeDelta}, nil
}

func (s *Service) snapshot(ctx context.Context, userID string) (coreservice.PullResult, error) {
	var entities []coreservice.FrameEntity
	for _, src := range s.sources {
		es, err := src.Snapshot(ctx, userID)
		if err != nil {
			return coreservice.PullResult{}, err
		}
		entities = append(entities, es...)
	}
	horizon, err := s.outbox.Horizon(ctx)
	if err != nil {
		return coreservice.PullResult{}, err
	}
	frames := []coreservice.Frame{}
	if len(entities) > 0 {
		frames = append(frames, coreservice.Frame{Entities: entities})
	}
	return coreservice.PullResult{Frames: frames, Cursor: horizon, Mode: coreservice.PullModeSnapshot}, nil
}
