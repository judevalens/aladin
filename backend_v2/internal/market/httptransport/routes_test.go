package httptransport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type marketDataStub struct {
	subscribed   []string
	unsubscribed []string
}

func (s *marketDataStub) Subscribe(_ context.Context, symbols []string) error {
	s.subscribed = append([]string(nil), symbols...)
	return nil
}

func (s *marketDataStub) Unsubscribe(_ context.Context, symbols []string) error {
	s.unsubscribed = append([]string(nil), symbols...)
	return nil
}

func (*marketDataStub) Start(context.Context) {}

func TestRegisterPreservesMarketSubscriptionRoutes(t *testing.T) {
	service := &marketDataStub{}
	mux := http.NewServeMux()
	Register(mux, service)

	for _, tc := range []struct {
		path string
		got  *[]string
	}{
		{path: "/api/market/subscribe", got: &service.subscribed},
		{path: "/api/market/unsubscribe", got: &service.unsubscribed},
	} {
		req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(`{"symbols":["AAPL","NVDA"]}`))
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, req)

		if res.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d; body=%s", tc.path, res.Code, http.StatusOK, res.Body.String())
		}
		if len(*tc.got) != 2 || (*tc.got)[0] != "AAPL" || (*tc.got)[1] != "NVDA" {
			t.Fatalf("%s symbols = %#v", tc.path, *tc.got)
		}
	}
}
