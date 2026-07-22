package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"aladin/backend_v2/internal/blocknote"
	"aladin/backend_v2/internal/llm"

	"github.com/google/uuid"
)

// CopilotService is Aladin's default agentic LLM interface: a surface-aware, streaming
// assistant that grounds its answers on the user's own Aladin data via read-only tools.
//
// A turn runs asynchronously — SendMessage persists the user turn, spawns the agent loop
// in a goroutine, and returns a sessionId immediately. The loop streams token/tool/message/
// done events over the realtime workspace stream (resource kind "copilot"), so the dock
// renders live; the final answer is persisted so a reconnecting client can replay it.
type CopilotService interface {
	SendMessage(ctx context.Context, in CopilotSendInput) (CopilotSendResult, error)
	ListThreads(ctx context.Context, userID string) ([]CopilotThread, error)
	GetThread(ctx context.Context, userID, threadID string) (CopilotThreadDetail, error)
	// Cancel stops an in-flight turn (halting the work + cost), scoped to its owner.
	Cancel(ctx context.Context, userID, sessionID string) error
	// ApproveAction runs a previously-proposed destructive action; RejectAction discards it.
	ApproveAction(ctx context.Context, userID, actionID string) error
	RejectAction(ctx context.Context, userID, actionID string) error
	// Configured reports whether an LLM backend is wired (OpenAI key present).
	Configured() bool
}

// destructiveActions are tools whose effects are hard to reverse — they are NOT executed inline;
// the agent proposes them and the user approves via the dock. Additive/iterative tools run
// directly. (Extended as page/publish tools land.)
var destructiveActions = map[string]bool{
	"delete_shard_file": true,
	"publish_shard":     true,
	"update_page":       true, // full-document replace wipes block ids
	"delete_block":      true,
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
	ThreadID  string // "" ⇒ start a new thread
	Text      string
	Surface   CopilotSurface
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
}

type CopilotThread struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	UpdatedAt string `json:"updatedAt"`
}

type CopilotMessage struct {
	ID        string     `json:"id"`
	Role      string     `json:"role"` // user | assistant
	Content   string     `json:"content"`
	Citations []Citation `json:"citations"`
	CreatedAt string     `json:"createdAt"`
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
}

// CopilotStore persists threads + the visible conversation. Ownership is enforced by
// passing userID on the scoped reads.
type CopilotStore interface {
	CreateThread(ctx context.Context, id, userID, title string) error
	TouchThread(ctx context.Context, threadID string) error
	ListThreads(ctx context.Context, userID string) ([]CopilotThread, error)
	// GetThread returns the thread iff it belongs to userID (found=false otherwise).
	GetThread(ctx context.Context, userID, threadID string) (CopilotThread, bool, error)
	AppendMessage(ctx context.Context, m StoredCopilotMessage) error
	ListMessages(ctx context.Context, threadID string) ([]CopilotMessage, error)
}

// CopilotDeps are the collaborators the agent loop needs: the LLM, the realtime hub for
// streaming, the store for persistence, and the read services exposed as tools. Agent may
// be nil (no OpenAI key) ⇒ the service reports Configured()=false and rejects sends cleanly.
type CopilotDeps struct {
	Store     CopilotStore
	Agent     llm.ChatAgent
	Realtime  RealtimeEventService
	Search    SearchService
	Entities  EntityContextService
	Insights  InsightService
	Artifacts ArtifactService
	Pages     PageService
	Watchlist WatchlistService
	Bars      BarService
	Snapshots QuoteSnapshotSource // optional: live quote seed for get_quote
	// Shard authoring (doc-surface): all already wired in the API process.
	DocStore   DocSurfaceStore
	ShardBuild ShardBuildService
	Preview    PreviewService // optional: headless preview loop (degrades without Chrome)
	// Page authoring: the blocknote sidecar (markdown⇄blocks + live-doc bridge). Both are the
	// same *blocknote.Client; nil ⇒ page tools are not exposed. Instruments resolves symbols.
	Converter   blocknote.Converter
	Bridge      blocknote.Bridge
	Instruments InstrumentService
}

