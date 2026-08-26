package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"aladin/backend_v2/internal/blocknote"
	"aladin/backend_v2/internal/copilotagent"
	"aladin/backend_v2/internal/safego"

	"github.com/google/uuid"
)

// CopilotService is Aladin's default agentic LLM interface: a surface-aware, streaming
// assistant that grounds its answers on the user's own Aladin data.
//
// The agent loop itself runs in the copilot-agent sidecar (Claude Agent SDK), which
// consumes tools from the Go MCP server with the calling user's own bearer. This service
// stays the orchestrator: SendMessage persists the user turn, spawns a goroutine that
// streams the sidecar's NDJSON events, and republishes them over the realtime workspace
// stream (resource kind "copilot") so the dock renders live; the final answer is
// persisted so a reconnecting client can replay it.
type CopilotService interface {
	SendMessage(ctx context.Context, in CopilotSendInput) (CopilotSendResult, error)
	ListThreads(ctx context.Context, userID string) ([]CopilotThread, error)
	GetThread(ctx context.Context, userID, threadID string) (CopilotThreadDetail, error)
	RenameThread(ctx context.Context, userID, threadID, title string) (CopilotThread, error)
	ArchiveThread(ctx context.Context, userID, threadID string) error
	SetThreadPinned(ctx context.Context, userID, threadID string, pinned bool) (CopilotThread, error)
	// Cancel stops an in-flight turn (halting the work + cost), scoped to its owner.
	Cancel(ctx context.Context, userID, sessionID string) error
	// ApproveAction releases a gated tool call held open in the sidecar; RejectAction
	// denies it. The result streams back as an action_result event.
	ApproveAction(ctx context.Context, userID, actionID string) error
	RejectAction(ctx context.Context, userID, actionID string) error
	// Configured reports whether the copilot-agent sidecar is wired.
	Configured() bool
	// Status probes the sidecar's health (incl. MCP tool-server reachability) so the
	// dock can warn before the user types. Never errors — degraded is a valid status.
	Status(ctx context.Context) CopilotStatusReport
}

// CopilotStatusReport is the dock's preflight view of the copilot's health.
type CopilotStatusReport struct {
	Configured    bool                  `json:"configured"`
	Sidecar       bool                  `json:"sidecar"`
	MCP           bool                  `json:"mcp"`
	DefaultModel  string                `json:"defaultModel,omitempty"`
	Models        []CopilotModelOption  `json:"models,omitempty"`
	DefaultEffort string                `json:"defaultEffort,omitempty"`
	Efforts       []CopilotEffortOption `json:"efforts,omitempty"`
}

// CopilotModelOption is one model the dock may select for the next turn.
type CopilotModelOption struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// CopilotEffortOption is one reasoning-effort level the dock may select for the next turn.
type CopilotEffortOption struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// gatedToolNames are tools whose effects are hard to reverse — the sidecar's canUseTool
// holds them open (proposed_action) until the user approves via the dock. Names are the
// MCP server's tool names.
func gatedToolNames() []string {
	return []string{"publish_app", "update_page", "delete_block", "delete_file", "create_alert"}
}

// CopilotSurface is what the user is looking at when they ask — makes the agent context-aware.
type CopilotSurface struct {
	Kind   string `json:"kind"`             // entity | ticker | page | markets | …
	ID     string `json:"id,omitempty"`     // entity id / page id when applicable
	Symbol string `json:"symbol,omitempty"` // ticker symbol when kind == ticker
	Label  string `json:"label,omitempty"`  // human label for the prompt
}

type CopilotSendInput struct {
	Principal Principal
	// Bearer is the caller's own credential (session token), forwarded to the sidecar so
	// its MCP tool calls are scoped exactly like the user's own API calls.
	Bearer   string
	ThreadID string // "" ⇒ start a new thread
	Text     string
	Model    string
	Effort   string
	Surface  CopilotSurface
}

type CopilotSendResult struct {
	ThreadID  string `json:"threadId"`
	SessionID string `json:"sessionId"`
}

// Citation is one grounding source the assistant used, rendered as a clickable chip.
type Citation struct {
	Kind  string `json:"kind"` // ticker | company | person | entity | page | shard
	ID    string `json:"id"`
	Title string `json:"title"`
	// Page anchors a document citation; the client opens the source at it. 0 = none.
	// Dedup is by kind|id, so a turn that read several spans keeps the last one's page.
	Page int `json:"page,omitempty"`
}

type CopilotThread struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	UpdatedAt string `json:"updatedAt"`
	Pinned    bool   `json:"pinned"`
	// SDKSessionID is the Claude Agent SDK session resumed on each turn (empty for
	// fresh/legacy threads). Server-internal — never serialized to the client.
	SDKSessionID string `json:"-"`
}

// CopilotActivityItem is one tool invocation in a turn's compact activity digest.
type CopilotActivityItem struct {
	Name string `json:"name"`
	OK   bool   `json:"ok"`
}

// CopilotMessageMeta is per-assistant-turn metadata: what the turn did and cost.
type CopilotMessageMeta struct {
	NumTurns     int                   `json:"numTurns,omitempty"`
	InputTokens  int                   `json:"inputTokens,omitempty"`
	OutputTokens int                   `json:"outputTokens,omitempty"`
	CostUSD      float64               `json:"costUsd,omitempty"`
	Activity     []CopilotActivityItem `json:"activity,omitempty"`
}

type CopilotMessage struct {
	ID        string              `json:"id"`
	Role      string              `json:"role"` // user | assistant
	Content   string              `json:"content"`
	Citations []Citation          `json:"citations"`
	Meta      *CopilotMessageMeta `json:"meta,omitempty"`
	CreatedAt string              `json:"createdAt"`
}

type CopilotThreadDetail struct {
	Thread   CopilotThread    `json:"thread"`
	Messages []CopilotMessage `json:"messages"`
}

// StoredCopilotMessage is one durable turn (the write shape for the store).
type StoredCopilotMessage struct {
	ID        string
	ThreadID  string
	Role      string
	Content   string
	Citations []Citation
	Meta      *CopilotMessageMeta
}

