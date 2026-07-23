// Package alpaca is a thin client for the Alpaca (alpaca.markets) trading + market-data
// APIs. It covers only what the trading substrate needs today — the Assets universe — and
// is the seam where historical bars (REST) and real-time quotes (the WS feed at
// wss://stream.data.alpaca.markets/v2/{feed}) slot in for T1+.
package alpaca

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Client talks to the Alpaca REST APIs. Key/secret authenticate both the trading and data
// APIs; only the base URL distinguishes them (and paper from live).
type Client struct {
	apiKey     string
	apiSecret  string
	tradingURL string
	dataURL    string
	http       *http.Client
}

func NewClient(apiKey, apiSecret, tradingBaseURL, dataBaseURL string) *Client {
	return &Client{
		apiKey:     apiKey,
		apiSecret:  apiSecret,
		tradingURL: tradingBaseURL,
		dataURL:    dataBaseURL,
		http:       &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) auth(req *http.Request) {
	req.Header.Set("APCA-API-KEY-ID", c.apiKey)
	req.Header.Set("APCA-API-SECRET-KEY", c.apiSecret)
}

// Bar is one OHLCV bar from the market-data API (fields we don't use — n, vw — dropped).
type Bar struct {
	Time   time.Time `json:"t"`
	Open   float64   `json:"o"`
	High   float64   `json:"h"`
	Low    float64   `json:"l"`
	Close  float64   `json:"c"`
	Volume int64     `json:"v"`
}

// snapBar / snapTrade are the pieces of a snapshot we use.
type snapBar struct {
	Open  float64 `json:"o"`
	High  float64 `json:"h"`
	Low   float64 `json:"l"`
	Close float64 `json:"c"`
	Vol   int64   `json:"v"`
	Time  string  `json:"t"`
}
type snapTrade struct {
	Price float64 `json:"p"`
	Time  string  `json:"t"`
}

// Snapshot is the current market picture for a symbol (last trade + today's + prior daily bar)
// — the "last known price" shown off-hours before live ticks arrive.
type Snapshot struct {
	LatestTrade  snapTrade `json:"latestTrade"`
	DailyBar     snapBar   `json:"dailyBar"`
	PrevDailyBar snapBar   `json:"prevDailyBar"`
}

// GetSnapshot fetches the current snapshot for a symbol (data API). Returns (nil, nil) on 404.
func (c *Client) GetSnapshot(ctx context.Context, symbol string) (*Snapshot, error) {
	u := fmt.Sprintf("%s/v2/stocks/%s/snapshot", c.dataURL, url.PathEscape(symbol))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("alpaca: build snapshot request: %w", err)
	}
	c.auth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("alpaca: snapshot request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alpaca: snapshot returned %s", resp.Status)
	}
	var s Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return nil, fmt.Errorf("alpaca: decode snapshot: %w", err)
	}
	return &s, nil
}

// GetBars fetches RAW (unadjusted) bars for a symbol/timeframe over [start,end], following
// page tokens. `timeframe` is Alpaca's form ("1Day", "1Hour", …); start/end are RFC-3339 or
// YYYY-MM-DD. Adjustment is deliberately "raw" — we store unadjusted and adjust on read.
func (c *Client) GetBars(ctx context.Context, symbol, timeframe, start, end string) ([]Bar, error) {
	base := fmt.Sprintf("%s/v2/stocks/%s/bars", c.dataURL, url.PathEscape(symbol))
	all := make([]Bar, 0, 256)
	pageToken := ""
	for {
		q := url.Values{}
		q.Set("timeframe", timeframe)
		q.Set("adjustment", "raw")
		q.Set("limit", "10000")
		if start != "" {
			q.Set("start", start)
		}
		if end != "" {
			q.Set("end", end)
		}
		if pageToken != "" {
			q.Set("page_token", pageToken)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"?"+q.Encode(), nil)
		if err != nil {
			return nil, fmt.Errorf("alpaca: build bars request: %w", err)
		}
		c.auth(req)
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("alpaca: bars request: %w", err)
		}
		var body struct {
			Bars          []Bar  `json:"bars"`
			NextPageToken string `json:"next_page_token"`
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("alpaca: bars returned %s", resp.Status)
		}
		err = json.NewDecoder(resp.Body).Decode(&body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("alpaca: decode bars: %w", err)
		}
		all = append(all, body.Bars...)
		if body.NextPageToken == "" {
			break
		}
		pageToken = body.NextPageToken
	}
	return all, nil
}

