package service

import (
	"context"
	"encoding/json"
	"errors"

	"aladin/backend_v2/internal/blocknote"
)

// PageDocumentService is the only service callers should reach for to read or
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
type PageDocumentService interface {
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

// PageDocumentStore is the minimal storage surface PageDocumentService needs.
// A concrete implementation lives in the repo package; tests stub it.
type PageDocumentStore interface {
	GetPageBlocks(ctx context.Context, pageID string) (blocks json.RawMessage, revision int64, err error)
	SavePageBlocks(ctx context.Context, pageID string, blocks json.RawMessage, searchText string, expectedRevision int64) (newRevision int64, err error)
}

// DefaultPageDocumentService is the production implementation backed by
// PageDocumentStore. The store does the SQL; this layer owns the search_text
// derivation, revision arithmetic, and (eventually) the block-ops splicing.
type DefaultPageDocumentService struct {
	store PageDocumentStore
}

func NewPageDocumentService(store PageDocumentStore) *DefaultPageDocumentService {
	return &DefaultPageDocumentService{store: store}
}

func (s *DefaultPageDocumentService) GetBlocks(ctx context.Context, pageID string) (json.RawMessage, int64, error) {
	if err := RequireScope(ctx, ScopeArtifactsRead); err != nil {
		return nil, 0, err
	}
	return s.store.GetPageBlocks(ctx, pageID)
}

func (s *DefaultPageDocumentService) ReplaceAll(ctx context.Context, pageID string, blocks json.RawMessage, expectedRevision int64) (int64, error) {
	if err := RequireScope(ctx, ScopeArtifactsWrite); err != nil {
		return 0, err
	}
	if len(blocks) == 0 || !looksLikeJSONArray(blocks) {
		return 0, BadRequest("blocks must be a JSON array")
	}
	searchText, err := blocknote.ExtractText(blocks)
	if err != nil {
		return 0, BadRequest("blocks: " + err.Error())
	}
	return s.store.SavePageBlocks(ctx, pageID, blocks, searchText, expectedRevision)
}

func (s *DefaultPageDocumentService) ReplaceBlock(ctx context.Context, pageID, blockID string, replacement json.RawMessage, expectedRevision int64) (int64, int, error) {
	return 0, 0, ErrNotImplemented
}

func (s *DefaultPageDocumentService) InsertBlocks(ctx context.Context, pageID string, position BlockPosition, blocks json.RawMessage, expectedRevision int64) (int64, []string, error) {
	return 0, nil, ErrNotImplemented
}

func (s *DefaultPageDocumentService) DeleteBlock(ctx context.Context, pageID, blockID string, expectedRevision int64) (int64, error) {
	return 0, ErrNotImplemented
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
