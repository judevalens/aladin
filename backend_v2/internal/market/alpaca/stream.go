package alpaca

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Trade is one upstream trade tick (Alpaca "T":"t" message), decoded to what the hub needs.
type Trade struct {
	Symbol string
	Price  float64
	Time   time.Time
}

// Stream is ONE Alpaca market-data WebSocket (fan-in). It holds a live symbol set, subscribes
// upstream on demand, decodes trade ticks to onTrade, and auto-reconnects (re-subscribing the
// live set). Safe for concurrent Subscribe/Unsubscribe.
type Stream struct {
	url       string
	apiKey    string
	apiSecret string
	onTrade   func(Trade)

	mu      sync.Mutex
	symbols map[string]bool // desired upstream subscriptions
	conn    *websocket.Conn // current connection (nil when down)
}

func NewStream(streamBaseURL, feed, apiKey, apiSecret string, onTrade func(Trade)) *Stream {
	return &Stream{
		url:       streamBaseURL + "/" + feed,
		apiKey:    apiKey,
		apiSecret: apiSecret,
		onTrade:   onTrade,
		symbols:   map[string]bool{},
	}
}

type wsMessage struct {
	T   string  `json:"T"`           // message type: "t" trade, "success", "error", "subscription"
	S   string  `json:"S,omitempty"` // symbol
	P   float64 `json:"p,omitempty"` // trade price
	Tm  string  `json:"t,omitempty"` // RFC-3339 timestamp
	Msg string  `json:"msg,omitempty"`
	// Sz (trade size) is unused, but the field MUST exist: real trade frames carry
	// both "S" (symbol, string) and "s" (size, number), and without an exact `json:"s"`
	// tag Go's case-insensitive fallback folds the numeric "s" into the string S field,
	// failing the unmarshal and silently dropping EVERY trade frame.
	Sz float64 `json:"s,omitempty"`
}

type command struct {
	Action string   `json:"action"` // auth | subscribe | unsubscribe
	Key    string   `json:"key,omitempty"`
	Secret string   `json:"secret,omitempty"`
	Trades []string `json:"trades,omitempty"`
}

// Run holds the connection open, reconnecting with backoff until ctx is cancelled.
func (s *Stream) Run(ctx context.Context) {
	backoff := time.Second
	for ctx.Err() == nil {
		if err := s.session(ctx); err != nil && ctx.Err() == nil {
			time.Sleep(backoff)
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = time.Second
	}
}

// session connects, authenticates, re-subscribes the live set, and reads until error.
func (s *Stream) session(ctx context.Context) error {
	conn, _, err := websocket.Dial(ctx, s.url, nil)
	if err != nil {
		slog.Warn("alpaca stream: dial failed", "component", "market", "url", s.url, "err", err)
		return err
	}
	defer conn.Close(websocket.StatusNormalClosure, "closing")
	conn.SetReadLimit(1 << 20)
	slog.Info("alpaca stream: connected", "component", "market", "url", s.url)

	if err := writeJSON(ctx, conn, command{Action: "auth", Key: s.apiKey, Secret: s.apiSecret}); err != nil {
		return err
	}

	s.mu.Lock()
	s.conn = conn
	live := s.symbolList()
	s.mu.Unlock()
	if len(live) > 0 {
		_ = writeJSON(ctx, conn, command{Action: "subscribe", Trades: live})
	}
	defer func() {
		s.mu.Lock()
		s.conn = nil
		s.mu.Unlock()
	}()

	decodeFails := 0
	trades := 0
	for ctx.Err() == nil {
		_, data, err := conn.Read(ctx)
		if err != nil {
			slog.Warn("alpaca stream: read error (reconnecting)", "component", "market", "err", err)
			return err
		}
		var msgs []wsMessage
		if err := json.Unmarshal(data, &msgs); err != nil {
			// A frame we can't decode is a frame we silently drop — say so (first few
			// only), because "receiving bytes but publishing nothing" is otherwise
			// undiagnosable from the outside.
			if decodeFails < 3 {
				decodeFails++
				slog.Warn("alpaca stream: undecodable frame", "component", "market",
					"err", err, "sample", string(data[:min(len(data), 200)]))
			}
			continue
		}
		for _, m := range msgs {
			switch m.T {
			case "success", "subscription":
				// Control frames from Alpaca: auth ok, current subscriptions.
				slog.Info("alpaca stream: control", "component", "market", "type", m.T, "msg", m.Msg)
			case "error":
				// An error frame (auth failure, "connection limit exceeded", …) means this
				// session is dead even though the TCP connection stays open. Ignoring it
				// creates a ZOMBIE that itself counts against Alpaca's connection limit —
				// permanently deadlocking the slot. Tear down and let Run's backoff retry.
				slog.Warn("alpaca stream: upstream error (reconnecting)", "component", "market", "msg", m.Msg)
				return fmt.Errorf("alpaca stream: upstream error: %s", m.Msg)
			case "t":
				if s.onTrade != nil {
					ts, _ := time.Parse(time.RFC3339Nano, m.Tm)
					s.onTrade(Trade{Symbol: m.S, Price: m.P, Time: ts})
				}
				// First tick + every 500th: proof of life at Info without tick-spam.
				if trades%500 == 0 {
					slog.Info("alpaca stream: trades flowing", "component", "market", "count", trades+1, "sym", m.S)
				}
				trades++
			}
		}
	}
	return ctx.Err()
}

// Subscribe adds symbols to the live set and, if connected, subscribes upstream now.
func (s *Stream) Subscribe(ctx context.Context, symbols ...string) {
	s.mu.Lock()
	fresh := make([]string, 0, len(symbols))
	for _, sym := range symbols {
		if sym != "" && !s.symbols[sym] {
			s.symbols[sym] = true
			fresh = append(fresh, sym)
		}
	}
	conn := s.conn
	s.mu.Unlock()
	if len(fresh) > 0 {
		slog.Info("alpaca stream: subscribe", "component", "market", "symbols", fresh, "connected", conn != nil)
	}
	if conn != nil && len(fresh) > 0 {
		_ = writeJSON(ctx, conn, command{Action: "subscribe", Trades: fresh})
	}
}

// Unsubscribe removes symbols from the live set and, if connected, unsubscribes upstream.
func (s *Stream) Unsubscribe(ctx context.Context, symbols ...string) {
	s.mu.Lock()
	drop := make([]string, 0, len(symbols))
	for _, sym := range symbols {
		if s.symbols[sym] {
			delete(s.symbols, sym)
			drop = append(drop, sym)
		}
	}
	conn := s.conn
	s.mu.Unlock()
	if conn != nil && len(drop) > 0 {
		_ = writeJSON(ctx, conn, command{Action: "unsubscribe", Trades: drop})
	}
}

func (s *Stream) symbolList() []string {
	out := make([]string, 0, len(s.symbols))
	for sym := range s.symbols {
		out = append(out, sym)
	}
	return out
}

func writeJSON(ctx context.Context, conn *websocket.Conn, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("alpaca stream: marshal: %w", err)
	}
	wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return conn.Write(wctx, websocket.MessageText, b)
}