// CopilotStore persists threads + the visible conversation. Ownership is enforced by
// passing userID on the scoped reads.
type CopilotStore interface {
	CreateThread(ctx context.Context, id, userID, title string) error
	TouchThread(ctx context.Context, threadID string) error
	ListThreads(ctx context.Context, userID string) ([]CopilotThread, error)
	// GetThread returns the thread iff it belongs to userID (found=false otherwise).
	GetThread(ctx context.Context, userID, threadID string) (CopilotThread, bool, error)
	RenameThread(ctx context.Context, userID, threadID, title string) (CopilotThread, bool, error)
	ArchiveThread(ctx context.Context, userID, threadID string) (bool, error)
	SetThreadPinned(ctx context.Context, userID, threadID string, pinned bool) (CopilotThread, bool, error)
	// SetThreadSDKSession stamps the Claude Agent SDK session id to resume next turn.
	SetThreadSDKSession(ctx context.Context, threadID, sessionID string) error
	AppendMessage(ctx context.Context, m StoredCopilotMessage) error
	ListMessages(ctx context.Context, threadID string) ([]CopilotMessage, error)
}

// CopilotAgent is the sidecar client surface the service consumes (satisfied by
// *copilotagent.Client; an interface so tests can fake the stream).
type CopilotAgent interface {
	StartTurn(ctx context.Context, req copilotagent.TurnRequest) (<-chan copilotagent.Event, error)
	Cancel(ctx context.Context, turnID string) error
	ResolveApproval(ctx context.Context, turnID, approvalID string, approved bool) error
	Healthz(ctx context.Context) (copilotagent.Health, error)
}

// CopilotDeps are the collaborators the orchestrator needs: the sidecar client, the
// realtime hub for streaming, the store for persistence, and the few read services that
// build the ambient surface-context block. Agent may be nil (sidecar not configured) ⇒
// the service reports Configured()=false and rejects sends cleanly.
type CopilotDeps struct {
	Store    CopilotStore
	Agent    CopilotAgent
	Realtime RealtimeEventService
	// Model pins the sidecar's model per turn ("" ⇒ the sidecar's own default).
	Model string
	// Effort guides adaptive thinking depth per turn ("" ⇒ high, matching Claude Code default).
	Effort string
	// Surface-context preloading (all optional/nil-safe).
	Snapshots QuoteSnapshotSource
	Artifacts ArtifactService
	Entities  EntityContextService
	Watchlist WatchlistService
}

const (
	copilotResourceKind  = "copilot"
	defaultCopilotModel  = "claude-opus-5"
	defaultCopilotEffort = "high"
	// Authoring a shard/page is multi-step (create → write → build → fix → build → preview),
	// so the loop needs real headroom; a Q&A answers in 1–3 and stops on its own. A
	// comprehensive multi-section shard blew through 24, so this is sized for deep authoring;
	// the 15-min turn deadline + the SDK's own stopping behavior are the real guardrails.
	// SDK turns, enforced sidecar-side. Hitting the limit is recoverable: the thread resumes
	// the SDK session, so "continue" picks up mid-build.
	maxCopilotTurns = 60
	// The hard turn deadline must contain one full approval hold (the sidecar keeps a gated
	// tool open up to 10 minutes waiting for approve/reject) plus real work either side.
	copilotTurnTimeout     = 15 * time.Minute
	maxSurfaceContextChars = 1800
	// historyFallback bounds: the durable text history sent along for resume-failure
	// recovery (most turns resume the SDK session and never use it).
	historyFallbackTurns = 12
	historyFallbackChars = 6000
	// maxActivityItems caps the per-turn tool digest persisted in message meta.
	maxActivityItems    = 40
	maxToolSummaryChars = 180
)

var copilotModelCatalog = []CopilotModelOption{
	{
		ID:          "claude-opus-5",
		Label:       "Opus 5",
		Description: "Best reasoning for shard authoring and hard workspace tasks.",
	},
	{
		ID:          "claude-sonnet-5",
		Label:       "Sonnet 5",
		Description: "Fast everyday coding and research assistant work.",
	},
	{
		ID:          "claude-fable-5",
		Label:       "Fable 5",
		Description: "Quick lightweight answers when speed matters most.",
	},
}

var legacyCopilotModelIDs = map[string]string{
	"opus":    "claude-opus-5",
	"opus5":   "claude-opus-5",
	"sonnet":  "claude-sonnet-5",
	"sonnet5": "claude-sonnet-5",
	"fable":   "claude-fable-5",
	"fable5":  "claude-fable-5",
}

var copilotEffortCatalog = []CopilotEffortOption{
	{ID: "low", Label: "Low", Description: "Fastest responses with minimal thinking."},
	{ID: "medium", Label: "Medium", Description: "Balanced reasoning for routine work."},
	{ID: "high", Label: "High", Description: "Deep reasoning; Claude Code default."},
	{ID: "xhigh", Label: "X-High", Description: "Deeper reasoning for harder agentic tasks."},
	{ID: "max", Label: "Max", Description: "Maximum effort for the hardest long-running tasks."},
}

// runningTurn is a live agent turn's cancel handle, kept so the owner can stop it and
// so SendMessage can refuse a second concurrent turn on the same thread.
type runningTurn struct {
	userID   string
	threadID string
	cancel   context.CancelFunc
}

// pendingAction is a gated tool call held open in the sidecar, awaiting approve/reject.
type pendingAction struct {
	userID   string
	threadID string
	turnID   string
	tool     string
	created  time.Time
}

// pendingActionTTL prunes stale pending-approval entries. Slightly above the sidecar's
// 10-minute approval hold (APPROVAL_TIMEOUT_MS) — after that the hold has already been
// denied as timed out, so anything older is garbage.
const pendingActionTTL = 11 * time.Minute

type defaultCopilotService struct {
	CopilotDeps
	mu      sync.Mutex
	running map[string]runningTurn   // sessionID → cancel handle
	pending map[string]pendingAction // actionID (sidecar approvalId) → held action
}

func NewCopilotService(deps CopilotDeps) CopilotService {
	return &defaultCopilotService{
		CopilotDeps: deps,
		running:     map[string]runningTurn{},
		pending:     map[string]pendingAction{},
	}
}

func (s *defaultCopilotService) Configured() bool { return s.Agent != nil }

func (s *defaultCopilotService) defaultModel() string {
	if model := strings.TrimSpace(s.Model); model != "" {
		return model
	}
	return defaultCopilotModel
}

func (s *defaultCopilotService) defaultEffort() string {
	if effort := strings.TrimSpace(s.Effort); effort != "" {
		if normalized, ok := normalizeCopilotEffort(effort); ok {
			return normalized
		}
	}
	return defaultCopilotEffort
}

func (s *defaultCopilotService) modelOptions() []CopilotModelOption {
	configured := s.defaultModel()
	options := make([]CopilotModelOption, 0, len(copilotModelCatalog)+1)
	seen := map[string]bool{}
	for _, option := range copilotModelCatalog {
		options = append(options, option)
		seen[option.ID] = true
	}
	if !seen[configured] {
		options = append([]CopilotModelOption{{
			ID:          configured,
			Label:       configured,
			Description: "Configured backend default.",
		}}, options...)
	}
	return options
}

