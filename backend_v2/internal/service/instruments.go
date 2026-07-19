package service

import (
	"context"
	"strings"
)

// InstrumentHit is one row of the ticker typeahead — a security (NVDA), not a company.
// Deliberately thin: symbol + name is what the command-box picker renders. The entity
// bridge / bars / fundamentals hang off later phases (project_trading_entity_data_model).
type InstrumentHit struct {
	ID         string `json:"id"`
	Symbol     string `json:"symbol"`
	Name       string `json:"name"`
	Exchange   string `json:"exchange"`
	AssetClass string `json:"assetClass"`
	IsActive   bool   `json:"isActive"`
}

// InstrumentUpsert is one row to write into the registry from a reference-data source
// (e.g. the Alpaca Assets API). Keyed on Symbol (the active-listing unique index).
type InstrumentUpsert struct {
	Symbol     string
	Name       string
	Exchange   string
	AssetClass string
	IsActive   bool
}

// InstrumentRepository is the persistence port for the instrument registry.
type InstrumentRepository interface {
	SearchInstruments(ctx context.Context, query string, limit int) ([]InstrumentHit, error)
	// UpsertInstruments writes reference data idempotently; returns rows affected.
	UpsertInstruments(ctx context.Context, rows []InstrumentUpsert) (int, error)
}

// AssetSource is a reference-data provider (Alpaca) that lists the tradeable universe.
// Kept as an interface so the sync is testable without a live vendor / API keys.
type AssetSource interface {
	FetchInstruments(ctx context.Context) ([]InstrumentUpsert, error)
}

// InstrumentService backs ticker search (the command-box typeahead) and, later, the
// watchlist/chart surfaces. Search is read-only; SyncAssets ingests reference data.
type InstrumentService interface {
	Search(ctx context.Context, query string, limit int) ([]InstrumentHit, error)
	// SyncAssets pulls the universe from an AssetSource and upserts it; returns rows written.
	SyncAssets(ctx context.Context, src AssetSource) (int, error)
}

const (
	instrumentDefaultLimit = 20
	instrumentMaxLimit     = 50
)

type defaultInstrumentService struct {
	repo InstrumentRepository
}

// NewInstrumentService returns the InstrumentService; the concrete impl stays unexported
// (DI exposes the interface, per the repo's clean-layering convention).
func NewInstrumentService(repo InstrumentRepository) InstrumentService {
	return &defaultInstrumentService{repo: repo}
}

func (s *defaultInstrumentService) Search(ctx context.Context, query string, limit int) ([]InstrumentHit, error) {
	if limit <= 0 {
		limit = instrumentDefaultLimit
	}
	if limit > instrumentMaxLimit {
		limit = instrumentMaxLimit
	}
	if strings.TrimSpace(query) == "" {
		return []InstrumentHit{}, nil
	}
	return s.repo.SearchInstruments(ctx, query, limit)
}

func (s *defaultInstrumentService) SyncAssets(ctx context.Context, src AssetSource) (int, error) {
	rows, err := src.FetchInstruments(ctx)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return s.repo.UpsertInstruments(ctx, rows)
}
