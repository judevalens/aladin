package blocknote

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClient_MDToBlocks(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /md-to-blocks", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var input struct {
			Markdown string `json:"markdown"`
		}
		if err := json.Unmarshal(body, &input); err != nil {
			t.Fatalf("decode input: %v", err)
		}
		if input.Markdown != "# Hi" {
			t.Fatalf("markdown payload = %q, want %q", input.Markdown, "# Hi")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"blocks":[{"id":"a","type":"heading"}]}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := NewClient(srv.URL, ClientOptions{})

	blocks, err := client.MDToBlocks(context.Background(), "# Hi")
	if err != nil {
		t.Fatalf("MDToBlocks error: %v", err)
	}
	if !strings.Contains(string(blocks), `"id":"a"`) {
		t.Fatalf("blocks = %s, want heading block", string(blocks))
	}
}

func TestClient_BlocksToMD(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /blocks-to-md", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		// The wrapper should produce {"blocks":[...]}
		var input struct {
			Blocks json.RawMessage `json:"blocks"`
		}
		if err := json.Unmarshal(body, &input); err != nil {
			t.Fatalf("decode input: %v", err)
		}
		if !strings.HasPrefix(string(input.Blocks), `[`) {
			t.Fatalf("blocks payload not a JSON array: %s", string(input.Blocks))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"markdown":"# rendered"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := NewClient(srv.URL, ClientOptions{})

	md, err := client.BlocksToMD(context.Background(), json.RawMessage(`[{"id":"a","type":"heading"}]`))
	if err != nil {
		t.Fatalf("BlocksToMD error: %v", err)
	}
	if md != "# rendered" {
		t.Fatalf("markdown = %q, want %q", md, "# rendered")
	}
}

func TestClient_BlocksToMD_EmptyShortCircuits(t *testing.T) {
	// Empty input should not even hit the server.
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("server should not be called for empty blocks")
	}))
	defer srv.Close()
	client := NewClient(srv.URL, ClientOptions{})

	got, err := client.BlocksToMD(context.Background(), nil)
	if err != nil {
		t.Fatalf("BlocksToMD error: %v", err)
	}
	if got != "" {
		t.Fatalf("markdown = %q, want empty", got)
	}
}

func TestClient_BlocksToMDBatch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /blocks-to-md-batch", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var input struct {
			Blocks []json.RawMessage `json:"blocks"`
		}
		if err := json.Unmarshal(body, &input); err != nil {
			t.Fatalf("decode input: %v", err)
		}
		if len(input.Blocks) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(input.Blocks))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"markdowns":["a","b"]}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := NewClient(srv.URL, ClientOptions{})

	out, err := client.BlocksToMDBatch(context.Background(), []json.RawMessage{
		json.RawMessage(`[{"id":"a"}]`),
		json.RawMessage(`[{"id":"b"}]`),
	})
	if err != nil {
		t.Fatalf("BlocksToMDBatch error: %v", err)
	}
	if len(out) != 2 || out[0] != "a" || out[1] != "b" {
		t.Fatalf("out = %#v, want [a b]", out)
	}
}

func TestClient_BlocksToMDBatch_EmptyShortCircuits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("server should not be called for empty input")
	}))
	defer srv.Close()
	client := NewClient(srv.URL, ClientOptions{})

	out, err := client.BlocksToMDBatch(context.Background(), nil)
	if err != nil {
		t.Fatalf("BlocksToMDBatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("out = %#v, want empty slice", out)
	}
}

func TestClient_PropagatesConverterError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /md-to-blocks", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"malformed markdown"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := NewClient(srv.URL, ClientOptions{})

	_, err := client.MDToBlocks(context.Background(), "bad input")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrConverter) {
		t.Fatalf("error = %v, want errors.Is(ErrConverter)", err)
	}
	if !strings.Contains(err.Error(), "malformed markdown") {
		t.Fatalf("error = %v, want it to include the server message", err)
	}
}

func TestClient_TimeoutTriggered(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	client := NewClient(srv.URL, ClientOptions{RequestTimeout: 50 * time.Millisecond})

	_, err := client.MDToBlocks(context.Background(), "x")
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestClient_Healthz(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := NewClient(srv.URL, ClientOptions{})
	if err := client.Healthz(context.Background()); err != nil {
		t.Fatalf("Healthz: %v", err)
	}
}

func TestClient_HealthzNonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	client := NewClient(srv.URL, ClientOptions{})
	if err := client.Healthz(context.Background()); err == nil {
		t.Fatal("expected error for non-OK healthz, got nil")
	}
}
