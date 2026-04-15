package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

const (
	defaultUserID = "00000000-0000-0000-0000-000000000001"
	defaultKGName = "Default Research Graph"
)

type sourceRecord struct {
	ID                   string         `json:"id"`
	Name                 string         `json:"name"`
	Type                 string         `json:"type"`
	SyncMode             string         `json:"syncMode"`
	SyncState            string         `json:"syncState"`
	Config               map[string]any `json:"config"`
	AutoPromoteThreshold float64        `json:"autoPromoteThreshold"`
	SuggestThreshold     float64        `json:"suggestThreshold"`
	CreatedAt            string         `json:"createdAt"`
	LastSyncedAt         *string        `json:"lastSyncedAt"`
}

func (s *Server) handleSourcesList(w http.ResponseWriter, r *http.Request) {
	out, err := s.deps.Sources.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleSourcesCreate(w http.ResponseWriter, r *http.Request) {
	var data map[string]any
	if err := readJSON(r, &data); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}

	payload, err := buildSourcePayload(data)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	rec, err := s.deps.Sources.Create(r.Context(), payload)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, rec)
}

func (s *Server) handleSourcesDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := uuid.Parse(id); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid source id"})
		return
	}
	if err := s.deps.Sources.Delete(r.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Source not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type sourcePayload struct {
	Name     string
	Type     string
	SyncMode string
	Config   map[string]any
}

func buildSourcePayload(data map[string]any) (*sourcePayload, error) {
	kind, err := cleanString(data["kind"], "kind")
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(asString(data["name"]))

	switch kind {
	case "reddit_subreddit":
		subreddit, err := cleanString(data["subreddit"], "subreddit")
		if err != nil {
			return nil, err
		}
		subreddit = strings.Trim(strings.TrimPrefix(subreddit, "r/"), "/")
		sort := strings.ToLower(strings.TrimSpace(asStringDefault(data["sort"], "new")))
		if sort != "new" && sort != "hot" && sort != "top" {
			return nil, errBadRequest("sort must be one of: new, hot, top")
		}
		return &sourcePayload{
			Name:     firstNonEmpty(name, "r/"+subreddit),
			Type:     "reddit",
			SyncMode: "poll",
			Config: map[string]any{
				"subreddit":             subreddit,
				"min_score":             maxInt(asIntDefault(data["minScore"], 10), 0),
				"include_comments":      asBoolDefault(data["includeComments"], true),
				"top_comments_n":        maxInt(asIntDefault(data["topComments"], 5), 0),
				"sort":                  sort,
				"after":                 nil,
				"last_seen_id":          nil,
				"sync_interval_seconds": asIntDefault(data["syncIntervalSeconds"], 10),
			},
		}, nil
	case "bluesky_search":
		query, err := cleanString(data["query"], "query")
		if err != nil {
			return nil, err
		}
		return &sourcePayload{
			Name:     firstNonEmpty(name, "Bluesky: "+truncate(query, 48)),
			Type:     "bluesky",
			SyncMode: "poll",
			Config: map[string]any{
				"mode":                  "search",
				"query":                 query,
				"cursor":                nil,
				"limit":                 maxInt(asIntDefault(data["limit"], 50), 1),
				"sync_interval_seconds": asIntDefault(data["syncIntervalSeconds"], 10),
			},
		}, nil
	case "bluesky_account":
		handle, err := cleanString(data["handle"], "handle")
		if err != nil {
			return nil, err
		}
		handle = strings.TrimPrefix(handle, "@")
		return &sourcePayload{
			Name:     firstNonEmpty(name, "@"+handle),
			Type:     "bluesky",
			SyncMode: "poll",
			Config: map[string]any{
				"mode":                  "account",
				"handle":                handle,
				"cursor":                nil,
				"limit":                 maxInt(asIntDefault(data["limit"], 50), 1),
				"sync_interval_seconds": asIntDefault(data["syncIntervalSeconds"], 10),
			},
		}, nil
	case "twitter_search":
		query, err := cleanString(data["query"], "query")
		if err != nil {
			return nil, err
		}
		return &sourcePayload{
			Name:     firstNonEmpty(name, "X: "+truncate(query, 48)),
			Type:     "twitter",
			SyncMode: "poll",
			Config: map[string]any{
				"mode":                  "search",
				"query":                 query,
				"since_id":              nil,
				"min_likes":             maxInt(asIntDefault(data["minLikes"], 5), 0),
				"max_results":           maxInt(asIntDefault(data["maxResults"], 25), 10),
				"sync_interval_seconds": asIntDefault(data["syncIntervalSeconds"], 10),
			},
		}, nil
	default:
		return nil, errBadRequest("Unsupported source kind '" + kind + "'")
	}
}

type badRequest string

func (e badRequest) Error() string   { return string(e) }
func errBadRequest(msg string) error { return badRequest(msg) }

func cleanString(value any, field string) (string, error) {
	s := strings.TrimSpace(asString(value))
	if s == "" {
		return "", errBadRequest(field + " is required")
	}
	return s, nil
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	default:
		return ""
	}
}

func asStringDefault(v any, fallback string) string {
	s := strings.TrimSpace(asString(v))
	if s == "" {
		return fallback
	}
	return s
}

func asIntDefault(v any, fallback int) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(n))
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func asBoolDefault(v any, fallback bool) bool {
	switch b := v.(type) {
	case bool:
		return b
	case string:
		switch strings.ToLower(strings.TrimSpace(b)) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return fallback
}

func firstNonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
