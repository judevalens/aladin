package app

import (
	"context"

	"aladin/backend_v2/internal/market/alpaca"
	coreservice "aladin/backend_v2/internal/service"
)

// Adapters bridging the Alpaca REST client to the service-layer read-through ports. They live
// in the app (wiring) package so the service layer stays free of a vendor import and alpaca
// stays free of a service import (no cycle).

type alpacaBarSource struct{ c *alpaca.Client }

func (a alpacaBarSource) FetchBars(ctx context.Context, symbol, timeframe, start, end string) ([]coreservice.Bar, error) {
	bars, err := a.c.GetBars(ctx, symbol, timeframe, start, end)
	if err != nil {
		return nil, err
	}
	out := make([]coreservice.Bar, 0, len(bars))
	for _, b := range bars {
		out = append(out, coreservice.Bar{Time: b.Time, Open: b.Open, High: b.High, Low: b.Low, Close: b.Close, Volume: b.Volume})
	}
	return out, nil
}

type alpacaSnapshotSource struct{ c *alpaca.Client }

func (a alpacaSnapshotSource) FetchSnapshot(ctx context.Context, symbol string) (coreservice.Quote, bool, error) {
	s, err := a.c.GetSnapshot(ctx, symbol)
	if err != nil {
		return coreservice.Quote{}, false, err
	}
	if s == nil {
		return coreservice.Quote{}, false, nil
	}
	last := s.LatestTrade.Price
	if last == 0 {
		last = s.DailyBar.Close
	}
	if last == 0 {
		return coreservice.Quote{}, false, nil
	}
	prev := s.PrevDailyBar.Close
	if prev == 0 {
		prev = s.DailyBar.Open
	}
	var change, pct float64
	if prev != 0 {
		change = last - prev
		pct = change / prev * 100
	}
	return coreservice.Quote{Last: last, PrevClose: prev, Change: change, ChangePct: pct, Ts: s.LatestTrade.Time}, true, nil
}

type alpacaAssetLookup struct{ c *alpaca.Client }

func (a alpacaAssetLookup) FetchInstrument(ctx context.Context, symbol string) (coreservice.InstrumentUpsert, bool, error) {
	as, err := a.c.GetAsset(ctx, symbol)
	if err != nil {
		return coreservice.InstrumentUpsert{}, false, err
	}
	if as == nil || !as.Tradable {
		return coreservice.InstrumentUpsert{}, false, nil
	}
	return coreservice.InstrumentUpsert{
		Symbol:     as.Symbol,
		Name:       as.Name,
		Exchange:   as.Exchange,
		AssetClass: as.Class,
		IsActive:   as.Status == "active",
	}, true, nil
}
