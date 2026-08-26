package service

import (
	"context"
	"errors"
	"strings"
)

// ReadingPosition is the synced "you are at page N of this document" row —
// per (user, artifact), last-write-wins across devices. UpdatedAt is unix
// milliseconds so clients can compare it against their own session caches
// (newer-of at open) without timestamp parsing.
type ReadingPosition struct {
	ArtifactID string `json:"artifactId"`
	Page       int64  `json:"page"`
	Seq        int64  `json:"seq"`
	UpdatedAt  int64  `json:"updatedAt"` // unix ms
}

var (
	// ErrInvalidReadingPositionInput is returned for an empty user/artifact id or page < 1.
	ErrInvalidReadingPositionInput = errors.New("reading position: user, artifact id, and page >= 1 are required")
	// ErrReadingPositionNotFound is returned when the artifact isn't the user's (PUT)
	// or no position exists (GET).
	ErrReadingPositionNotFound = errors.New("reading position: not found")
)

// ReadingPositionRepository is the persistence port. PutReadingPosition upserts
// LWW, bumps seq, and appends the sync frame in one tx.
type ReadingPositionRepository interface {
	PutReadingPosition(ctx context.Context, userID, artifactID string, page int64) (ReadingPosition, error)
	GetReadingPosition(ctx context.Context, userID, artifactID string) (ReadingPosition, bool, error)
}

// ReadingPositionService records and reads where the user is in a document.
// Reads on clients normally ride the sync replica; the GET exists for the
// writer's confirmation (anchor applies the committed row) and debugging.
type ReadingPositionService interface {
	Put(ctx context.Context, userID, artifactID string, page int64) (ReadingPosition, error)
	Get(ctx context.Context, userID, artifactID string) (ReadingPosition, bool, error)
}

type defaultReadingPositionService struct {
	repo ReadingPositionRepository
}

func NewReadingPositionService(repo ReadingPositionRepository) ReadingPositionService {
	return &defaultReadingPositionService{repo: repo}
}

func (s *defaultReadingPositionService) Put(ctx context.Context, userID, artifactID string, page int64) (ReadingPosition, error) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(artifactID) == "" || page < 1 {
		return ReadingPosition{}, ErrInvalidReadingPositionInput
	}
	return s.repo.PutReadingPosition(ctx, userID, artifactID, page)
}

func (s *defaultReadingPositionService) Get(ctx context.Context, userID, artifactID string) (ReadingPosition, bool, error) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(artifactID) == "" {
		return ReadingPosition{}, false, ErrInvalidReadingPositionInput
	}
	return s.repo.GetReadingPosition(ctx, userID, artifactID)
}
