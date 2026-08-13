package service

import (
	"context"
	"encoding/json"
	"errors"
)

// The launch adapters for the entity registry. Each wraps an existing SERVICE so
// ownership + scope rules are not re-implemented here, and shapes a NodeView that
// is deliberately small — projection data, never whole bodies.

// --- artifact ----------------------------------------------------------------

type artifactEntityService struct{ artifacts ArtifactService }

func NewArtifactEntityService(artifacts ArtifactService) EntityService {
	return artifactEntityService{artifacts: artifacts}
}

func (artifactEntityService) Kind() string { return "artifact" }

// Observable: artifacts ride the tree sync source (every write emits a node frame).
func (artifactEntityService) Observable() bool { return true }

func (s artifactEntityService) Exists(ctx context.Context, id string) (bool, error) {
	_, err := s.artifacts.Get(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s artifactEntityService) NodeView(ctx context.Context, id string) (NodeView, error) {
	rec, err := s.artifacts.Get(ctx, id)
	if err != nil {
		return NodeView{}, err
	}
	// Page BLOCKS are deliberately omitted: a page body is large, structured, and
	// belongs to the editor. A shard projects the summary (or links out); if it
	// truly needs the text, that content should be promoted to records.
	content, truncated := TruncateForView(rec.Content)
	data := map[string]any{
		"type":      rec.Type,
		"title":     rec.Title,
		"updatedAt": rec.UpdatedAt,
	}
	if rec.Summary != nil {
		data["summary"] = *rec.Summary
	}
	if rec.SourceURL != nil {
		data["sourceUrl"] = *rec.SourceURL
	}
	if rec.Type != "page" && content != "" {
		data["content"] = content
	}
	if props, ok := rec.Metadata["properties"]; ok {
		data["properties"] = props
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return NodeView{}, err
	}
	return NodeView{
		ID:        rec.ID,
		Kind:      "artifact",
		Title:     rec.Title,
		Data:      raw,
		Seq:       rec.Seq,
		Truncated: truncated,
	}, nil
}

// --- record ------------------------------------------------------------------

// RecordReader is the narrow read the bridge needs. Records have NO owner column
// in the schema (see 00001_baseline records) — every authenticated caller already
// sees every record through /api/records, so this adapter is no wider than the
// existing surface. The manifest grant is the real gate here.
type RecordReader interface {
	GetRecord(ctx context.Context, id string) (RecordResponse, bool, error)
}

type recordEntityService struct{ records RecordReader }

func NewRecordEntityService(records RecordReader) EntityService {
	return recordEntityService{records: records}
}

func (recordEntityService) Kind() string { return "record" }

// Records do NOT emit sync frames today: a shard can render one, but the region
// can never go live (the publish gate says so rather than letting it look live).
func (recordEntityService) Observable() bool { return false }

func (s recordEntityService) Exists(ctx context.Context, id string) (bool, error) {
	_, ok, err := s.records.GetRecord(ctx, id)
	return ok, err
}

func (s recordEntityService) NodeView(ctx context.Context, id string) (NodeView, error) {
	rec, ok, err := s.records.GetRecord(ctx, id)
	if err != nil {
		return NodeView{}, err
	}
	if !ok {
		return NodeView{}, ErrNotFound
	}
	content, truncated := TruncateForView(rec.Content)
	data := map[string]any{
		"type":      rec.Type,
		"label":     rec.Label,
		"content":   content,
		"createdAt": rec.CreatedAt,
	}
	if rec.SourceURL != nil {
		data["sourceUrl"] = *rec.SourceURL
	}
	if rec.Enrichment != nil {
		data["enrichment"] = rec.Enrichment
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return NodeView{}, err
	}
	return NodeView{ID: rec.ID, Kind: "record", Title: rec.Label, Data: raw, Truncated: truncated}, nil
}

// --- watchlist ---------------------------------------------------------------

// watchlistEntityService reads a list + its members. Watchlists are user-scoped
// by argument (not ctx), so the adapter resolves the principal itself.
type watchlistEntityService struct{ watchlists WatchlistService }

func NewWatchlistEntityService(watchlists WatchlistService) EntityService {
	return watchlistEntityService{watchlists: watchlists}
}

func (watchlistEntityService) Kind() string { return "watchlist" }

// Observable: watchlists have their own sync source (add/remove re-emits the list).
func (watchlistEntityService) Observable() bool { return true }

func (s watchlistEntityService) Exists(ctx context.Context, id string) (bool, error) {
	principal, err := RequirePrincipal(ctx)
	if err != nil {
		return false, err
	}
	lists, err := s.watchlists.ListWatchlists(ctx, principal.UserID)
	if err != nil {
		return false, err
	}
	for _, l := range lists {
		if l.ID == id {
			return true, nil
		}
	}
	return false, nil
}

func (s watchlistEntityService) NodeView(ctx context.Context, id string) (NodeView, error) {
	principal, err := RequirePrincipal(ctx)
	if err != nil {
		return NodeView{}, err
	}
	lists, err := s.watchlists.ListWatchlists(ctx, principal.UserID)
	if err != nil {
		return NodeView{}, err
	}
	var found *Watchlist
	for i := range lists {
		if lists[i].ID == id {
			found = &lists[i]
			break
		}
	}
	if found == nil {
		return NodeView{}, ErrNotFound
	}
	items, err := s.watchlists.ListItems(ctx, principal.UserID, id)
	if err != nil {
		return NodeView{}, err
	}
	raw, err := json.Marshal(map[string]any{
		"name":      found.Name,
		"kind":      found.Kind,
		"position":  found.Position,
		"itemCount": len(items),
		"items":     items,
	})
	if err != nil {
		return NodeView{}, err
	}
	return NodeView{ID: id, Kind: "watchlist", Title: found.Name, Data: raw}, nil
}

// --- research ----------------------------------------------------------------

type researchEntityService struct{ research ResearchService }

func NewResearchEntityService(research ResearchService) EntityService {
	return researchEntityService{research: research}
}

func (researchEntityService) Kind() string { return "research" }

// Observable: a research folder is a tree kind — its frames ride the tree source.
func (researchEntityService) Observable() bool { return true }

func (s researchEntityService) Exists(ctx context.Context, id string) (bool, error) {
	_, err := s.research.Get(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s researchEntityService) NodeView(ctx context.Context, id string) (NodeView, error) {
	folder, err := s.research.Get(ctx, id)
	if err != nil {
		return NodeView{}, err
	}
	hypothesis, truncated := TruncateForView(folder.Hypothesis)
	raw, err := json.Marshal(map[string]any{
		"title":      folder.Title,
		"hypothesis": hypothesis,
		"sourceKind": folder.SourceKind,
		"execMode":   folder.ExecMode,
		"runState":   folder.RunState,
	})
	if err != nil {
		return NodeView{}, err
	}
	return NodeView{
		ID:        folder.NodeID,
		Kind:      "research",
		Title:     folder.Title,
		Data:      raw,
		Truncated: truncated,
	}, nil
}
