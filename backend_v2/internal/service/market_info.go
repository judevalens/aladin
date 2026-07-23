package service

import "context"

// MarketInfoService is the read-only market-intelligence surface backed by Alpaca:
// news/catalysts, the movers + most-actives screeners, and the (paper or live) trading
// account's balance + open positions. Nil when Alpaca isn't configured — callers degrade
// with a clear "not configured" error rather than a nil panic.
//
// Strictly read-only: positions/account expose state, never place or modify orders.
// Execution (T5) is a separate, gated surface.
type MarketInfoService interface {
	News(ctx context.Context, symbols string, limit int) ([]NewsArticle, error)
	Movers(ctx context.Context, top int) (MoversResult, error)
	MostActives(ctx context.Context, top int) ([]ActiveStock, error)
	Account(ctx context.Context) (AccountSummary, error)
	Positions(ctx context.Context) ([]PositionView, error)
}

type NewsArticle struct {
	ID        int64    `json:"id"`
	Headline  string   `json:"headline"`
	Summary   string   `json:"summary"`
	Source    string   `json:"source"`
	URL       string   `json:"url"`
	Symbols   []string `json:"symbols"`
	CreatedAt string   `json:"createdAt"`
}

type Mover struct {
	Symbol        string  `json:"symbol"`
	Price         float64 `json:"price"`
	Change        float64 `json:"change"`
	PercentChange float64 `json:"percentChange"`
}

type MoversResult struct {
	Gainers     []Mover `json:"gainers"`
	Losers      []Mover `json:"losers"`
	LastUpdated string  `json:"lastUpdated"`
}

type ActiveStock struct {
	Symbol     string `json:"symbol"`
	Volume     int64  `json:"volume"`
	TradeCount int64  `json:"tradeCount"`
}

type AccountSummary struct {
	Status          string `json:"status"`
	Currency        string `json:"currency"`
	Cash            string `json:"cash"`
	PortfolioValue  string `json:"portfolioValue"`
	Equity          string `json:"equity"`
	BuyingPower     string `json:"buyingPower"`
	LongMarketValue string `json:"longMarketValue"`
	// Paper reports whether this is a paper account (derived from the trading base URL),
	// so the copilot can caveat the numbers honestly.
	Paper bool `json:"paper"`
}

type PositionView struct {
	Symbol         string `json:"symbol"`
	Qty            string `json:"qty"`
	Side           string `json:"side"`
	AvgEntryPrice  string `json:"avgEntryPrice"`
	MarketValue    string `json:"marketValue"`
	CostBasis      string `json:"costBasis"`
	UnrealizedPL   string `json:"unrealizedPl"`
	UnrealizedPLPC string `json:"unrealizedPlPct"`
	CurrentPrice   string `json:"currentPrice"`
}