func (s *defaultCopilotService) normalizeModel(model string) (string, bool) {
	model = strings.TrimSpace(model)
	if model == "" {
		return s.defaultModel(), true
	}
	if normalized, ok := legacyCopilotModelIDs[model]; ok {
		model = normalized
	}
	for _, option := range s.modelOptions() {
		if model == option.ID {
			return model, true
		}
	}
	return "", false
}

func normalizeCopilotEffort(effort string) (string, bool) {
	effort = strings.ToLower(strings.TrimSpace(effort))
	if effort == "x-high" || effort == "extra-high" {
		effort = "xhigh"
	}
	for _, option := range copilotEffortCatalog {
		if effort == option.ID {
			return effort, true
		}
	}
	return "", false
}

func (s *defaultCopilotService) normalizeEffort(effort string) (string, bool) {
	effort = strings.TrimSpace(effort)
	if effort == "" {
		return s.defaultEffort(), true
	}
	return normalizeCopilotEffort(effort)
}

func (s *defaultCopilotService) Status(ctx context.Context) CopilotStatusReport {
	base := CopilotStatusReport{
		Configured:    s.Agent != nil,
		DefaultModel:  s.defaultModel(),
		Models:        s.modelOptions(),
		DefaultEffort: s.defaultEffort(),
		Efforts:       copilotEffortCatalog,
	}
	if s.Agent == nil {
		return base
	}
	h, err := s.Agent.Healthz(ctx)
	if err != nil {
		return base
	}
	base.Sidecar = h.OK
	base.MCP = h.MCP
	return base
}

// Cancel stops the in-flight turn for sessionID if it belongs to userID. Idempotent:
// cancelling an already-finished (or unknown) session is a no-op.
func (s *defaultCopilotService) Cancel(_ context.Context, userID, sessionID string) error {
	s.mu.Lock()
	rt, ok := s.running[sessionID]
	s.mu.Unlock()
	if !ok {
		return nil
	}
	if rt.userID != userID {
		return ErrNotFound
	}
	rt.cancel()
	// Best-effort: tell the sidecar too, so the SDK query aborts promptly instead of
	// discovering the dropped stream.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.Agent.Cancel(ctx, sessionID)
	}()
	return nil
}

// registerPending records a held gated action under the sidecar's approvalId, pruning
// stale entries.
func (s *defaultCopilotService) registerPending(actionID, userID, threadID, turnID, tool string) {
	s.mu.Lock()
	for id, pa := range s.pending {
		if time.Since(pa.created) > pendingActionTTL {
			delete(s.pending, id)
		}
	}
	s.pending[actionID] = pendingAction{userID: userID, threadID: threadID, turnID: turnID, tool: tool, created: time.Now()}
	s.mu.Unlock()
}

func (s *defaultCopilotService) takePending(userID, actionID string) (pendingAction, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pa, ok := s.pending[actionID]
	if !ok || pa.userID != userID {
		return pendingAction{}, false
	}
	delete(s.pending, actionID)
	return pa, true
}

func (s *defaultCopilotService) dropPending(actionID string) {
	s.mu.Lock()
	delete(s.pending, actionID)
	s.mu.Unlock()
}

// ApproveAction releases a gated tool call held open in the sidecar (owner-scoped). The
// tool then runs inside the same turn; its result streams back as an action_result event.
func (s *defaultCopilotService) ApproveAction(ctx context.Context, userID, actionID string) error {
	pa, ok := s.takePending(userID, actionID)
	if !ok {
		return ErrNotFound
	}
	if err := s.Agent.ResolveApproval(ctx, pa.turnID, actionID, true); err != nil {
		// The hold is gone (turn ended, approval timed out) — resolve the dock card.
		s.publish(userID, pa.threadID, "action_result", copilotActionResultPayload{
			ThreadID: pa.threadID, ActionID: actionID, OK: false,
			Message: "That approval expired — ask the copilot to try again.",
		})
		return nil
	}
	return nil
}

// RejectAction denies a held gated tool call. Idempotent.
func (s *defaultCopilotService) RejectAction(ctx context.Context, userID, actionID string) error {
	pa, ok := s.takePending(userID, actionID)
	if !ok {
		return nil
	}
	if err := s.Agent.ResolveApproval(ctx, pa.turnID, actionID, false); err != nil {
		// Hold already gone — still resolve the dock card.
		s.publish(userID, pa.threadID, "action_result", copilotActionResultPayload{
			ThreadID: pa.threadID, ActionID: actionID, OK: false, Message: "Dismissed.",
		})
	}
	return nil
}

func (s *defaultCopilotService) SendMessage(ctx context.Context, in CopilotSendInput) (CopilotSendResult, error) {
	if s.Agent == nil {
		return CopilotSendResult{}, BadRequest("copilot not configured")
	}
	text := strings.TrimSpace(in.Text)
	if text == "" {
		return CopilotSendResult{}, BadRequest("text is required")
	}
	userID := strings.TrimSpace(in.Principal.UserID)
	if userID == "" {
		return CopilotSendResult{}, ErrUnauthenticated
	}
	if strings.TrimSpace(in.Bearer) == "" {
		return CopilotSendResult{}, ErrUnauthenticated
	}
	model, ok := s.normalizeModel(in.Model)
	if !ok {
		return CopilotSendResult{}, BadRequest("unsupported copilot model")
	}
	effort, ok := s.normalizeEffort(in.Effort)
	if !ok {
		return CopilotSendResult{}, BadRequest("unsupported copilot effort")
	}

	threadID := strings.TrimSpace(in.ThreadID)
	if threadID == "" {
		threadID = uuid.NewString()
		if err := s.Store.CreateThread(ctx, threadID, userID, threadTitle(text)); err != nil {
			return CopilotSendResult{}, err
		}
	} else {
		_, ok, err := s.Store.GetThread(ctx, userID, threadID)
		if err != nil {
			return CopilotSendResult{}, err
		}
		if !ok {
			return CopilotSendResult{}, ErrNotFound
		}
	}

	// One turn per thread. The turn context is created here and registered under the
	// SAME lock as the conflict check, closing the check-then-spawn race (double-send,
	// a second window). The guard runs before the user message is persisted so a
	// rejected send leaves no orphan turn in the transcript.
	sessionID := uuid.NewString()
	turnCtx, cancel := context.WithTimeout(WithPrincipal(context.Background(), in.Principal), copilotTurnTimeout)
	s.mu.Lock()
	for _, rt := range s.running {
		if rt.threadID == threadID {
			s.mu.Unlock()
			cancel()
			return CopilotSendResult{}, ErrConflict
		}
	}
	s.running[sessionID] = runningTurn{userID: userID, threadID: threadID, cancel: cancel}
	s.mu.Unlock()
	release := func() {
		s.mu.Lock()
		delete(s.running, sessionID)
		s.mu.Unlock()
		cancel()
	}

	if err := s.Store.AppendMessage(ctx, StoredCopilotMessage{
		ID: uuid.NewString(), ThreadID: threadID, Role: "user", Content: text,
	}); err != nil {
		release()
		return CopilotSendResult{}, err
	}
	_ = s.Store.TouchThread(ctx, threadID)

	// Fire-and-forget turn, panic-contained: a malformed NDJSON event from the sidecar must not
	// crash the api process. runAgent defers release(), so the hold-open slot is freed even on panic.
	safego.Go("copilot.turn", func() {
		s.runAgent(turnCtx, release, in.Principal, in.Bearer, threadID, sessionID, model, effort, in.Surface)
	})
	return CopilotSendResult{ThreadID: threadID, SessionID: sessionID}, nil
}

