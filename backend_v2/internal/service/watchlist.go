package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
)

// WatchlistItem is one tracked security, joined with its instrument identity for display.
type WatchlistItem struct {
	InstrumentID string `json:"instrumentId"`
	Symbol       string `json:"symbol"`
	Name         string `json:"name"`
	Exchange     string `json:"exchange"`
	AddedAt      string `json:"addedAt"`
}

// Watchlist kinds — how the universe's membership resolves.
const (
	WatchlistManual   = "manual"   // explicit member rows
	WatchlistScreener = "screener" // a stored rule (definition), resolved at read time (v2)
	WatchlistHybrid   = "hybrid"   // manual picks ∪ rule (v2)
)

// Watchlist is a named instrument set — a UNIVERSE the trading engine consumes/produces. Kind
// determines how it resolves to instruments (see ResolveInstruments).
type Watchlist struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Kind       string          `json:"kind"`
	Definition json.RawMessage `json:"definition,omitempty"`
	Position   int64           `json:"position"`
	ItemCount  int             `json:"itemCount"`
	CreatedAt  string          `json:"createdAt"`
}

var (
	// ErrInvalidWatchlistInput is returned for an empty user / instrument id / name.
	ErrInvalidWatchlistInput = errors.New("watchlist: user, instrument id, or name is required")
	// ErrWatchlistNotFound is returned when a list isn't found for the user (rename/delete).
	ErrWatchlistNotFound = errors.New("watchlist: list not found")
	// ErrScreenerNotImplemented marks the reserved dynamic-resolution path (v2/T3).
	ErrScreenerNotImplemented = errors.New("watchlist: screener resolution is not implemented yet")
)

// WatchlistRepository is the persistence port. List-scoped; ownership is enforced in the WHERE.
type WatchlistRepository interface {
	ListWatchlists(ctx context.Context, userID string) ([]Watchlist, error)
	CreateWatchlist(ctx context.Context, w Watchlist, userID string) (Watchlist, error)
	RenameWatchlist(ctx context.Context, userID, id, name string) error
	DeleteWatchlist(ctx context.Context, userID, id string) error
	GetWatchlist(ctx context.Context, userID, id string) (Watchlist, bool, error)
	// DefaultWatchlistID returns the user's lowest-position list (false if none).
	DefaultWatchlistID(ctx context.Context, userID string) (string, bool, error)
	ListItems(ctx context.Context, userID, listID string) ([]WatchlistItem, error)
	AddItem(ctx context.Context, userID, listID, instrumentID string) error
	RemoveItem(ctx context.Context, userID, listID, instrumentID string) error
}

// WatchlistService backs the Markets surface AND is the universe primitive for the trading engine.
type WatchlistService interface {
	ListWatchlists(ctx context.Context, userID string) ([]Watchlist, error)
	CreateWatchlist(ctx context.Context, userID, name string) (Watchlist, error)
	RenameWatchlist(ctx context.Context, userID, id, name string) error
	DeleteWatchlist(ctx context.Context, userID, id string) error
	// ListItems returns a list's members; listID "" resolves to the user's default list.
	ListItems(ctx context.Context, userID, listID string) ([]WatchlistItem, error)
	AddItem(ctx context.Context, userID, listID, instrumentID string) error
	RemoveItem(ctx context.Context, userID, listID, instrumentID string) error
	// ResolveOrCreateByName maps a list name to its id (create-if-new); "" → default. For the copilot.
	ResolveOrCreateByName(ctx context.Context, userID, name string) (string, error)
	// ResolveInstruments is the UNIVERSE port the trading engine consumes: resolve a list to its
	// instruments, dispatching on kind. manual → member rows; screener/hybrid → v2 (ErrScreener…).
	ResolveInstruments(ctx context.Context, userID, listID string) ([]WatchlistItem, error)

	// Backward-compat shims onto the default list (keep MCP/older callers compiling).
	List(ctx context.Context, userID string) ([]WatchlistItem, error)
	Add(ctx context.Context, userID, instrumentID string) error
	Remove(ctx context.Context, userID, instrumentID string) error
}

type defaultWatchlistService struct {
	repo WatchlistRepository
}

func NewWatchlistService(repo WatchlistRepository) WatchlistService {
	return &defaultWatchlistService{repo: repo}
}

func (s *defaultWatchlistService) ListWatchlists(ctx context.Context, userID string) ([]Watchlist, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, ErrInvalidWatchlistInput
	}
	lists, err := s.repo.ListWatchlists(ctx, userID)
	if err != nil {
		return nil, err
	}
	if lists == nil {
		lists = []Watchlist{}
	}
	return lists, nil
}