// Asset is one row of the Assets API — a tradeable security. Fields we don't use yet
// (margin, shortable, …) are dropped; add them when a consumer needs them.
type Asset struct {
	ID           string `json:"id"` // Alpaca's stable asset UUID (an external id, not our instrument_id)
	Class        string `json:"class"`
	Exchange     string `json:"exchange"`
	Symbol       string `json:"symbol"`
	Name         string `json:"name"`
	Status       string `json:"status"` // "active" | "inactive"
	Tradable     bool   `json:"tradable"`
	Fractionable bool   `json:"fractionable"`
}

// GetAsset fetches a single asset by symbol (for on-demand instrument resolution / cache fill).
// Returns (nil, nil) when the symbol is unknown (404).
func (c *Client) GetAsset(ctx context.Context, symbol string) (*Asset, error) {
	url := fmt.Sprintf("%s/v2/assets/%s", c.tradingURL, url.PathEscape(symbol))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("alpaca: build asset request: %w", err)
	}
	c.auth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("alpaca: asset request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alpaca: asset returned %s", resp.Status)
	}
	var a Asset
	if err := json.NewDecoder(resp.Body).Decode(&a); err != nil {
		return nil, fmt.Errorf("alpaca: decode asset: %w", err)
	}
	return &a, nil
}

