package instrument

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
	// ResolveInstrumentID maps an active symbol to its stable instrument_id.
	ResolveInstrumentID(ctx context.Context, symbol string) (string, bool, error)
}

// AssetSource is a reference-data provider (Alpaca) that lists the tradeable universe.
// Kept as an interface so the sync is testable without a live vendor / API keys.
type AssetSource interface {
	FetchInstruments(ctx context.Context) ([]InstrumentUpsert, error)
}

// AssetLookup fetches ONE instrument by symbol — the read-through path for ResolveInstrumentID
// on a cache miss (found=false ⇒ unknown symbol). Optional; nil ⇒ local-only resolution.
type AssetLookup interface {
	FetchInstrument(ctx context.Context, symbol string) (InstrumentUpsert, bool, error)
}

// InstrumentService backs ticker search (the command-box typeahead) and, later, the
// watchlist/chart surfaces. Search is read-only; SyncAssets ingests reference data.
type InstrumentService interface {
	Search(ctx context.Context, query string, limit int) ([]InstrumentHit, error)
	// SyncAssets pulls the universe from an AssetSource and upserts it; returns rows written.
	SyncAssets(ctx context.Context, src AssetSource) (int, error)
	// ResolveInstrumentID maps an active symbol to its stable instrument_id (ok=false if unknown).
	ResolveInstrumentID(ctx context.Context, symbol string) (string, bool, error)
}

const (
	instrumentDefaultLimit = 20
	instrumentMaxLimit     = 50
)

type defaultInstrumentService struct {
	repo   InstrumentRepository
	lookup AssetLookup // optional vendor fallback for on-demand resolve
}

// NewInstrumentService returns the service. Concrete type so WithAssetLookup can chain; it
// implements InstrumentService, so DI still exposes the interface.
func NewInstrumentService(repo InstrumentRepository) *defaultInstrumentService {
	return &defaultInstrumentService{repo: repo}
}

// WithAssetLookup enables read-through resolve: on a symbol miss, fetch the single asset from
// the vendor and upsert it before re-resolving.
func (s *defaultInstrumentService) WithAssetLookup(l AssetLookup) *defaultInstrumentService {
	s.lookup = l
	return s
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

func (s *defaultInstrumentService) ResolveInstrumentID(ctx context.Context, symbol string) (string, bool, error) {
	if strings.TrimSpace(symbol) == "" {
		return "", false, nil
	}
	id, ok, err := s.repo.ResolveInstrumentID(ctx, symbol)
	if err != nil || ok || s.lookup == nil {
		return id, ok, err
	}
	// Cache miss → fetch the single asset from the vendor, upsert, re-resolve (read-through).
	up, found, err := s.lookup.FetchInstrument(ctx, symbol)
	if err != nil || !found {
		return "", false, err
	}
	if _, err := s.repo.UpsertInstruments(ctx, []InstrumentUpsert{up}); err != nil {
		return "", false, err
	}
	return s.repo.ResolveInstrumentID(ctx, symbol)
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
