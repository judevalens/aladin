package service

import (
	"context"
	"strings"
)

type RecordService interface {
	List(context.Context) ([]RecordResponse, error)
	Children(context.Context, string, int, int) (map[string]any, error)
	Create(context.Context, string, string, string, string, string, string) error
	Delete(context.Context, string) error
	// Retry re-drives a 'failed' record. Returns false if it wasn't failed.
	Retry(context.Context, string) (bool, error)
	// Similar returns records whose embedding is closest to the given record's (the vector
	// "similar sources" lens). Empty if the record has no embedding yet.
	Similar(ctx context.Context, id string, limit int) ([]SimilarRecord, error)
}

type RecordRepository interface {
	List(context.Context) ([]RecordResponse, error)
	Children(context.Context, string, int, int) (map[string]any, error)
	Create(context.Context, string, string, string, string, string, string) error
	Delete(context.Context, string) error
	ResetForRetry(context.Context, string) (bool, error)
	SimilarRecords(ctx context.Context, id string, limit int) ([]SimilarRecord, error)
}

// SimilarRecord is a vector-near record (cosine similarity 0..1).
type SimilarRecord struct {
	ID        string  `json:"id"`
	Label     string  `json:"label"`
	SourceURL string  `json:"sourceUrl"`
	Provider  string  `json:"provider"`
	Cosine    float64 `json:"cosine"`
}

const similarRecordsMaxLimit = 50

type RecordResponse struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Label      string         `json:"label"`
	Content    string         `json:"content"`
	SourceURL  *string        `json:"sourceUrl"`
	ParentID   *string        `json:"parentId"`
	Enrichment any            `json:"enrichment"`
	Metadata   map[string]any `json:"metadata"`
	ChildCount int            `json:"childCount"`
	CreatedAt  string         `json:"createdAt"`
}

type DefaultRecordService struct{ repo RecordRepository }

func NewRecordService(repo RecordRepository) *DefaultRecordService {
	return &DefaultRecordService{repo: repo}
}

func (s *DefaultRecordService) List(ctx context.Context) ([]RecordResponse, error) {
	return s.repo.List(ctx)
}

func (s *DefaultRecordService) Children(ctx context.Context, id string, limit int, offset int) (map[string]any, error) {
	return s.repo.Children(ctx, id, limit, offset)
}

func (s *DefaultRecordService) Create(ctx context.Context, id string, kind string, label string, content string, sourceURL string, parentID string) error {
	if strings.TrimSpace(id) == "" {
		id = newID("record-")
	}
	return s.repo.Create(ctx, id, kind, label, content, sourceURL, parentID)
}

func (s *DefaultRecordService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *DefaultRecordService) Retry(ctx context.Context, id string) (bool, error) {
	return s.repo.ResetForRetry(ctx, id)
}

func (s *DefaultRecordService) Similar(ctx context.Context, id string, limit int) ([]SimilarRecord, error) {
	if limit <= 0 || limit > similarRecordsMaxLimit {
		limit = 10
	}
	return s.repo.SimilarRecords(ctx, id, limit)
}
