package page

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"aladin/backend_v2/internal/apperror"
	"aladin/backend_v2/internal/auth"
	"aladin/backend_v2/internal/blocknote"
)

// DocumentService is the only service callers should reach for to read or
// write page block content. It is the seam that lets M7 (collaborative editing
// via Yjs/Hocuspocus) swap the storage backend without rewriting every caller:
// today this is JSONB read/write, tomorrow it loads a Y.Doc, applies CRDT
// operations, and broadcasts to live editors.
//
// MCP tools and API handlers MUST go through this interface for any page-block
// mutation. They MUST NOT call the artifact repo's page document methods
// directly.
//
// M5 only implements GetBlocks and ReplaceAll. The block-ID-targeted methods
// are stubbed and return ErrNotImplemented until M6 wires up block_ops.
type DocumentService interface {
	// GetBlocks returns the page's current block document and revision. The
	// blocks value is a JSON array of BlockNote blocks; opaque to this layer.
	GetBlocks(ctx context.Context, pageID string) (blocks json.RawMessage, revision int64, err error)

	// ReplaceAll replaces the entire block document. If expectedRevision is
	// non-zero the call fails with ErrConflict when the stored revision is
	// >= expectedRevision (i.e. someone else wrote ahead of us). Pass 0 for
	// last-write-wins.
	ReplaceAll(ctx context.Context, pageID string, blocks json.RawMessage, expectedRevision int64) (newRevision int64, err error)

	// ReplaceBlock replaces the block with the given ID. The replacement
	// markdown may yield multiple blocks; the original block_id is preserved
	// on the first new block. Returns the new revision and the number of
	// blocks the replacement parsed into. (M6.)
	ReplaceBlock(ctx context.Context, pageID, blockID string, replacement json.RawMessage, expectedRevision int64) (newRevision int64, blockCount int, err error)

	// InsertBlocks inserts blocks at the resolved position. Returns the new
	// revision and the IDs of the inserted blocks (in order). (M6.)
	InsertBlocks(ctx context.Context, pageID string, position BlockPosition, blocks json.RawMessage, expectedRevision int64) (newRevision int64, insertedIDs []string, err error)

	// DeleteBlock removes a block. Refuses to delete the last block on a
	// page (BlockNote requires >=1). (M6.)
	DeleteBlock(ctx context.Context, pageID, blockID string, expectedRevision int64) (newRevision int64, err error)
}

// BlockPosition addresses where InsertBlocks should place new content.
// Exactly one of AfterID, BeforeID, At must be set; the implementation
// reports a BadRequest otherwise. "At" accepts "start" or "end".
type BlockPosition struct {
	AfterID  *string
	BeforeID *string
	At       *string
}

// ErrNotImplemented is returned by M6-scope methods while M5 is in flight.
var ErrNotImplemented = errors.New("not implemented")

// DocumentStore is the minimal storage surface DocumentService needs.
// A concrete implementation lives in the repo package; tests stub it.
type DocumentStore interface {
	GetPageBlocks(ctx context.Context, pageID string) (blocks json.RawMessage, revision int64, err error)
	SavePageBlocks(ctx context.Context, pageID string, blocks json.RawMessage, searchText string, expectedRevision int64) (newRevision int64, err error)
}

// DefaultDocumentService is the production implementation backed by
// DocumentStore. The store does the SQL; this layer owns the search_text
// derivation, revision arithmetic, and (eventually) the block-ops splicing.
type DefaultDocumentService struct {
	store DocumentStore
}

func NewDocumentService(store DocumentStore) *DefaultDocumentService {
	return &DefaultDocumentService{store: store}
}

func (s *DefaultDocumentService) GetBlocks(ctx context.Context, pageID string) (json.RawMessage, int64, error) {
	if err := auth.RequireScope(ctx, auth.ScopeArtifactsRead); err != nil {
		return nil, 0, err
	}
	return s.store.GetPageBlocks(ctx, pageID)
}

func (s *DefaultDocumentService) ReplaceAll(ctx context.Context, pageID string, blocks json.RawMessage, expectedRevision int64) (int64, error) {
	if err := auth.RequireScope(ctx, auth.ScopeArtifactsWrite); err != nil {
		return 0, err
	}
	if len(blocks) == 0 || !looksLikeJSONArray(blocks) {
		return 0, apperror.BadRequest("blocks must be a JSON array")
	}
	searchText, err := blocknote.ExtractText(blocks)
	if err != nil {
		return 0, apperror.BadRequest("blocks: " + err.Error())
	}
	return s.store.SavePageBlocks(ctx, pageID, blocks, searchText, expectedRevision)
}

