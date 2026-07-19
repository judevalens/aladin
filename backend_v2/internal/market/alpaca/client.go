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
	"time"
)

// Client talks to the Alpaca REST APIs. Key/secret authenticate both the trading and data
// APIs; only the base URL distinguishes them (and paper from live).
type Client struct {
	apiKey     string
	apiSecret  string
	tradingURL string
	http       *http.Client
}

func NewClient(apiKey, apiSecret, tradingBaseURL string) *Client {
	return &Client{
		apiKey:     apiKey,
		apiSecret:  apiSecret,
		tradingURL: tradingBaseURL,
		http:       &http.Client{Timeout: 30 * time.Second},
	}
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