func (s *defaultCopilotService) ListThreads(ctx context.Context, userID string) ([]CopilotThread, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, ErrUnauthenticated
	}
	return s.Store.ListThreads(ctx, userID)
}

func (s *defaultCopilotService) GetThread(ctx context.Context, userID, threadID string) (CopilotThreadDetail, error) {
	thread, ok, err := s.Store.GetThread(ctx, userID, threadID)
	if err != nil {
		return CopilotThreadDetail{}, err
	}
	if !ok {
		return CopilotThreadDetail{}, ErrNotFound
	}
	messages, err := s.Store.ListMessages(ctx, threadID)
	if err != nil {
		return CopilotThreadDetail{}, err
	}
	return CopilotThreadDetail{Thread: thread, Messages: messages}, nil
}

func (s *defaultCopilotService) RenameThread(ctx context.Context, userID, threadID, title string) (CopilotThread, error) {
	if strings.TrimSpace(userID) == "" {
		return CopilotThread{}, ErrUnauthenticated
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return CopilotThread{}, BadRequest("title is required")
	}
	if len(title) > 120 {
		title = title[:120]
	}
	thread, ok, err := s.Store.RenameThread(ctx, userID, threadID, title)
	if err != nil {
		return CopilotThread{}, err
	}
	if !ok {
		return CopilotThread{}, ErrNotFound
	}
	return thread, nil
}

