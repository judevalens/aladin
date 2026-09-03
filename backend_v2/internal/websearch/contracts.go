package websearch

import "context"

// Searcher is the interface for web search.
// CachedSearcher implements this — swap for a mock in tests.
type Searcher interface {
	Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error)
}
