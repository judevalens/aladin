package service

import (
	"context"
	"errors"
	"strings"
)

// WatchlistItem is one tracked security, joined with its instrument identity for display.
type WatchlistItem struct {
	InstrumentID string `json:"instrumentId"`
	Symbol       string `json:"symbol"`
	Name         string `json:"name"`
	Exchange     string `json:"exchange"`
	AddedAt      string `json:"addedAt"`
}

// ErrInvalidWatchlistInput is returned for an empty user or instrument id.
var ErrInvalidWatchlistInput = errors.New("watchlist: user and instrument id are required")

// WatchlistRepository is the persistence port for the per-user watchlist.
type WatchlistRepository interface {
	ListWatchlist(ctx context.Context, userID string) ([]WatchlistItem, error)
	AddToWatchlist(ctx context.Context, userID, instrumentID string) error
	RemoveFromWatchlist(ctx context.Context, userID, instrumentID string) error
}

// WatchlistService backs the Markets surface: the tickers a user is tracking.
type WatchlistService interface {
	List(ctx context.Context, userID string) ([]WatchlistItem, error)
	Add(ctx context.Context, userID, instrumentID string) error
	Remove(ctx context.Context, userID, instrumentID string) error
}

type defaultWatchlistService struct {
	repo WatchlistRepository
}

// NewWatchlistService returns the service; concrete impl stays unexported.
func NewWatchlistService(repo WatchlistRepository) WatchlistService {
	return &defaultWatchlistService{repo: repo}
}

func (s *defaultWatchlistService) List(ctx context.Context, userID string) ([]WatchlistItem, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, ErrInvalidWatchlistInput
	}
	items, err := s.repo.ListWatchlist(ctx, userID)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []WatchlistItem{}
	}
	return items, nil
}

func (s *defaultWatchlistService) Add(ctx context.Context, userID, instrumentID string) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(instrumentID) == "" {
		return ErrInvalidWatchlistInput
	}
	return s.repo.AddToWatchlist(ctx, userID, instrumentID)
}

func (s *defaultWatchlistService) Remove(ctx context.Context, userID, instrumentID string) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(instrumentID) == "" {
		return ErrInvalidWatchlistInput
	}
	return s.repo.RemoveFromWatchlist(ctx, userID, instrumentID)
}
