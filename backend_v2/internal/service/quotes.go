package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Quote is a live price update — the payload of a broadcast 'market' quote app_event. Kept
// small (a tick, not a snapshot); the frontend overlays it onto its Quote view.
type Quote struct {
	Symbol       string  `json:"symbol"`
	InstrumentID string  `json:"instrumentId"`
	Last         float64 `json:"last"`
	Change       float64 `json:"change,omitempty"`
	ChangePct    float64 `json:"changePct,omitempty"`
	// PrevClose anchors change for later WS ticks (which carry only Last). Set on the snapshot seed.
	PrevClose float64 `json:"prevClose,omitempty"`
	// Ts is the tick time, RFC-3339. Set by the producer.
	Ts string `json:"ts,omitempty"`
}

// QuoteSnapshotSource fetches the current snapshot (last price + prev close) for a symbol — the
// seed shown before live ticks. Optional; nil ⇒ no seeding (placeholder shows until first tick).
type QuoteSnapshotSource interface {
	FetchSnapshot(ctx context.Context, symbol string) (Quote, bool, error)
}

// QuoteOutbox is the write port: append a quote to the transactional outbox as a broadcast
// 'market' app_event. Satisfied by repo.SyncRepo.AppendMarketQuote (no db import cycle).
type QuoteOutbox interface {
	AppendMarketQuote(ctx context.Context, instrumentID string, payload []byte) error
}

// QuoteService publishes live quotes onto the outbox; the drainer fans them out to every
// subscriber of the broadcast market stream.
type QuoteService interface {
	Publish(ctx context.Context, q Quote) error
}

type defaultQuoteService struct {
	outbox QuoteOutbox
}

func NewQuoteService(outbox QuoteOutbox) QuoteService {
	return &defaultQuoteService{outbox: outbox}
}

func (s *defaultQuoteService) Publish(ctx context.Context, q Quote) error {
	if strings.TrimSpace(q.InstrumentID) == "" {
		return fmt.Errorf("quote: instrument id required")
	}
	payload, err := json.Marshal(q)
	if err != nil {
		return fmt.Errorf("quote: marshal: %w", err)
	}
	return s.outbox.AppendMarketQuote(ctx, q.InstrumentID, payload)
}
