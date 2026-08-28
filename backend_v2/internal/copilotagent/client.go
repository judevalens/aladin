// Package copilotagent is the HTTP client for the copilot-agent sidecar
// (services/copilot-agent): the Node process that runs the Claude Agent SDK
// per copilot turn. The Go API starts a turn, consumes the NDJSON event
// stream, and relays cancels/approvals — mirroring internal/blocknote's
// client-for-a-sidecar shape.
package copilotagent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// streamIdleTimeout bounds transport silence, not model/tool progress. The sidecar
// sends a heartbeat every 15 seconds even while reasoning or waiting for approval.
// The caller's 15-minute deadline still bounds a live but stuck provider.
// A var (not const) only so tests can shrink it.
var streamIdleTimeout = 120 * time.Second

var errStreamIdle = errors.New("copilot-agent stream idle timeout")

// TurnRequest is the body of POST /turn.
type TurnRequest struct {
	TurnID          string   `json:"turnId"`
	ThreadID        string   `json:"threadId"`
	ResumeSessionID string   `json:"resumeSessionId,omitempty"`
	UserBearer      string   `json:"userBearer"`
	SystemPrompt    string   `json:"systemPrompt"`
	Prompt          string   `json:"prompt"`
	HistoryFallback string   `json:"historyFallback,omitempty"`
	Provider        string   `json:"provider,omitempty"`
	Model           string   `json:"model,omitempty"`
	Effort          string   `json:"effort,omitempty"`
	GatedTools      []string `json:"gatedTools,omitempty"`
	MaxTurns        int      `json:"maxTurns,omitempty"`
}

