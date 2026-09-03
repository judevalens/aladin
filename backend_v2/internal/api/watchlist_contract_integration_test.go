package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/dbtest"
	"aladin/backend_v2/internal/repo"
	"aladin/backend_v2/internal/watchlist"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestWatchlistHTTPServiceRepositoryContract freezes one representative mutation/read lifecycle
// through the real HTTP router, service, PostgreSQL repository, and outbox. It deliberately uses
// the same public payloads as clients so a dependency-only refactor cannot silently change them.
func TestWatchlistHTTPServiceRepositoryContract(t *testing.T) {
	dsn := dbtest.RequireTestDSN(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}

	userID := uuid.NewString()
	instrumentID := uuid.NewString()
	symbol := "WLC" + uuid.NewString()[:6]
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,email,created_at,updated_at) VALUES($1::uuid,$2,now(),now())`, userID, userID+"@watchlist.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO instruments(instrument_id,symbol,name,exchange) VALUES($1::uuid,$2,'Watchlist Contract','TEST')`, instrumentID, symbol); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM outbox_events WHERE user_id=$1::uuid`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM watchlists WHERE user_id=$1::uuid`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM instruments WHERE instrument_id=$1::uuid`, instrumentID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1::uuid`, userID)
	})

	watchlists := watchlist.NewService(repo.NewWatchlistPostgres(pool))
	server := NewWithDependencies(":0", testDependencies{
		AuthSvc:      &resourceAPIAuth{userID: userID},
		WatchlistSvc: watchlists,
	})
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	call := func(method, path, raw string, wantStatus int, dst any) {
		t.Helper()
		var body *bytes.Reader
		if raw == "" {
			body = bytes.NewReader(nil)
		} else {
			body = bytes.NewReader([]byte(raw))
		}
		req, err := http.NewRequestWithContext(ctx, method, httpServer.URL+path, body)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer resource-test-token")
		if raw != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := httpServer.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != wantStatus {
			var failure any
			_ = json.NewDecoder(resp.Body).Decode(&failure)
			t.Fatalf("%s %s status = %d, want %d: %#v", method, path, resp.StatusCode, wantStatus, failure)
		}
		if dst != nil {
			if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
				t.Fatalf("decode %s %s: %v", method, path, err)
			}
		}
	}

	var created watchlist.Watchlist
	call(http.MethodPost, "/api/watchlists", `{"name":"Semis"}`, http.StatusCreated, &created)
	if created.ID == "" || created.Name != "Semis" || created.Kind != watchlist.Manual {
		t.Fatalf("created = %#v", created)
	}

	call(http.MethodPost, "/api/watchlists/"+created.ID+"/items", `{"instrumentId":"`+instrumentID+`"}`, http.StatusCreated, &map[string]bool{})

	var listed struct {
		Watchlists []watchlist.Watchlist `json:"watchlists"`
	}
	call(http.MethodGet, "/api/watchlists", "", http.StatusOK, &listed)
	if len(listed.Watchlists) != 1 || listed.Watchlists[0].ID != created.ID || listed.Watchlists[0].ItemCount != 1 {
		t.Fatalf("listed = %#v", listed.Watchlists)
	}

	var items []watchlist.WatchlistItem
	call(http.MethodGet, "/api/watchlists/"+created.ID+"/items", "", http.StatusOK, &items)
	if len(items) != 1 || items[0].InstrumentID != instrumentID || items[0].Symbol != symbol {
		t.Fatalf("items = %#v", items)
	}

	call(http.MethodPatch, "/api/watchlists/"+created.ID, `{"name":"Semiconductors"}`, http.StatusOK, &map[string]bool{})
	call(http.MethodDelete, "/api/watchlists/"+created.ID+"/items/"+instrumentID, "", http.StatusOK, &map[string]bool{})
	call(http.MethodDelete, "/api/watchlists/"+created.ID, "", http.StatusOK, &map[string]bool{})

	var frameCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE user_id=$1::uuid AND type='data_event'`, userID).Scan(&frameCount); err != nil {
		t.Fatal(err)
	}
	if frameCount != 5 {
		t.Fatalf("outbox frame count = %d, want 5 for create/add/rename/remove/delete", frameCount)
	}
}
