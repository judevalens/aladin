package search

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"aladin/backend_v2/internal/ratelimit"
)

const defaultTTL = 7 * 24 * time.Hour

type CachedSearcher struct {
	client  *TavilyClient
	redis   *redis.Client
	limiter *ratelimit.Limiter
	ttl     time.Duration
}

func NewCachedSearcher(tavily *TavilyClient, redis *redis.Client, limiter *ratelimit.Limiter) *CachedSearcher {
	return &CachedSearcher{
		client:  tavily,
		redis:   redis,
		limiter: limiter,
		ttl:     defaultTTL,
	}
}

func (s *CachedSearcher) Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	key := fmt.Sprintf("search:%s", query)

	// Cache hit
	cached, err := s.redis.Get(ctx, key).Bytes()
	if err == nil {
		var results []SearchResult
		if err := json.Unmarshal(cached, &results); err == nil {
			return results, nil
		}
	}

	// Cache miss — rate limit then call Tavily
	if err := s.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	results, err := s.client.Search(ctx, query, maxResults)
	if err != nil {
		return nil, err
	}

	// Store in Redis with TTL
	if data, err := json.Marshal(results); err == nil {
		s.redis.Set(ctx, key, data, s.ttl)
	}

	return results, nil
}
