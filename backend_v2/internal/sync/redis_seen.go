package sync

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

const redisSeenHashKey = "sync:seen"

type redisSeenStore struct {
	client *redis.Client
}

func NewRedisSeenStore(client *redis.Client) SeenStore {
	if client == nil {
		return NewNoopSeenStore()
	}
	return &redisSeenStore{client: client}
}

func (s *redisSeenStore) Seen(ctx context.Context, sourceID string, externalIDs []string) (map[string]bool, error) {
	known := make(map[string]bool, len(externalIDs))
	if len(externalIDs) == 0 {
		return known, nil
	}

	fields := make([]string, 0, len(externalIDs))
	ids := make([]string, 0, len(externalIDs))
	for _, id := range externalIDs {
		if id == "" {
			continue
		}
		fields = append(fields, seenField(sourceID, id))
		ids = append(ids, id)
	}
	if len(fields) == 0 {
		return known, nil
	}

	values, err := s.client.HMGet(ctx, redisSeenHashKey, fields...).Result()
	if err != nil {
		return nil, fmt.Errorf("redis seen hmget: %w", err)
	}
	for i, v := range values {
		if v != nil {
			known[ids[i]] = true
		}
	}
	return known, nil
}

func (s *redisSeenStore) MarkSeen(ctx context.Context, sourceID string, externalIDs []string) error {
	if len(externalIDs) == 0 {
		return nil
	}

	values := make(map[string]any, len(externalIDs))
	for _, id := range externalIDs {
		if id == "" {
			continue
		}
		values[seenField(sourceID, id)] = "1"
	}
	if len(values) == 0 {
		return nil
	}

	if err := s.client.HSet(ctx, redisSeenHashKey, values).Err(); err != nil {
		return fmt.Errorf("redis seen hset: %w", err)
	}
	return nil
}

func seenField(sourceID, externalID string) string {
	return sourceID + ":" + externalID
}