const (
	copilotResourceKind  = "copilot"
	maxCopilotIterations = 6
	copilotTurnTimeout   = 120 * time.Second
	maxToolResultChars   = 12000
)

// runningTurn is a live agent turn's cancel handle, kept so the owner can stop it.
type runningTurn struct {
	userID string
	cancel context.CancelFunc
}

// pendingAction is a proposed destructive tool call awaiting the user's approval.
type pendingAction struct {
	userID   string
	threadID string
	tool     string
	args     string
	created  time.Time
}

const pendingActionTTL = 30 * time.Minute

type defaultCopilotService struct {
	CopilotDeps
	mu      sync.Mutex
	running map[string]runningTurn   // sessionID → cancel handle
	pending map[string]pendingAction // actionID → proposed destructive action
}

func NewCopilotService(deps CopilotDeps) CopilotService {
	return &defaultCopilotService{
		CopilotDeps: deps,
		running:     map[string]runningTurn{},
		pending:     map[string]pendingAction{},
	}
}

func (s *defaultCopilotService) Configured() bool { return s.Agent != nil }

// Cancel stops the in-flight turn for sessionID if it belongs to userID. Idempotent: cancelling
// an already-finished (or unknown) session is a no-op.
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
	return nil
}

// registerPending stores a proposed destructive action and returns its id, pruning stale entries.
func (s *defaultCopilotService) registerPending(userID, threadID, tool, args string) string {
	actionID := uuid.NewString()
	s.mu.Lock()
	for id, pa := range s.pending {
		if time.Since(pa.created) > pendingActionTTL {
			delete(s.pending, id)
		}
	}
	s.pending[actionID] = pendingAction{userID: userID, threadID: threadID, tool: tool, args: args, created: time.Now()}
	s.mu.Unlock()
	return actionID
}

// ApproveAction executes a proposed destructive action (owner-scoped) and streams the result.
func (s *defaultCopilotService) ApproveAction(ctx context.Context, userID, actionID string) error {
	s.mu.Lock()
	pa, ok := s.pending[actionID]
	if ok {
		delete(s.pending, actionID)
	}
	s.mu.Unlock()
	if !ok || pa.userID != userID {
		return ErrNotFound
	}
	_, _, err := s.runTool(ctx, userID, pa.tool, pa.args)
	message := "Done."
	if err != nil {
		message = err.Error()
	}
	s.publish(userID, pa.threadID, "action_result", copilotActionResultPayload{
		ThreadID: pa.threadID, ActionID: actionID, OK: err == nil, Message: message,
	})
	return err
}

