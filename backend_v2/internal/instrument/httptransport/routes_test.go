package httptransport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aladin/backend_v2/internal/instrument"
	"aladin/backend_v2/internal/market"
)

type instrumentServiceStub struct{ query string }

func (s *instrumentServiceStub) Search(_ context.Context, query string, _ int) ([]instrument.InstrumentHit, error) {
	s.query = query
	return []instrument.InstrumentHit{{ID: "i1", Symbol: "AAPL"}}, nil
}
func (*instrumentServiceStub) SyncAssets(context.Context, instrument.AssetSource) (int, error) {
	return 0, nil
}
func (*instrumentServiceStub) ResolveInstrumentID(context.Context, string) (string, bool, error) {
	return "", false, nil
}

type barReaderStub struct{ symbol string }

func (s *barReaderStub) Get(_ context.Context, symbol, _ string, _ int) ([]market.Bar, error) {
	s.symbol = symbol
	return []market.Bar{{Close: 200}}, nil
}

func TestRegisterPreservesInstrumentRoutes(t *testing.T) {
	instruments := &instrumentServiceStub{}
	bars := &barReaderStub{}
	mux := http.NewServeMux()
	Register(mux, instruments, bars)

	searchResponse := httptest.NewRecorder()
	mux.ServeHTTP(searchResponse, httptest.NewRequest(http.MethodGet, "/api/instruments/search?q=apple&limit=4", nil))
	if searchResponse.Code != http.StatusOK || !strings.Contains(searchResponse.Body.String(), `"symbol":"AAPL"`) {
		t.Fatalf("search status=%d body=%s", searchResponse.Code, searchResponse.Body.String())
	}

	barResponse := httptest.NewRecorder()
	mux.ServeHTTP(barResponse, httptest.NewRequest(http.MethodGet, "/api/instruments/AAPL/bars?timeframe=1Day&limit=30", nil))
	if barResponse.Code != http.StatusOK || !strings.Contains(barResponse.Body.String(), `"c":200`) {
		t.Fatalf("bars status=%d body=%s", barResponse.Code, barResponse.Body.String())
	}
	if instruments.query != "apple" || bars.symbol != "AAPL" {
		t.Fatalf("adapter arguments changed: query=%q symbol=%q", instruments.query, bars.symbol)
	}
}