func (s *defaultCopilotService) ArchiveThread(ctx context.Context, userID, threadID string) error {
	if strings.TrimSpace(userID) == "" {
		return ErrUnauthenticated
	}
	ok, err := s.Store.ArchiveThread(ctx, userID, threadID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

func (s *defaultCopilotService) SetThreadPinned(ctx context.Context, userID, threadID string, pinned bool) (CopilotThread, error) {
	if strings.TrimSpace(userID) == "" {
		return CopilotThread{}, ErrUnauthenticated
	}
	thread, ok, err := s.Store.SetThreadPinned(ctx, userID, threadID, pinned)
	if err != nil {
		return CopilotThread{}, err
	}
	if !ok {
		return CopilotThread{}, ErrNotFound
	}
	return thread, nil
}

// runAgent drives ONE turn through the sidecar, in its own goroutine: start the turn,
// translate its NDJSON events onto the realtime hub, persist the final assistant turn
// and the SDK session id. The ctx (principal + hard timeout) and its registration in
// s.running are created by SendMessage under the per-thread guard; release deregisters
// and cancels on exit.
func (s *defaultCopilotService) runAgent(ctx context.Context, release func(), principal Principal, bearer, threadID, sessionID, model, effort string, surface CopilotSurface) {
	defer release()
	userID := principal.UserID

	thread, ok, err := s.Store.GetThread(ctx, userID, threadID)
	if err != nil || !ok {
		s.fail(userID, threadID, sessionID, "failed to load conversation")
		return
	}
	history, err := s.Store.ListMessages(ctx, threadID)
	if err != nil {
		s.fail(userID, threadID, sessionID, "failed to load conversation")
		return
	}
	// The turn prompt is the user message SendMessage just persisted; everything before it
	// is the durable history (the sidecar only needs it if the SDK session can't resume).
	prompt := ""
	if n := len(history); n > 0 && history[n-1].Role == "user" {
		prompt = history[n-1].Content
		history = history[:n-1]
	}
	if strings.TrimSpace(prompt) == "" {
		s.fail(userID, threadID, sessionID, "failed to load conversation")
		return
	}

	// Ambient context: preload what the user is looking at into the system prompt so the
	// agent can answer surface questions instantly, without a tool round-trip.
	sysPrompt := s.systemPrompt(surface)
	if block := s.surfaceContext(ctx, userID, surface); block != "" {
		sysPrompt += "\n\n" + block
	}

	events, err := s.Agent.StartTurn(ctx, copilotagent.TurnRequest{
		TurnID:          sessionID,
		ThreadID:        threadID,
		ResumeSessionID: thread.SDKSessionID,
		UserBearer:      bearer,
		SystemPrompt:    sysPrompt,
		Prompt:          prompt,
		HistoryFallback: historyFallback(history),
		Model:           model,
		Effort:          effort,
		GatedTools:      gatedToolNames(),
		MaxTurns:        maxCopilotTurns,
	})
	if err != nil {
		slog.Error("copilot: start turn failed", "component", "copilot", "err", err)
		s.fail(userID, threadID, sessionID, "the assistant is unavailable right now")
		return
	}

	citations := map[string]Citation{}
	meta := CopilotMessageMeta{}
	final := ""
	sawDone := false
	for ev := range events {
		switch ev.Type {
		case "session":
			if ev.SessionID != "" {
				_ = s.Store.SetThreadSDKSession(ctx, threadID, ev.SessionID)
			}
		case "token":
			s.publish(userID, threadID, "token", copilotTokenPayload{sessionID, threadID, ev.Delta})
		case "tool_start":
			s.publish(userID, threadID, "tool", copilotToolPayload{
				SessionID: sessionID, ThreadID: threadID,
				Name: ev.Name, Label: toolLabel(ev.Name), InputSummary: toolInputSummary(ev.Name, ev.Input),
			})
		case "thinking":
			s.publish(userID, threadID, "thinking", copilotThinkingPayload{sessionID, threadID})
		case "tool_result":
			for _, c := range parseCitations(ev.Content) {
				citations[c.Kind+"|"+c.ID] = c
			}
			if len(meta.Activity) < maxActivityItems {
				meta.Activity = append(meta.Activity, CopilotActivityItem{Name: ev.Name, OK: !ev.IsError})
			}
			s.publish(userID, threadID, "tool_done", copilotToolDonePayload{
				SessionID: sessionID, ThreadID: threadID, Name: ev.Name, OK: !ev.IsError,
				ResultSummary: toolResultSummary(ev.Content, ev.IsError),
			})
			if ev.ApprovalID != "" {
				// The approved gated tool ran — settle the dock's approval card.
				message := "Done."
				if ev.IsError {
					message = firstLineOf(ev.Content, "the action failed")
				}
				s.publish(userID, threadID, "action_result", copilotActionResultPayload{
					ThreadID: threadID, ActionID: ev.ApprovalID, OK: !ev.IsError, Message: message,
				})
				s.dropPending(ev.ApprovalID)
			}
		case "proposed_action":
			s.registerPending(ev.ApprovalID, userID, threadID, sessionID, ev.Tool)
			s.publish(userID, threadID, "proposed_action", copilotProposedPayload{
				SessionID: sessionID, ThreadID: threadID, ActionID: ev.ApprovalID,
				Tool: ev.Tool, Summary: proposalSummary(ev.Tool, ev.Input),
			})
		case "approval_resolved":
			// Approved holds settle later via their tool_result; denials settle here.
			if !ev.Approved {
				message := "Dismissed."
				if ev.TimedOut {
					message = "Approval timed out."
				}
				s.publish(userID, threadID, "action_result", copilotActionResultPayload{
					ThreadID: threadID, ActionID: ev.ApprovalID, OK: false, Message: message,
				})
				s.dropPending(ev.ApprovalID)
			}
		case "message":
			final = ev.Text
		case "error":
			s.publish(userID, threadID, "error", copilotErrorPayload{
				SessionID: sessionID, ThreadID: threadID, Message: ev.Message, Code: ev.Code,
			})
		case "done":
			sawDone = true
			meta.NumTurns = ev.NumTurns
			meta.InputTokens = ev.Usage.InputTokens
			meta.OutputTokens = ev.Usage.OutputTokens
			meta.CostUSD = ev.CostUSD
			if ev.SessionID != "" {
				_ = s.Store.SetThreadSDKSession(ctx, threadID, ev.SessionID)
			}
		}
	}
	slog.Info("copilot: turn finished", "component", "copilot",
		"thread", threadID, "numTurns", meta.NumTurns, "costUsd", meta.CostUSD,
		"tools", len(meta.Activity), "clean", sawDone)

	if strings.TrimSpace(final) != "" {
		cites := make([]Citation, 0, len(citations))
		for _, c := range citations {
			cites = append(cites, c)
		}
		msgID := uuid.NewString()
		// Persist with a fresh ctx: the turn ctx may already be cancelled (user hit stop
		// right as the answer landed) and the final answer must not be lost.
		persistCtx, persistCancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = s.Store.AppendMessage(persistCtx, StoredCopilotMessage{
			ID: msgID, ThreadID: threadID, Role: "assistant", Content: final, Citations: cites, Meta: &meta,
		})
		_ = s.Store.TouchThread(persistCtx, threadID)
		persistCancel()
		s.publish(userID, threadID, "message", copilotMessagePayload{
			SessionID: sessionID, ThreadID: threadID, MessageID: msgID,
			Content: final, Citations: cites, Meta: &meta,
		})
	}

	// A cancelled/timed-out turn is a clean stop (the user hit stop), not an error; a
	// stream that broke without its `done` (sidecar died mid-turn) is.
	if !sawDone && ctx.Err() == nil {
		s.fail(userID, threadID, sessionID, "the assistant connection was interrupted")
		return
	}
	s.publish(userID, threadID, "done", copilotDonePayload{sessionID, threadID})
}

func (s *defaultCopilotService) fail(userID, threadID, sessionID, msg string) {
	s.publish(userID, threadID, "error", copilotErrorPayload{SessionID: sessionID, ThreadID: threadID, Message: msg})
	s.publish(userID, threadID, "done", copilotDonePayload{sessionID, threadID})
}

func (s *defaultCopilotService) publish(userID, threadID, op string, payload any) {
	_ = s.Realtime.Publish(context.Background(), PublishTarget{
		TenantID:     userID,
		Stream:       WorkspaceStream,
		ResourceKind: copilotResourceKind,
		ResourceID:   threadID,
		Operation:    op,
	}, payload)
}

// --- streaming payloads -------------------------------------------------------

type copilotTokenPayload struct {
	SessionID string `json:"sessionId"`
	ThreadID  string `json:"threadId"`
	Delta     string `json:"delta"`
}
type copilotToolPayload struct {
	SessionID    string `json:"sessionId"`
	ThreadID     string `json:"threadId"`
	Name         string `json:"name"`
	Label        string `json:"label"`
	InputSummary string `json:"inputSummary,omitempty"`
}
type copilotToolDonePayload struct {
	SessionID     string `json:"sessionId"`
	ThreadID      string `json:"threadId"`
	Name          string `json:"name"`
	OK            bool   `json:"ok"`
	ResultSummary string `json:"resultSummary,omitempty"`
}
type copilotThinkingPayload struct {
	SessionID string `json:"sessionId"`
	ThreadID  string `json:"threadId"`
}
type copilotMessagePayload struct {
	SessionID string              `json:"sessionId"`
	ThreadID  string              `json:"threadId"`
	MessageID string              `json:"messageId"`
	Content   string              `json:"content"`
	Citations []Citation          `json:"citations"`
	Meta      *CopilotMessageMeta `json:"meta,omitempty"`
}
type copilotDonePayload struct {
	SessionID string `json:"sessionId"`
	ThreadID  string `json:"threadId"`
}
type copilotErrorPayload struct {
	SessionID string `json:"sessionId"`
	ThreadID  string `json:"threadId"`
	Message   string `json:"message"`
	// Code lets the dock switch affordances (e.g. max_turns → a Continue button).
	Code string `json:"code,omitempty"`
}
type copilotProposedPayload struct {
	SessionID string `json:"sessionId"`
	ThreadID  string `json:"threadId"`
	ActionID  string `json:"actionId"`
	Tool      string `json:"tool"`
	Summary   string `json:"summary"`
}
type copilotActionResultPayload struct {
	ThreadID string `json:"threadId"`
	ActionID string `json:"actionId"`
	OK       bool   `json:"ok"`
	Message  string `json:"message"`
}

// parseCitations pulls the citations array out of a tool result's JSON content (the MCP
// workspace tools' convention: a top-level "citations" field). Non-JSON or citation-free
// results yield nothing.
func parseCitations(content string) []Citation {
	var probe struct {
		Citations []Citation `json:"citations"`
	}
	if err := json.Unmarshal([]byte(content), &probe); err != nil {
		return nil
	}
	return probe.Citations
}

// historyFallback renders the durable text history the sidecar falls back to when the SDK
// session can't resume: the most recent turns, capped.
func historyFallback(history []CopilotMessage) string {
	if len(history) == 0 {
		return ""
	}
	start := 0
	if len(history) > historyFallbackTurns {
		start = len(history) - historyFallbackTurns
	}
	var b strings.Builder
	for _, m := range history[start:] {
		role := "User"
		if m.Role == "assistant" {
			role = "Assistant"
		}
		fmt.Fprintf(&b, "%s: %s\n", role, m.Content)
	}
	out := b.String()
	if len(out) > historyFallbackChars {
		out = out[len(out)-historyFallbackChars:]
	}
	return strings.TrimSpace(out)
}

// firstLineOf compacts an error-ish tool result into a one-line message.
func firstLineOf(s, fallback string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return fallback
	}
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}

func toolInputSummary(tool string, input json.RawMessage) string {
	if len(input) == 0 || string(input) == "null" {
		return ""
	}
	var parsed any
	if err := json.Unmarshal(input, &parsed); err != nil {
		return capSummary(string(input))
	}
	sanitized := redactToolInput(parsed)
	if m, ok := sanitized.(map[string]any); ok {
		for _, key := range primaryToolInputKeys(tool) {
			if value, ok := m[key]; ok {
				if s := compactValue(value); s != "" {
					return key + ": " + capSummary(s)
				}
			}
		}
	}
	payload, err := json.Marshal(sanitized)
	if err != nil {
		return ""
	}
	return capSummary(string(payload))
}

func toolResultSummary(content string, isError bool) string {
	if strings.TrimSpace(content) == "" {
		if isError {
			return "failed"
		}
		return ""
	}
	fallback := "done"
	if isError {
		fallback = "failed"
	}
	return capSummary(firstLineOf(content, fallback))
}

func primaryToolInputKeys(tool string) []string {
	switch tool {
	case "search", "search_pages", "get_news":
		return []string{"query", "symbol"}
	case "get_quote", "get_bars", "create_alert", "add_to_watchlist":
		return []string{"symbol", "listName"}
	case "get_artifact", "get_page", "read_document", "search_document":
		return []string{"artifact_id", "artifactId", "page_id", "pageId", "query"}
	case "read_file", "write_file", "edit_file", "delete_file":
		return []string{"path", "page_id", "pageId"}
	case "create_app", "publish_app", "build_app", "preview_open", "preview_snapshot":
		return []string{"page_id", "pageId", "title"}
	default:
		return nil
	}
}

func redactToolInput(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			if isSensitiveKey(k) {
				out[k] = "[redacted]"
			} else {
				out[k] = redactToolInput(v)
			}
		}
		return out
	case []any:
		out := make([]any, 0, min(len(x), 6))
		for i, v := range x {
			if i >= 6 {
				out = append(out, "…")
				break
			}
			out = append(out, redactToolInput(v))
		}
		return out
	default:
		return x
	}
}

