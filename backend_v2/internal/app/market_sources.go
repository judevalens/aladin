package app

import (
	"context"

	"aladin/backend_v2/internal/instrument"
	"aladin/backend_v2/internal/market"
	"aladin/backend_v2/internal/market/alpaca"
)

// Adapters bridging the Alpaca REST client to the service-layer read-through ports. They live
// in the app (wiring) package so the service layer stays free of a vendor import and alpaca
// stays free of a service import (no cycle).

// alpacaMarketInfo adapts the Alpaca client to service.MarketInfoService (news + screeners
// + read-only account/positions). paper is derived from the trading base URL so the copilot
// can caveat the account numbers.
type alpacaMarketInfo struct {
	c     *alpaca.Client
	paper bool
}

func (a alpacaMarketInfo) News(ctx context.Context, symbols string, limit int) ([]market.NewsArticle, error) {
	items, err := a.c.GetNews(ctx, symbols, limit)
	if err != nil {
		return nil, err
	}
	out := make([]market.NewsArticle, 0, len(items))
	for _, n := range items {
		out = append(out, market.NewsArticle{
			ID: n.ID, Headline: n.Headline, Summary: n.Summary, Source: n.Source,
			URL: n.URL, Symbols: n.Symbols, CreatedAt: n.CreatedAt,
		})
	}
	return out, nil
}

func (a alpacaMarketInfo) Movers(ctx context.Context, top int) (market.MoversResult, error) {
	m, err := a.c.GetMovers(ctx, top)
	if err != nil {
		return market.MoversResult{}, err
	}
	conv := func(in []alpaca.Mover) []market.Mover {
		out := make([]market.Mover, 0, len(in))
		for _, v := range in {
			out = append(out, market.Mover{Symbol: v.Symbol, Price: v.Price, Change: v.Change, PercentChange: v.PercentChange})
		}
		return out
	}
	return market.MoversResult{Gainers: conv(m.Gainers), Losers: conv(m.Losers), LastUpdated: m.LastUpdated}, nil
}

func (a alpacaMarketInfo) MostActives(ctx context.Context, top int) ([]market.ActiveStock, error) {
	m, err := a.c.GetMostActives(ctx, top)
	if err != nil {
		return nil, err
	}
	out := make([]market.ActiveStock, 0, len(m.MostActives))
	for _, v := range m.MostActives {
		out = append(out, market.ActiveStock{Symbol: v.Symbol, Volume: v.Volume, TradeCount: v.TradeCount})
	}
	return out, nil
}

func (a alpacaMarketInfo) Account(ctx context.Context) (market.AccountSummary, error) {
	acc, err := a.c.GetAccount(ctx)
	if err != nil {
		return market.AccountSummary{}, err
	}
	return market.AccountSummary{
		Status: acc.Status, Currency: acc.Currency, Cash: acc.Cash,
		PortfolioValue: acc.PortfolioValue, Equity: acc.Equity, BuyingPower: acc.BuyingPower,
		LongMarketValue: acc.LongMarketValue, Paper: a.paper,
	}, nil
}

func (a alpacaMarketInfo) Positions(ctx context.Context) ([]market.PositionView, error) {
	ps, err := a.c.GetPositions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]market.PositionView, 0, len(ps))
	for _, p := range ps {
		out = append(out, market.PositionView{
			Symbol: p.Symbol, Qty: p.Qty, Side: p.Side, AvgEntryPrice: p.AvgEntryPrice,
			MarketValue: p.MarketValue, CostBasis: p.CostBasis, UnrealizedPL: p.UnrealizedPL,
			UnrealizedPLPC: p.UnrealizedPLPC, CurrentPrice: p.CurrentPrice,
		})
	}
	return out, nil
}

type alpacaBarSource struct{ c *alpaca.Client }

func (a alpacaBarSource) FetchBars(ctx context.Context, symbol, timeframe, start, end string) ([]market.Bar, error) {
	bars, err := a.c.GetBars(ctx, symbol, timeframe, start, end)
	if err != nil {
		return nil, err
	}
	out := make([]market.Bar, 0, len(bars))
	for _, b := range bars {
		out = append(out, market.Bar{Time: b.Time, Open: b.Open, High: b.High, Low: b.Low, Close: b.Close, Volume: b.Volume})
	}
	return out, nil
}

type alpacaSnapshotSource struct{ c *alpaca.Client }

func (a alpacaSnapshotSource) FetchSnapshot(ctx context.Context, symbol string) (market.Quote, bool, error) {
	s, err := a.c.GetSnapshot(ctx, symbol)
	if err != nil {
		return market.Quote{}, false, err
	}
	if s == nil {
		return market.Quote{}, false, nil
	}
	last := s.LatestTrade.Price
	if last == 0 {
		last = s.DailyBar.Close
	}
	if last == 0 {
		return market.Quote{}, false, nil
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
	return market.Quote{Last: last, PrevClose: prev, Change: change, ChangePct: pct, Ts: s.LatestTrade.Time}, true, nil
}

type alpacaAssetLookup struct{ c *alpaca.Client }

func (a alpacaAssetLookup) FetchInstrument(ctx context.Context, symbol string) (instrument.InstrumentUpsert, bool, error) {
	as, err := a.c.GetAsset(ctx, symbol)
	if err != nil {
		return instrument.InstrumentUpsert{}, false, err
	}
	if as == nil || !as.Tradable {
		return instrument.InstrumentUpsert{}, false, nil
	}
	return instrument.InstrumentUpsert{
		Symbol:     as.Symbol,
		Name:       as.Name,
		Exchange:   as.Exchange,
		AssetClass: as.Class,
		IsActive:   as.Status == "active",
	}, true, nil
}

type alpacaCorporateActionSource struct{ c *alpaca.Client }

// FetchCorporateActions adapts Alpaca's splits/dividends to the service's action log.
func (a alpacaCorporateActionSource) FetchCorporateActions(ctx context.Context, symbol, start, end string) ([]market.CorporateAction, error) {
	acts, err := a.c.GetCorporateActions(ctx, symbol, start, end)
	if err != nil {
		return nil, err
	}
	out := make([]market.CorporateAction, 0, len(acts))
	for _, x := range acts {
		out = append(out, market.CorporateAction{
			Type: x.Type, ExDate: x.ExDate, SplitRatio: x.Ratio, CashAmount: x.Cash,
		})
	}
	return out, nil
}

// NewAlpacaCorporateActionSource exposes the corporate-actions feed to the backfill command.
func NewAlpacaCorporateActionSource(c *alpaca.Client) market.CorporateActionSource {
	return alpacaCorporateActionSource{c: c}
}