// RejectAction discards a proposed action. Idempotent.
func (s *defaultCopilotService) RejectAction(_ context.Context, userID, actionID string) error {
	s.mu.Lock()
	pa, ok := s.pending[actionID]
	if ok && pa.userID == userID {
		delete(s.pending, actionID)
	}
	s.mu.Unlock()
	if ok && pa.userID == userID {
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

	if err := s.Store.AppendMessage(ctx, StoredCopilotMessage{
		ID: uuid.NewString(), ThreadID: threadID, Role: "user", Content: text,
	}); err != nil {
		return CopilotSendResult{}, err
	}
	_ = s.Store.TouchThread(ctx, threadID)

	sessionID := uuid.NewString()
	go s.runAgent(in.Principal, threadID, sessionID, in.Surface)
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

// runAgent is the tool-use loop, run in its own goroutine. It streams over the realtime
// hub and persists the final assistant turn.
func (s *defaultCopilotService) runAgent(principal Principal, threadID, sessionID string, surface CopilotSurface) {
	// The tool ctx carries the principal so principal-scoped tools (pages/artifacts) work,
	// with a hard timeout so a stuck turn can't leak a goroutine.
	ctx, cancel := context.WithTimeout(WithPrincipal(context.Background(), principal), copilotTurnTimeout)
	defer cancel()
	userID := principal.UserID

	// Register the cancel handle so the owner can stop this turn; deregister on exit.
	s.mu.Lock()
	s.running[sessionID] = runningTurn{userID: userID, cancel: cancel}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.running, sessionID)
		s.mu.Unlock()
	}()

	history, err := s.Store.ListMessages(ctx, threadID)
	if err != nil {
		s.fail(userID, threadID, sessionID, "failed to load conversation")
		return
	}

	// Ambient context: preload what the user is looking at into the system prompt so the agent
	// can answer surface questions instantly, without a tool round-trip.
	sysPrompt := s.systemPrompt(surface)
	if block := s.surfaceContext(ctx, userID, surface); block != "" {
		sysPrompt += "\n\n" + block
	}
	messages := []llm.ChatMessage{{Role: llm.ChatRoleSystem, Content: sysPrompt}}
	for _, m := range history {
		role := llm.ChatRoleUser
		if m.Role == "assistant" {
			role = llm.ChatRoleAssistant
		}
		messages = append(messages, llm.ChatMessage{Role: role, Content: m.Content})
	}

	citations := map[string]Citation{}
	final := ""
	for iter := 0; iter < maxCopilotIterations; iter++ {
		assistant, err := s.Agent.Chat(ctx, messages, s.toolDefs(), func(ev llm.ChatEvent) {
			switch ev.Kind {
			case llm.ChatEventText:
				s.publish(userID, threadID, "token", copilotTokenPayload{sessionID, threadID, ev.Text})
			case llm.ChatEventToolCall:
				s.publish(userID, threadID, "tool", copilotToolPayload{
					SessionID: sessionID, ThreadID: threadID,
					Name: ev.ToolCall.Name, Label: toolLabel(ev.ToolCall.Name),
				})
			}
		})
		if err != nil {
			// A cancelled/timed-out turn is a clean stop (the user hit stop), not an error.
			if ctx.Err() != nil {
				s.publish(userID, threadID, "done", copilotDonePayload{sessionID, threadID})
				return
			}
			s.fail(userID, threadID, sessionID, "the assistant hit an error")
			return
		}
		messages = append(messages, assistant)
		if len(assistant.ToolCalls) == 0 {
			final = assistant.Content
			break
		}
		for _, tc := range assistant.ToolCalls {
			// Destructive tools are not run inline — propose them for the user to approve.
			if destructiveActions[tc.Name] {
				actionID := s.registerPending(userID, threadID, tc.Name, tc.Arguments)
				s.publish(userID, threadID, "proposed_action", copilotProposedPayload{
					SessionID: sessionID, ThreadID: threadID, ActionID: actionID,
					Tool: tc.Name, Summary: proposalSummary(tc.Name, tc.Arguments),
				})
				messages = append(messages, llm.ChatMessage{
					Role:       llm.ChatRoleTool,
					Content:    `{"status":"proposed","note":"awaiting user approval — do not assume it happened; tell the user you proposed it"}`,
					ToolCallID: tc.ID,
				})
				continue
			}
			result, cites, terr := s.runTool(ctx, userID, tc.Name, tc.Arguments)
			if terr != nil {
				result = fmt.Sprintf(`{"error":%q}`, terr.Error())
			}
			for _, c := range cites {
				citations[c.Kind+"|"+c.ID] = c
			}
			messages = append(messages, llm.ChatMessage{Role: llm.ChatRoleTool, Content: result, ToolCallID: tc.ID})
		}
	}
	if strings.TrimSpace(final) == "" {
		final = "I couldn't finish that within the step limit — try narrowing the question."
	}

	cites := make([]Citation, 0, len(citations))
	for _, c := range citations {
		cites = append(cites, c)
	}

	msgID := uuid.NewString()
	_ = s.Store.AppendMessage(ctx, StoredCopilotMessage{
		ID: msgID, ThreadID: threadID, Role: "assistant", Content: final, Citations: cites,
	})
	_ = s.Store.TouchThread(ctx, threadID)
	s.publish(userID, threadID, "message", copilotMessagePayload{sessionID, threadID, msgID, final, cites})
	s.publish(userID, threadID, "done", copilotDonePayload{sessionID, threadID})
}