func isSensitiveKey(key string) bool {
	key = strings.ToLower(key)
	return strings.Contains(key, "token") ||
		strings.Contains(key, "secret") ||
		strings.Contains(key, "password") ||
		strings.Contains(key, "bearer") ||
		strings.Contains(key, "credential")
}

func compactValue(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case float64:
		return fmt.Sprintf("%g", x)
	case bool:
		return fmt.Sprintf("%t", x)
	default:
		payload, err := json.Marshal(x)
		if err != nil {
			return ""
		}
		return string(payload)
	}
}

func capSummary(s string) string {
	s = strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
	if len(s) > maxToolSummaryChars {
		return s[:maxToolSummaryChars] + "…"
	}
	return s
}

// proposalSummary renders a short human description of a proposed gated action.
func proposalSummary(tool string, input json.RawMessage) string {
	switch tool {
	case "delete_file":
		var a struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal(input, &a)
		if strings.TrimSpace(a.Path) != "" {
			return "Delete file " + a.Path + " from the shard"
		}
		return "Delete a shard file"
	case "publish_app":
		return "Publish the shard (make it live)"
	case "update_page":
		return "Replace the page's entire content"
	case "delete_block":
		return "Delete a block from the page"
	case "create_alert":
		var a struct {
			Symbol    string  `json:"symbol"`
			Direction string  `json:"direction"`
			Threshold float64 `json:"threshold"`
		}
		_ = json.Unmarshal(input, &a)
		if a.Symbol != "" {
			return fmt.Sprintf("Create a price alert: %s %s %.2f", strings.ToUpper(a.Symbol), a.Direction, a.Threshold)
		}
		return "Create a price alert"
	default:
		return "Run " + tool
	}
}

// toolLabel is the human-facing phrase shown while a tool runs (a "Searching…" affordance).
// Names are the MCP server's tool names.
func toolLabel(name string) string {
	switch name {
	case "search":
		return "Searching your workspace"
	case "get_entity":
		return "Looking up an entity"
	case "get_insights":
		return "Reading your insights"
	case "list_artifacts":
		return "Listing your artifacts"
	case "get_artifact":
		return "Reading an artifact"
	case "get_page", "list_pages":
		return "Reading a page"
	case "get_watchlist":
		return "Checking your watchlist"
	case "get_browser_tree", "list_folders":
		return "Browsing your workspace"
	case "search_pages":
		return "Searching your pages"
	case "get_bars":
		return "Reading price history"
	case "get_quote":
		return "Fetching a live quote"
	case "get_news":
		return "Reading market news"
	case "get_movers":
		return "Scanning today's movers"
	case "get_most_actives":
		return "Checking the most-actives"
	case "get_account":
		return "Reading your account"
	case "get_positions":
		return "Reading your positions"
	case "create_alert":
		return "Setting a price alert"
	case "list_alerts":
		return "Checking your alerts"
	case "delete_alert":
		return "Removing an alert"
	case "create_app":
		return "Creating a shard"
	case "list_dir", "read_file":
		return "Reading shard files"
	case "write_file", "edit_file":
		return "Writing shard code"
	case "install_lib":
		return "Adding a shard dependency"
	case "build_app":
		return "Building the shard"
	case "delete_file":
		return "Deleting a shard file"
	case "publish_app":
		return "Publishing the shard"
	case "preview_open", "preview_navigate", "preview_snapshot", "preview_screenshot",
		"preview_eval", "preview_click", "preview_console", "preview_close", "preview_restart":
		return "Previewing the shard"
	case "create_page":
		return "Creating a page"
	case "insert_blocks", "update_block", "update_page":
		return "Editing the page"
	case "delete_block":
		return "Removing a block"
	case "add_to_watchlist":
		return "Updating your watchlist"
	case "list_watchlists":
		return "Listing your watchlists"
	case "create_watchlist":
		return "Creating a watchlist"
	case "draw_edge":
		return "Linking entities"
	default:
		return "Working"
	}
}