func (s *DefaultDocumentService) ReplaceBlock(ctx context.Context, pageID, blockID string, replacement json.RawMessage, expectedRevision int64) (int64, int, error) {
	if err := auth.RequireScope(ctx, auth.ScopeArtifactsWrite); err != nil {
		return 0, 0, err
	}
	if strings.TrimSpace(blockID) == "" {
		return 0, 0, apperror.BadRequest("block_id is required")
	}
	if !looksLikeJSONArray(replacement) {
		return 0, 0, apperror.BadRequest("replacement must be a JSON array")
	}
	current, _, err := s.store.GetPageBlocks(ctx, pageID)
	if err != nil {
		return 0, 0, err
	}
	blocks, err := blocknote.ParseBlocks(current)
	if err != nil {
		return 0, 0, err
	}
	newBlocks, err := blocknote.ParseBlocks(replacement)
	if err != nil {
		return 0, 0, apperror.BadRequest("replacement: " + err.Error())
	}
	updated, count, err := blocknote.ReplaceByID(blocks, blockID, newBlocks)
	if err != nil {
		if errors.Is(err, blocknote.ErrBlockNotFound) {
			return 0, 0, apperror.ErrNotFound
		}
		return 0, 0, err
	}
	return s.persistBlocks(ctx, pageID, updated, expectedRevision, count)
}

func (s *DefaultDocumentService) InsertBlocks(ctx context.Context, pageID string, position BlockPosition, newDoc json.RawMessage, expectedRevision int64) (int64, []string, error) {
	if err := auth.RequireScope(ctx, auth.ScopeArtifactsWrite); err != nil {
		return 0, nil, err
	}
	if !looksLikeJSONArray(newDoc) {
		return 0, nil, apperror.BadRequest("blocks must be a JSON array")
	}
	if err := validatePosition(position); err != nil {
		return 0, nil, err
	}
	current, _, err := s.store.GetPageBlocks(ctx, pageID)
	if err != nil {
		return 0, nil, err
	}
	blocks, err := blocknote.ParseBlocks(current)
	if err != nil {
		return 0, nil, err
	}
	newBlocks, err := blocknote.ParseBlocks(newDoc)
	if err != nil {
		return 0, nil, apperror.BadRequest("blocks: " + err.Error())
	}
	if len(newBlocks) == 0 {
		return 0, nil, apperror.BadRequest("blocks: at least one block required")
	}
	var updated []blocknote.Block
	switch {
	case position.AfterID != nil:
		updated, err = blocknote.InsertAfter(blocks, *position.AfterID, newBlocks)
	case position.BeforeID != nil:
		updated, err = blocknote.InsertBefore(blocks, *position.BeforeID, newBlocks)
	case position.At != nil:
		switch *position.At {
		case "start":
			updated = blocknote.InsertAtStart(blocks, newBlocks)
		case "end":
			updated = blocknote.InsertAtEnd(blocks, newBlocks)
		}
	}
	if err != nil {
		if errors.Is(err, blocknote.ErrBlockNotFound) {
			return 0, nil, apperror.ErrNotFound
		}
		return 0, nil, err
	}
	insertedIDs := blocknote.CollectIDs(newBlocks)
	rev, _, err := s.persistBlocks(ctx, pageID, updated, expectedRevision, len(newBlocks))
	if err != nil {
		return 0, nil, err
	}
	return rev, insertedIDs, nil
}

func (s *DefaultDocumentService) DeleteBlock(ctx context.Context, pageID, blockID string, expectedRevision int64) (int64, error) {
	if err := auth.RequireScope(ctx, auth.ScopeArtifactsWrite); err != nil {
		return 0, err
	}
	if strings.TrimSpace(blockID) == "" {
		return 0, apperror.BadRequest("block_id is required")
	}
	current, _, err := s.store.GetPageBlocks(ctx, pageID)
	if err != nil {
		return 0, err
	}
	blocks, err := blocknote.ParseBlocks(current)
	if err != nil {
		return 0, err
	}
	if len(blocks) <= 1 {
		return 0, apperror.BadRequest("cannot delete the last block on a page; BlockNote requires at least one block")
	}
	updated, err := blocknote.DeleteByID(blocks, blockID)
	if err != nil {
		if errors.Is(err, blocknote.ErrBlockNotFound) {
			return 0, apperror.ErrNotFound
		}
		return 0, err
	}
	rev, _, err := s.persistBlocks(ctx, pageID, updated, expectedRevision, 0)
	return rev, err
}

func (s *DefaultDocumentService) persistBlocks(ctx context.Context, pageID string, blocks []blocknote.Block, expectedRevision int64, count int) (int64, int, error) {
	serialized, err := blocknote.SerializeBlocks(blocks)
	if err != nil {
		return 0, 0, err
	}
	searchText, err := blocknote.ExtractText(serialized)
	if err != nil {
		return 0, 0, err
	}
	rev, err := s.store.SavePageBlocks(ctx, pageID, serialized, searchText, expectedRevision)
	if err != nil {
		return 0, 0, err
	}
	return rev, count, nil
}

func validatePosition(p BlockPosition) error {
	set := 0
	if p.AfterID != nil {
		set++
	}
	if p.BeforeID != nil {
		set++
	}
	if p.At != nil {
		set++
		if *p.At != "start" && *p.At != "end" {
			return apperror.BadRequest("position.at must be \"start\" or \"end\"")
		}
	}
	if set != 1 {
		return apperror.BadRequest("position requires exactly one of after_id, before_id, or at")
	}
	return nil
}

func looksLikeJSONArray(raw json.RawMessage) bool {
	for _, b := range raw {
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		case '[':
			return true
		default:
			return false
		}
	}
	return false
}