// Event is one NDJSON line from the sidecar's turn stream. A flat union — Type
// selects which fields are meaningful (see services/copilot-agent/src/translate.js
// for the authoritative vocabulary).
type Event struct {
	Type string `json:"type"`
	// session / done
	SessionID string `json:"sessionId,omitempty"`
	Resumed   bool   `json:"resumed,omitempty"`
	// token
	Delta string `json:"delta,omitempty"`
	// tool_start / tool_result
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"toolUseId,omitempty"`
	Content   string          `json:"content,omitempty"`
	IsError   bool            `json:"isError,omitempty"`
	// proposed_action / approval_resolved (ApprovalID also rides on tool_result)
	ApprovalID string `json:"approvalId,omitempty"`
	Tool       string `json:"tool,omitempty"`
	Approved   bool   `json:"approved,omitempty"`
	TimedOut   bool   `json:"timedOut,omitempty"`
	// message / error
	Text    string `json:"text,omitempty"`
	Message string `json:"message,omitempty"`
	// Code is a compact machine code on error events ("max_turns", "max_budget",
	// "api_error", "run_failed") — the dock switches UI affordances on it.
	Code string `json:"code,omitempty"`
	// done
	NumTurns int       `json:"numTurns,omitempty"`
	Usage    TurnUsage `json:"usage,omitzero"`
	CostUSD  float64   `json:"costUsd,omitempty"`
}

type TurnUsage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
}

type Client struct {
	baseURL      string
	sharedSecret string
	// stream has no overall timeout (a turn is long-lived; the caller's ctx
	// bounds it); control covers the short cancel/approval/health calls.
	stream  *http.Client
	control *http.Client
}

func New(baseURL, sharedSecret string) *Client {
	return &Client{
		baseURL:      strings.TrimRight(baseURL, "/"),
		sharedSecret: sharedSecret,
		stream:       &http.Client{},
		control:      &http.Client{Timeout: 10 * time.Second},
	}
}

// StartTurn posts the turn and returns a channel of decoded events. The
// channel closes when the stream ends; the sidecar guarantees the last event
// is `done` on every path, so a close without one means the stream broke
// (sidecar died mid-turn) and the caller should treat the turn as failed.
func (c *Client) StartTurn(ctx context.Context, req TurnRequest) (<-chan Event, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("copilot-agent marshal turn: %w", err)
	}
	// Child context so the idle watchdog can abort a STALLED stream without cancelling the caller's
	// turn context — a broken stream then surfaces as "interrupted", not a clean user-stop.
	streamCtx, cancelStream := context.WithCancelCause(ctx)
	httpReq, err := http.NewRequestWithContext(streamCtx, http.MethodPost, c.baseURL+"/turn", bytes.NewReader(payload))
	if err != nil {
		cancelStream(nil)
		return nil, fmt.Errorf("copilot-agent turn request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Agent-Secret", c.sharedSecret)

	resp, err := c.stream.Do(httpReq)
	if err != nil {
		cancelStream(nil)
		return nil, fmt.Errorf("copilot-agent turn: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		_ = resp.Body.Close()
		cancelStream(nil)
		return nil, fmt.Errorf("copilot-agent turn: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	events := make(chan Event, 16)
	go func() {
		defer close(events)
		defer cancelStream(nil)
		defer func() { _ = resp.Body.Close() }()
		// Missing heartbeats mean the transport, rather than merely
		// the model, stopped responding. Keep this independent of turn timeout.
		idle := time.AfterFunc(streamIdleTimeout, func() { cancelStream(errStreamIdle) })
		defer idle.Stop()
		scanner := bufio.NewScanner(resp.Body)
		// Tool results are capped at 32KB sidecar-side but inputs/prompts ride
		// along too — allow generous lines.
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			idle.Reset(streamIdleTimeout)
			line := bytes.TrimSpace(scanner.Bytes())
			if len(line) == 0 {
				continue
			}
			var ev Event
			if err := json.Unmarshal(line, &ev); err != nil {
				continue // one malformed line must not kill the turn
			}
			if ev.Type == "heartbeat" {
				continue // liveness only; never alter the dock's progress state
			}
			select {
			case events <- ev:
			case <-streamCtx.Done():
				return
			}
			if ev.Type == "done" {
				return // terminal event; don't wait for provider cleanup to close HTTP
			}
		}
		if ctx.Err() == nil {
			reason := "unexpected_eof"
			if errors.Is(context.Cause(streamCtx), errStreamIdle) {
				reason = "idle_timeout"
			} else if scanner.Err() != nil {
				reason = "read_error"
			}
			slog.Warn("copilot-agent: stream interrupted", "thread", req.ThreadID,
				"turn", req.TurnID, "provider", req.Provider, "reason", reason, "err", scanner.Err())
		}
	}()
	return events, nil
}

// Cancel aborts a running turn. Idempotent — cancelling an unknown/finished
// turn is a no-op sidecar-side.
func (c *Client) Cancel(ctx context.Context, turnID string) error {
	return c.postControl(ctx, "/turn/"+turnID+"/cancel", nil)
}

// ResolveApproval settles a gated tool call held open in the sidecar's
// canUseTool. A 404 (expired/unknown approval) comes back as an error.
func (c *Client) ResolveApproval(ctx context.Context, turnID, approvalID string, approved bool) error {
	body := map[string]bool{"approved": approved}
	return c.postControl(ctx, "/turn/"+turnID+"/approvals/"+approvalID, body)
}

// Health is the sidecar's /healthz report.
type Health struct {
	OK           bool    `json:"ok"`
	AuthMode     string  `json:"authMode"`
	AnthropicKey bool    `json:"anthropicKey"`
	OpenAIKey    bool    `json:"openaiKey"`
	CodexCommand string  `json:"codexCommand,omitempty"`
	CodexCwd     string  `json:"codexCwd,omitempty"`
	MCP          bool    `json:"mcp"`
	Catalog      Catalog `json:"catalog,omitzero"`
}

type Catalog struct {
	DefaultProvider string            `json:"defaultProvider,omitempty"`
	DefaultModel    string            `json:"defaultModel,omitempty"`
	Providers       []ProviderCatalog `json:"providers,omitempty"`
}

type ProviderCatalog struct {
	ID            string                 `json:"id"`
	Label         string                 `json:"label"`
	DefaultModel  string                 `json:"defaultModel,omitempty"`
	DefaultEffort string                 `json:"defaultEffort,omitempty"`
	Capabilities  map[string]bool        `json:"capabilities,omitempty"`
	Models        []ProviderModelOption  `json:"models,omitempty"`
	Efforts       []ProviderEffortOption `json:"efforts,omitempty"`
}

type ProviderModelOption struct {
	ID          string `json:"id"`
	Provider    string `json:"provider,omitempty"`
	Model       string `json:"model,omitempty"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

type ProviderEffortOption struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

func (c *Client) Healthz(ctx context.Context) (Health, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/healthz", nil)
	if err != nil {
		return Health{}, err
	}
	resp, err := c.control.Do(req)
	if err != nil {
		return Health{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return Health{}, fmt.Errorf("copilot-agent healthz: status %d", resp.StatusCode)
	}
	var h Health
	if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
		return Health{}, fmt.Errorf("copilot-agent healthz decode: %w", err)
	}
	return h, nil
}

func (c *Client) postControl(ctx context.Context, path string, body any) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-Secret", c.sharedSecret)
	resp, err := c.control.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("copilot-agent %s: status %d: %s", path, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}