// --- system prompt ------------------------------------------------------------

func (s *defaultCopilotService) systemPrompt(surface CopilotSurface) string {
	var b strings.Builder
	b.WriteString(`You are Aladin's Copilot — a research assistant for a personal algo/swing-trading workspace (US equities).
Ground every answer in the user's own Aladin data by calling the available tools before answering; do not invent tickers, entities, prices, pages, or shards.
The workspace holds several artifact kinds: pages (the user's writing), shards (agent-built interactive docs; artifact type "app"), links, files, and voice notes. To read whatever the user currently has open, call get_artifact with its id — it works for ANY kind, including shards. Do not claim you can only see pages; use get_artifact.
You CAN create and author shards: create_app to make one (its result includes an authoring_guide — the @aladin/kit component reference; FOLLOW it exactly and only use components/props it lists — plus the seeded current_index_tsx), write_file/edit_file to author its files (each write auto-builds and returns diagnostics in build — if build.ok is false, read build.log, fix the exact file, and write again until it builds), build_app to compile the publishable bundle. When EDITING an existing shard, call get_authoring_guide(page_id) FIRST — it returns the same component reference plus that shard's files and anchors.json, so you edit against what the kit really exports instead of guessing. write_file only creates: replacing an existing file needs overwrite:true (prefer edit_file for targeted changes). Preview with preview_open then preview_snapshot to confirm it rendered, then verify_app to check every declared anchor is really in the DOM and every ref resolves; publish_app makes it live (that step asks the user to approve) and refuses if verification fails. Shards are React apps composed from @aladin/kit (Page/Section/Region, DataTable/MetricRow, AppShell, Quiz/Timer/Checklist, …) styled with Tailwind + Aladin token classes (bg-panel, text-ink, text-amber, …); they follow the app's theme automatically, can persist their own state with useShardState/useKV, and can read workspace entities they declare in anchors.json refs. When asked to make a shard, actually create it and write the content — don't just output an outline. Only create_app for a brand-NEW shard; if the user is already viewing a shard (or asks to update/add to "the shard"/"this"), edit that EXISTING shard by its id (get_authoring_guide → read_file → write_file/edit_file with its page_id) instead of creating another. Same rule for pages.
You CAN also author pages (the user's writing): create_page from markdown; for edits, get_page to see the blocks with their ids, then insert_blocks / update_block for surgical changes (update_page replaces the whole body and delete_block removes content — both ask the user to approve). And light actions: add_to_watchlist (by symbol; optional list name — watchlists are named instrument sets and a user can keep several, e.g. "Tech" or "Shorts"; the list is created if it doesn't exist; omit the name for the default list), create_watchlist, list_watchlists, draw_edge (link two entities).
Market intelligence tools: get_news (explain WHY a stock moved — a catalyst vs. a liquidity move — never just that it did), get_movers and get_most_actives (what's moving / where liquidity is, no symbol needed), and READ-ONLY account state: get_account (cash/equity/buying power) and get_positions (actual holdings + unrealized P&L — reason about the user's REAL exposure, not just the watchlist). You cannot place or modify orders. If get_account.paper is true, say the numbers are from a paper (simulated) account. There is no VIX or index feed — use ETF proxies (SPY, QQQ, IWM, DIA, VIXY; sector SPDRs XLK/XLF/XLE/XLV/XLY/XLP/XLI/XLU/XLB/XLRE/XLC). There is no earnings-calendar or fundamentals tool — say so plainly rather than inventing dates or EPS.
You can also set PRICE ALERTS: create_alert (symbol, direction "above"/"below", threshold) — it's recurring and self-re-arms (fires on a genuine cross with momentum, then waits for a real pullback before it can fire again, so no jitter spam). Creating one asks the user to approve first. When it fires, the user gets a notification. list_alerts and delete_alert manage them. If the price is already past the threshold at creation, the alert is still set but won't fire until it pulls back and crosses again — tell the user that.
Prefer specific, concise answers. When you reference an entity, artifact, or ticker, use the tool that fetches it so the app can cite it.
If the tools return nothing relevant, say so plainly rather than guessing.
If a tool returns an error, tell the user the EXACT error message verbatim and what you were trying to do — never vaguely say "a technical issue" or claim the action is impossible. The capability exists; a specific error means something is misconfigured (e.g. a service is down) and the exact text helps fix it.

You may enrich final answers with Aladin markdown directives. These directives are declarative data only: never put HTML, JavaScript, CSS, external URLs, or secrets in them. Keep ordinary prose readable before/after the block, and use a directive only when it helps the user inspect or act on workspace state.
Supported directives:
- ::aladin-artifact{id="artifact_id" kind="page|shard|document|artifact" title="Title"} for workspace objects you fetched or created.
- ::aladin-ticker{symbol="NVDA"} for tickers you fetched.
- :::aladin-activity ... ::: with a fenced JSON body: [{"label":"Read shard files","status":"ok|running|error","detail":"optional","inputSummary":"optional","resultSummary":"optional"}].
- :::aladin-actions ... ::: with a fenced JSON body: [{"label":"Continue","action":"continue"},{"label":"Retry","action":"retry","prompt":"try again"},{"label":"Open shard","action":"open_artifact","artifactId":"...","kind":"shard"},{"label":"Open NVDA","action":"open_ticker","symbol":"NVDA"}].
- :::aladin-approval ... ::: with a fenced JSON body: {"action":"Publish shard","target":"Shard title","status":"pending|approved|rejected|expired","risk":"what changes","details":["exact action"]} when summarizing a pending or completed gated action.
- :::aladin-diff ... ::: with fenced JSON {"title":"Update","path":"src/index.tsx","lines":[{"kind":"context|add|remove","text":"..."}]} or a fenced short unified diff when showing edits.
- :::aladin-shard-preview ... ::: with a fenced JSON body: {"artifactId":"...","title":"Shard title","status":"building|ready|published|error","previewUrl":"/local/path","diagnostics":["bounded build messages"]} after build/preview/publish work.
- :::aladin-error-recovery ... ::: with a fenced JSON body: {"title":"Build failed","message":"exact error","code":"optional","actions":[...same action schema...]} for recoverable errors.
Prefer aladin-activity for multi-step work, aladin-diff for material edits, aladin-shard-preview after shard builds/previews, and aladin-error-recovery when a user can retry or open context.
Rich directive trigger rules:
- After create_app, read_file, write_file, edit_file, build_app, preview_open, preview_snapshot, publish_app, or publish approval, include an aladin-activity summary and an aladin-shard-preview block when you know the shard id/title/status.
- After update_page, insert_blocks, update_block, delete_block, write_file, or edit_file, include aladin-diff for the most important bounded change.
- When a gated action is pending, approved, rejected, expired, or failed, include aladin-approval with exact action/target/risk/status details.
- When build, preview, publish, or edit work fails but the user can retry or inspect context, include aladin-error-recovery with the exact error text and a retry/continue/open action.`)
	if hint := surfaceHint(surface); hint != "" {
		b.WriteString("\n\n")
		b.WriteString(hint)
	}
	return b.String()
}