// NewsItem is one Alpaca (Benzinga-sourced) news article. Content is often empty on the
// list endpoint; Summary + Headline are the reliable text.
type NewsItem struct {
	ID        int64    `json:"id"`
	Headline  string   `json:"headline"`
	Summary   string   `json:"summary"`
	Author    string   `json:"author"`
	Source    string   `json:"source"`
	URL       string   `json:"url"`
	Symbols   []string `json:"symbols"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
}

// GetNews fetches recent news, optionally filtered to symbols (comma-separated). limit is
// capped at 50 by Alpaca. Newest first.
func (c *Client) GetNews(ctx context.Context, symbols string, limit int) ([]NewsItem, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	q := url.Values{}
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("sort", "desc")
	if symbols != "" {
		q.Set("symbols", symbols)
	}
	u := fmt.Sprintf("%s/v1beta1/news?%s", c.dataURL, q.Encode())
	var body struct {
		News []NewsItem `json:"news"`
	}
	if err := c.getJSON(ctx, u, &body); err != nil {
		return nil, err
	}
	return body.News, nil
}

// Mover is one screener gainer/loser row.
type Mover struct {
	Symbol        string  `json:"symbol"`
	Price         float64 `json:"price"`
	Change        float64 `json:"change"`
	PercentChange float64 `json:"percent_change"`
}

// Movers is the screener's top gainers + losers.
type Movers struct {
	Gainers     []Mover `json:"gainers"`
	Losers      []Mover `json:"losers"`
	LastUpdated string  `json:"last_updated"`
}

// GetMovers fetches the top `top` gainers and losers (consolidated tape). top caps at 50.
func (c *Client) GetMovers(ctx context.Context, top int) (*Movers, error) {
	if top <= 0 || top > 50 {
		top = 10
	}
	u := fmt.Sprintf("%s/v1beta1/screener/stocks/movers?top=%d", c.dataURL, top)
	var m Movers
	if err := c.getJSON(ctx, u, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// ActiveStock is one most-actives row.
type ActiveStock struct {
	Symbol     string `json:"symbol"`
	Volume     int64  `json:"volume"`
	TradeCount int64  `json:"trade_count"`
}

// MostActives is the screener's highest-volume names.
type MostActives struct {
	MostActives []ActiveStock `json:"most_actives"`
	LastUpdated string        `json:"last_updated"`
}

// GetMostActives fetches the `top` most-active symbols by volume. top caps at 50.
func (c *Client) GetMostActives(ctx context.Context, top int) (*MostActives, error) {
	if top <= 0 || top > 50 {
		top = 10
	}
	u := fmt.Sprintf("%s/v1beta1/screener/stocks/most-actives?by=volume&top=%d", c.dataURL, top)
	var m MostActives
	if err := c.getJSON(ctx, u, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// Account is the trading account summary (numbers arrive as strings — kept verbatim).
type Account struct {
	Status           string `json:"status"`
	Currency         string `json:"currency"`
	Cash             string `json:"cash"`
	PortfolioValue   string `json:"portfolio_value"`
	Equity           string `json:"equity"`
	BuyingPower      string `json:"buying_power"`
	LongMarketValue  string `json:"long_market_value"`
	PatternDayTrader bool   `json:"pattern_day_trader"`
}

// GetAccount fetches the (paper or live, per tradingURL) account summary.
func (c *Client) GetAccount(ctx context.Context) (*Account, error) {
	u := fmt.Sprintf("%s/v2/account", c.tradingURL)
	var a Account
	if err := c.getJSON(ctx, u, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// Position is one open position (numeric fields are strings from Alpaca — kept verbatim).
type Position struct {
	Symbol         string `json:"symbol"`
	Qty            string `json:"qty"`
	Side           string `json:"side"`
	AvgEntryPrice  string `json:"avg_entry_price"`
	MarketValue    string `json:"market_value"`
	CostBasis      string `json:"cost_basis"`
	UnrealizedPL   string `json:"unrealized_pl"`
	UnrealizedPLPC string `json:"unrealized_plpc"`
	CurrentPrice   string `json:"current_price"`
}

// GetPositions fetches all open positions for the (paper or live) account.
func (c *Client) GetPositions(ctx context.Context) ([]Position, error) {
	u := fmt.Sprintf("%s/v2/positions", c.tradingURL)
	var ps []Position
	if err := c.getJSON(ctx, u, &ps); err != nil {
		return nil, err
	}
	return ps, nil
}

// getJSON is the shared GET+decode for the read-only info endpoints (auth, status check,
// decode). 404 is treated as an empty/absent result by decoding into the caller's zero value.
func (c *Client) getJSON(ctx context.Context, u string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("alpaca: build request: %w", err)
	}
	c.auth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("alpaca: request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("alpaca: %s returned %s", u, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("alpaca: decode: %w", err)
	}
	return nil
}

// ListAssets fetches the asset universe for an asset class (default us_equity), filtered
// by status ("active" is the common case). The Assets API returns the full list in one
// response — no pagination cursor — so this is a single request.
func (c *Client) ListAssets(ctx context.Context, assetClass, status string) ([]Asset, error) {
	if assetClass == "" {
		assetClass = "us_equity"
	}
	url := fmt.Sprintf("%s/v2/assets?asset_class=%s", c.tradingURL, assetClass)
	if status != "" {
		url += "&status=" + status
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("alpaca: build assets request: %w", err)
	}
	req.Header.Set("APCA-API-KEY-ID", c.apiKey)
	req.Header.Set("APCA-API-SECRET-KEY", c.apiSecret)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("alpaca: assets request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alpaca: assets returned %s", resp.Status)
	}

	var assets []Asset
	if err := json.NewDecoder(resp.Body).Decode(&assets); err != nil {
		return nil, fmt.Errorf("alpaca: decode assets: %w", err)
	}
	return assets, nil
}