func (s *defaultCopilotService) fail(userID, threadID, sessionID, msg string) {
	s.publish(userID, threadID, "error", copilotErrorPayload{sessionID, threadID, msg})
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
	SessionID string `json:"sessionId"`
	ThreadID  string `json:"threadId"`
	Name      string `json:"name"`
	Label     string `json:"label"`
}
type copilotMessagePayload struct {
	SessionID string     `json:"sessionId"`
	ThreadID  string     `json:"threadId"`
	MessageID string     `json:"messageId"`
	Content   string     `json:"content"`
	Citations []Citation `json:"citations"`
}
type copilotDonePayload struct {
	SessionID string `json:"sessionId"`
	ThreadID  string `json:"threadId"`
}
type copilotErrorPayload struct {
	SessionID string `json:"sessionId"`
	ThreadID  string `json:"threadId"`
	Message   string `json:"message"`
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

// proposalSummary renders a short human description of a proposed destructive action.
func proposalSummary(tool, args string) string {
	switch tool {
	case "delete_shard_file":
		var a struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal([]byte(args), &a)
		if strings.TrimSpace(a.Path) != "" {
			return "Delete file " + a.Path + " from the shard"
		}
		return "Delete a shard file"
	case "publish_shard":
		return "Publish the shard (make it live)"
	case "update_page":
		return "Replace the page's entire content"
	case "delete_block":
		return "Delete a block from the page"
	default:
		return "Run " + tool
	}
}

// --- system prompt ------------------------------------------------------------

func (s *defaultCopilotService) systemPrompt(surface CopilotSurface) string {
	var b strings.Builder
	b.WriteString(`You are Aladin's Copilot — a research assistant for a personal algo/swing-trading workspace (US equities).
Ground every answer in the user's own Aladin data by calling the available tools before answering; do not invent tickers, entities, prices, pages, or shards.
The workspace holds several artifact kinds: pages (the user's writing), shards (agent-built interactive docs; artifact type "app"), links, files, and voice notes. To read whatever the user currently has open, call get_artifact with its id — it works for ANY kind, including shards. Do not claim you can only see pages; use get_artifact.
You CAN create and author shards: create_shard to make one, write_shard_file/edit_shard_file to author it (each write auto-builds and returns diagnostics — read them and fix errors), build_shard to recompile. Preview it with preview_open then preview_snapshot to confirm it rendered; publish_shard makes it live (that step asks the user to approve). Shards are React apps composed from @aladin/kit (Page/Section/Region) styled with Tailwind + Aladin token classes (bg-panel, text-ink, text-amber, …). When asked to make a shard, actually create it and write the content — don't just output an outline.
You CAN also author pages (the user's writing): create_page from markdown; for edits, get_page_blocks to find block ids, then insert_blocks / update_block for surgical changes (update_page replaces the whole body and delete_block remove content — both ask the user to approve). And light actions: add_to_watchlist (by symbol), draw_edge (link two entities).
Prefer specific, concise answers. When you reference an entity, artifact, or ticker, use the tool that fetches it so the app can cite it.
If the tools return nothing relevant, say so plainly rather than guessing.`)
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
			return "The user is currently viewing an artifact (a page, shard, link, or file), id " + s.ID + ". Call get_artifact with that id to read it before answering questions about it."
		}
	case "markets":
		return "The user is on the Markets surface (their watchlist). Consider get_watchlist."
	}
	return ""
}

// threadTitle derives a short thread title from the first user message.
func threadTitle(text string) string {
	t := strings.TrimSpace(text)
	if len(t) > 60 {
		t = t[:60] + "…"
	}
	return t
}

func jsonString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return capText(string(b), maxToolResultChars)
}

func capText(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + `…(truncated)`
}