func (s *defaultWatchlistService) CreateWatchlist(ctx context.Context, userID, name string) (Watchlist, error) {
	userID = strings.TrimSpace(userID)
	name = strings.TrimSpace(name)
	if userID == "" || name == "" {
		return Watchlist{}, ErrInvalidWatchlistInput
	}
	// Next position = current list count (append at the end).
	existing, err := s.repo.ListWatchlists(ctx, userID)
	if err != nil {
		return Watchlist{}, err
	}
	w := Watchlist{
		ID:       uuid.NewString(),
		Name:     name,
		Kind:     WatchlistManual,
		Position: int64(len(existing)),
	}
	return s.repo.CreateWatchlist(ctx, w, userID)
}

func (s *defaultWatchlistService) RenameWatchlist(ctx context.Context, userID, id, name string) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(id) == "" || strings.TrimSpace(name) == "" {
		return ErrInvalidWatchlistInput
	}
	return s.repo.RenameWatchlist(ctx, userID, id, strings.TrimSpace(name))
}

func (s *defaultWatchlistService) DeleteWatchlist(ctx context.Context, userID, id string) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(id) == "" {
		return ErrInvalidWatchlistInput
	}
	// Deleting the last list is allowed — the default is lazily recreated on the next resolve.
	return s.repo.DeleteWatchlist(ctx, userID, id)
}

func (s *defaultWatchlistService) ListItems(ctx context.Context, userID, listID string) ([]WatchlistItem, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, ErrInvalidWatchlistInput
	}
	id, err := s.resolveListID(ctx, userID, listID)
	if err != nil {
		return nil, err
	}
	items, err := s.repo.ListItems(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []WatchlistItem{}
	}
	return items, nil
}

func (s *defaultWatchlistService) AddItem(ctx context.Context, userID, listID, instrumentID string) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(instrumentID) == "" {
		return ErrInvalidWatchlistInput
	}
	id, err := s.resolveListID(ctx, userID, listID)
	if err != nil {
		return err
	}
	return s.repo.AddItem(ctx, userID, id, instrumentID)
}

func (s *defaultWatchlistService) RemoveItem(ctx context.Context, userID, listID, instrumentID string) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(instrumentID) == "" {
		return ErrInvalidWatchlistInput
	}
	id, err := s.resolveListID(ctx, userID, listID)
	if err != nil {
		return err
	}
	return s.repo.RemoveItem(ctx, userID, id, instrumentID)
}

// ResolveInstruments — the universe port. Dispatches on kind.
func (s *defaultWatchlistService) ResolveInstruments(ctx context.Context, userID, listID string) ([]WatchlistItem, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, ErrInvalidWatchlistInput
	}
	id, err := s.resolveListID(ctx, userID, listID)
	if err != nil {
		return nil, err
	}
	w, ok, err := s.repo.GetWatchlist(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrWatchlistNotFound
	}
	switch w.Kind {
	case WatchlistManual:
		return s.repo.ListItems(ctx, userID, id)
	case WatchlistScreener, WatchlistHybrid:
		// Reserved: a screener resolves w.Definition against instruments+bars (v2/T3). The seam
		// exists so strategies/scans can already reference any watchlist as a universe.
		return nil, ErrScreenerNotImplemented
	default:
		return s.repo.ListItems(ctx, userID, id)
	}
}

func (s *defaultWatchlistService) ResolveOrCreateByName(ctx context.Context, userID, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return s.resolveListID(ctx, userID, "")
	}
	lists, err := s.repo.ListWatchlists(ctx, userID)
	if err != nil {
		return "", err
	}
	for _, l := range lists {
		if strings.EqualFold(l.Name, name) {
			return l.ID, nil
		}
	}
	created, err := s.CreateWatchlist(ctx, userID, name)
	if err != nil {
		return "", err
	}
	return created.ID, nil
}

// resolveListID returns the given list id, or the user's default (lazily creating one if the user
// has no lists yet — e.g. a brand-new user, or after deleting their last list).
func (s *defaultWatchlistService) resolveListID(ctx context.Context, userID, listID string) (string, error) {
	if id := strings.TrimSpace(listID); id != "" {
		return id, nil
	}
	id, ok, err := s.repo.DefaultWatchlistID(ctx, userID)
	if err != nil {
		return "", err
	}
	if ok {
		return id, nil
	}
	created, err := s.CreateWatchlist(ctx, userID, "Watchlist")
	if err != nil {
		return "", err
	}
	return created.ID, nil
}

// --- backward-compat shims (default list) ---------------------------------------------------

func (s *defaultWatchlistService) List(ctx context.Context, userID string) ([]WatchlistItem, error) {
	return s.ListItems(ctx, userID, "")
}
func (s *defaultWatchlistService) Add(ctx context.Context, userID, instrumentID string) error {
	return s.AddItem(ctx, userID, "", instrumentID)
}
func (s *defaultWatchlistService) Remove(ctx context.Context, userID, instrumentID string) error {
	return s.RemoveItem(ctx, userID, "", instrumentID)
}
