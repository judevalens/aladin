package sync

import "context"

// SeenStore is the fast visited-set for sync traversal.
// The backing implementation is intentionally abstract so Redis can be swapped later.
type SeenStore interface {
	Seen(ctx context.Context, sourceID string, externalIDs []string) (map[string]bool, error)
	MarkSeen(ctx context.Context, sourceID string, externalIDs []string) error
}

type noopSeenStore struct{}

func NewNoopSeenStore() SeenStore { return noopSeenStore{} }

func (noopSeenStore) Seen(ctx context.Context, sourceID string, externalIDs []string) (map[string]bool, error) {
	return map[string]bool{}, nil
}

func (noopSeenStore) MarkSeen(ctx context.Context, sourceID string, externalIDs []string) error {
	return nil
}