func surfaceHint(s CopilotSurface) string {
	switch strings.TrimSpace(s.Kind) {
	case "ticker":
		if s.Symbol != "" {
			return "The user is currently looking at the ticker " + strings.ToUpper(s.Symbol) + ". Assume questions are about it unless stated otherwise."
		}
	case "entity":
		if s.ID != "" {
			label := s.Label
			if label == "" {
				label = "entity " + s.ID
			}
			return "The user is currently viewing " + label + " (entity id " + s.ID + "). Use get_entity for details."
		}
	case "artifact", "page", "shard":
		if s.ID != "" {
			return "The user is currently viewing an artifact (a page, shard, link, or file), id " + s.ID +
				". Call get_artifact with that id to read it. If they ask to change/add to it, EDIT THIS artifact by its id — never create a new one."
		}
	case "markets":
		return "The user is on the Markets surface (their watchlist). Consider get_watchlist."
	}
	return ""
}

// surfaceContext builds a compact "current context" block from what the user is looking at,
// so the agent can answer about the current surface WITHOUT a tool round-trip. Best-effort:
// any missing field or fetch error yields an empty (skipped) block.
func (s *defaultCopilotService) surfaceContext(ctx context.Context, userID string, surface CopilotSurface) string {
	switch strings.TrimSpace(surface.Kind) {
	case "ticker":
		sym := strings.ToUpper(strings.TrimSpace(surface.Symbol))
		if sym == "" || s.Snapshots == nil {
			return ""
		}
		q, ok, err := s.Snapshots.FetchSnapshot(ctx, sym)
		if err != nil || !ok {
			return ""
		}
		return fmt.Sprintf("Current context — the user is viewing ticker %s. Latest snapshot: last %.2f, previous close %.2f, change %.2f%%.",
			sym, q.Last, q.PrevClose, q.ChangePct)

	case "artifact", "page", "shard":
		if strings.TrimSpace(surface.ID) == "" || s.Artifacts == nil {
			return ""
		}
		art, err := s.Artifacts.Get(ctx, surface.ID)
		if err != nil {
			return ""
		}
		body := strings.TrimSpace(art.Content)
		if len(art.Blocks) > 0 {
			if text, terr := blocknote.ExtractText(art.Blocks); terr == nil && strings.TrimSpace(text) != "" {
				body = strings.TrimSpace(text)
			}
		}
		var edit string
		switch art.Type {
		case "app":
			edit = fmt.Sprintf(" To change it, EDIT THIS shard — read_file/write_file/edit_file with page_id=%q. Do NOT create a new shard.", surface.ID)
		case "file":
			// A file is a SOURCE, not editable prose. Without this branch it fell into the
			// page case below and the agent was told to "EDIT THIS page — get_page then
			// insert_blocks", i.e. to rewrite the user's own PDF as a BlockNote document:
			// every one of those calls can only fail, and the instruction is the opposite of
			// what a source is for. Point it at the document tools instead (INGESTION_PRD's
			// retrieval contract) and at citing pages.
			edit = fmt.Sprintf(" It is a SOURCE the user is reading, NOT an editable page."+
				" If it has been ingested, read it with read_document / search_document (artifact_id=%q)"+
				" and cite page numbers when you answer. Never call get_page, insert_blocks, update_block"+
				" or update_page on it, and never try to rewrite it.", surface.ID)
		default:
			edit = fmt.Sprintf(" To change it, EDIT THIS page — get_page then insert_blocks/update_block/update_page with page id %q. Do NOT create a new page.", surface.ID)
		}
		header := fmt.Sprintf("Current context — the user is viewing the %s %q (id %s).%s", artifactNoun(art.Type), art.Title, surface.ID, edit)
		if body == "" {
			return header
		}
		return capText(header+" Its content:\n"+body, maxSurfaceContextChars)

	case "entity":
		if strings.TrimSpace(surface.ID) == "" || s.Entities == nil {
			return ""
		}
		ec, err := s.Entities.Get(ctx, userID, surface.ID)
		if err != nil {
			return ""
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Current context — the user is viewing the entity %q (%s).", ec.Entity.Name, ec.Entity.Kind)
		if g := strings.TrimSpace(ec.Entity.Gist); g != "" {
			fmt.Fprintf(&b, " %s", g)
		}
		rels := make([]string, 0, 5)
		for _, e := range ec.Edges {
			if len(rels) >= 5 {
				break
			}
			if strings.TrimSpace(e.To) != "" {
				rels = append(rels, e.To)
			}
		}
		if len(rels) > 0 {
			fmt.Fprintf(&b, " Related: %s.", strings.Join(rels, ", "))
		}
		return capText(b.String(), maxSurfaceContextChars)

	case "markets":
		if s.Watchlist == nil {
			return ""
		}
		items, err := s.Watchlist.List(ctx, userID)
		if err != nil || len(items) == 0 {
			return ""
		}
		syms := make([]string, 0, len(items))
		for _, it := range items {
			syms = append(syms, it.Symbol)
		}
		return capText("Current context — the user is on the Markets surface. Watchlist: "+strings.Join(syms, ", ")+".", maxSurfaceContextChars)
	}
	return ""
}

func artifactNoun(t string) string {
	switch t {
	case "app":
		return "shard"
	case "page":
		return "page"
	case "file":
		// "the artifact X" told the agent nothing about what it was allowed to do with it.
		return "source file"
	default:
		return "artifact"
	}
}

// threadTitle derives a short thread title from the first user message.
func threadTitle(text string) string {
	t := strings.TrimSpace(text)
	if len(t) > 60 {
		t = t[:60] + "…"
	}
	return t
}

func capText(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + `…(truncated)`
}
