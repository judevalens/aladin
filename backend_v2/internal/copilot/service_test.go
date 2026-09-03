package copilot

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"aladin/backend_v2/internal/market"
	"time"

	"aladin/backend_v2/internal/copilotagent"
	rtruntime "aladin/backend_v2/internal/realtime"
	coreservice "aladin/backend_v2/internal/service"
)

type SubscriptionKey = rtruntime.SubscriptionKey

const (
	WorkspaceStream = rtruntime.WorkspaceStream
	AnyResource     = rtruntime.AnyResource
)

func NewSubscriptionKeyResolver() *rtruntime.SubscriptionKeyResolver {
	return rtruntime.NewSubscriptionKeyResolver(
		func(ctx context.Context) (string, error) {
			principal, err := coreservice.RequirePrincipal(ctx)
			return principal.UserID, err
		},
		func(ctx context.Context) error {
			return coreservice.RequireScope(ctx, coreservice.ScopeArtifactsRead)
		},
		func(message string) error { return coreservice.BadRequest(message) },
	)
}

func NewInMemoryRealtimeEventService(resolver rtruntime.KeyResolver) *rtruntime.Service {
	return rtruntime.NewService(resolver)
}

type fakeSnapshot struct{}

func (fakeSnapshot) FetchSnapshot(_ context.Context, sym string) (market.Quote, bool, error) {
	return market.Quote{Symbol: sym, Last: 184.5, PrevClose: 180.0, ChangePct: 2.5}, true, nil
}

// TestCopilotSurfaceContext — ambient context: a ticker surface preloads the live snapshot,
// and a surface with no data source yields an empty (skipped) block.
func TestCopilotSurfaceContext(t *testing.T) {
	withSnap := &defaultCopilotService{CopilotDeps: CopilotDeps{Snapshots: fakeSnapshot{}}}
	block := withSnap.surfaceContext(context.Background(), "u1", CopilotSurface{Kind: "ticker", Symbol: "nvda"})
	if !strings.Contains(block, "NVDA") || !strings.Contains(block, "184.50") {
		t.Fatalf("expected NVDA snapshot in context, got %q", block)
	}

	noSnap := &defaultCopilotService{CopilotDeps: CopilotDeps{}}
	if got := noSnap.surfaceContext(context.Background(), "u1", CopilotSurface{Kind: "ticker", Symbol: "NVDA"}); got != "" {
		t.Fatalf("expected empty context with no snapshot source, got %q", got)
	}
}

// fakeAgent is a scripted CopilotAgent (the sidecar client surface): StartTurn streams a
// canned NDJSON event sequence, and it records the turn request + any approve/reject/cancel
// calls. In block mode it never emits — it holds until the turn ctx is cancelled (for the
// cancel test).
type fakeAgent struct {
	mu        sync.Mutex
	events    []copilotagent.Event
	block     bool
	started   chan struct{}
	startOnce sync.Once
	lastReq   copilotagent.TurnRequest
	resolved  []resolveCall
	cancelled []string
}

type resolveCall struct {
	turnID     string
	approvalID string
	approved   bool
}

func (a *fakeAgent) StartTurn(ctx context.Context, req copilotagent.TurnRequest) (<-chan copilotagent.Event, error) {
	a.mu.Lock()
	a.lastReq = req
	a.mu.Unlock()
	if a.started != nil {
		a.startOnce.Do(func() { close(a.started) })
	}
	ch := make(chan copilotagent.Event, len(a.events)+1)
	go func() {
		defer close(ch)
		if a.block {
			<-ctx.Done()
			return
		}
		for _, ev := range a.events {
			select {
			case ch <- ev:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

func (a *fakeAgent) Cancel(_ context.Context, turnID string) error {
	a.mu.Lock()
	a.cancelled = append(a.cancelled, turnID)
	a.mu.Unlock()
	return nil
}

func (a *fakeAgent) Healthz(_ context.Context) (copilotagent.Health, error) {
	return copilotagent.Health{OK: true, MCP: true}, nil
}

func (a *fakeAgent) ResolveApproval(_ context.Context, turnID, approvalID string, approved bool) error {
	a.mu.Lock()
	a.resolved = append(a.resolved, resolveCall{turnID, approvalID, approved})
	a.mu.Unlock()
	return nil
}

func (a *fakeAgent) request() copilotagent.TurnRequest {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.lastReq
}

func (a *fakeAgent) resolveCalls() []resolveCall {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]resolveCall, len(a.resolved))
	copy(out, a.resolved)
	return out
}

type fakeCopilotStore struct {
	mu          sync.Mutex
	owners      map[string]string
	titles      map[string]string
	archived    map[string]bool
	pinned      map[string]bool
	msgs        map[string][]CopilotMessage
	sdkSessions map[string]string
	touches     int
}

func newFakeStore() *fakeCopilotStore {
	return &fakeCopilotStore{
		owners:      map[string]string{},
		titles:      map[string]string{},
		archived:    map[string]bool{},
		pinned:      map[string]bool{},
		msgs:        map[string][]CopilotMessage{},
		sdkSessions: map[string]string{},
	}
}

func (s *fakeCopilotStore) CreateThread(_ context.Context, id, userID, title string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.owners[id] = userID
	s.titles[id] = title
	return nil
}
func (s *fakeCopilotStore) TouchThread(_ context.Context, _ string) error {
	s.mu.Lock()
	s.touches++
	s.mu.Unlock()
	return nil
}
func (s *fakeCopilotStore) ListThreads(_ context.Context, _ string) ([]CopilotThread, error) {
	return nil, nil
}
func (s *fakeCopilotStore) GetThread(_ context.Context, userID, threadID string) (CopilotThread, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.owners[threadID] != userID || s.archived[threadID] {
		return CopilotThread{}, false, nil
	}
	return CopilotThread{ID: threadID, Title: s.titles[threadID], SDKSessionID: s.sdkSessions[threadID], Pinned: s.pinned[threadID]}, true, nil
}
func (s *fakeCopilotStore) RenameThread(_ context.Context, userID, threadID, title string) (CopilotThread, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.owners[threadID] != userID || s.archived[threadID] {
		return CopilotThread{}, false, nil
	}
	s.titles[threadID] = title
	return CopilotThread{ID: threadID, Title: title, SDKSessionID: s.sdkSessions[threadID], Pinned: s.pinned[threadID]}, true, nil
}
func (s *fakeCopilotStore) ArchiveThread(_ context.Context, userID, threadID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.owners[threadID] != userID || s.archived[threadID] {
		return false, nil
	}
	s.archived[threadID] = true
	return true, nil
}
func (s *fakeCopilotStore) SetThreadPinned(_ context.Context, userID, threadID string, pinned bool) (CopilotThread, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.owners[threadID] != userID || s.archived[threadID] {
		return CopilotThread{}, false, nil
	}
	s.pinned[threadID] = pinned
	return CopilotThread{ID: threadID, Title: s.titles[threadID], SDKSessionID: s.sdkSessions[threadID], Pinned: pinned}, true, nil
}
func (s *fakeCopilotStore) SetThreadSDKSession(_ context.Context, threadID, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sdkSessions[threadID] = sessionID
	return nil
}
func (s *fakeCopilotStore) AppendMessage(_ context.Context, m StoredCopilotMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.msgs[m.ThreadID] = append(s.msgs[m.ThreadID], CopilotMessage{
		ID: m.ID, Role: m.Role, Content: m.Content, Citations: m.Citations, Meta: m.Meta,
	})
	return nil
}
func (s *fakeCopilotStore) ListMessages(_ context.Context, threadID string) ([]CopilotMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]CopilotMessage, len(s.msgs[threadID]))
	copy(out, s.msgs[threadID])
	return out, nil
}
func (s *fakeCopilotStore) messages(threadID string) []CopilotMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]CopilotMessage, len(s.msgs[threadID]))
	copy(out, s.msgs[threadID])
	return out
}
func (s *fakeCopilotStore) sdkSession(threadID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sdkSessions[threadID]
}

// TestCopilotStreamsTurn drives a full turn from a scripted sidecar stream: session id →
// a tool call (whose result carries a citation) → a streamed token → the final message →
// done. It asserts the session id was persisted, token/message/done streamed over the hub,
// and the answer + citation were persisted.
func TestCopilotStreamsTurn(t *testing.T) {
	const userID = "11111111-1111-1111-1111-111111111111"

	store := newFakeStore()
	realtime := NewInMemoryRealtimeEventService(NewSubscriptionKeyResolver())
	agent := &fakeAgent{events: []copilotagent.Event{
		{Type: "session", SessionID: "sdk-1"},
		{Type: "tool_start", Name: "search"},
		{Type: "tool_result", Name: "search", Content: `{"citations":[{"kind":"entity","id":"e1","title":"NVIDIA"}]}`},
		{Type: "token", Delta: "NVDA looks strong."},
		{Type: "message", Text: "NVDA looks strong."},
		{Type: "done", SessionID: "sdk-1", NumTurns: 2, CostUSD: 0.05,
			Usage: copilotagent.TurnUsage{InputTokens: 10, OutputTokens: 5}},
	}}

	svc := NewCopilotService(CopilotDeps{Store: store, Agent: agent, Realtime: realtime})

	keys := []SubscriptionKey{{
		TenantID: userID, Stream: WorkspaceStream, ResourceKind: copilotResourceKind, ResourceID: AnyResource,
	}}
	events, unsubscribe, err := realtime.Subscribe(context.Background(), keys, "")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer unsubscribe()

	res, err := svc.SendMessage(context.Background(), CopilotSendInput{
		Principal: Principal{UserID: userID},
		Bearer:    "tok",
		Text:      "how does NVDA look?",
		Surface:   CopilotSurface{Kind: "ticker", Symbol: "NVDA"},
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if res.ThreadID == "" || res.SessionID == "" {
		t.Fatalf("expected thread + session ids, got %+v", res)
	}

	var gotToken, gotMessage, gotDone bool
	var messagePayload copilotMessagePayload
	deadline := time.After(3 * time.Second)
loop:
	for {
		select {
		case ev := <-events:
			switch ev.Type {
			case copilotResourceKind + ".token":
				gotToken = true
			case copilotResourceKind + ".message":
				gotMessage = true
				if p, ok := ev.Payload.(copilotMessagePayload); ok {
					messagePayload = p
				}
			case copilotResourceKind + ".done":
				gotDone = true
				break loop
			}
		case <-deadline:
			t.Fatal("timed out waiting for copilot.done")
		}
	}

	if !gotToken || !gotMessage || !gotDone {
		t.Fatalf("missing events: token=%v message=%v done=%v", gotToken, gotMessage, gotDone)
	}
	if messagePayload.Content != "NVDA looks strong." {
		t.Errorf("message content = %q", messagePayload.Content)
	}
	if len(messagePayload.Citations) != 1 || messagePayload.Citations[0].ID != "e1" {
		t.Errorf("expected one citation to e1, got %+v", messagePayload.Citations)
	}

	// The turn request carried the forwarded bearer + the gated-tool list.
	req := agent.request()
	if req.UserBearer != "tok" {
		t.Errorf("turn request bearer = %q, want tok", req.UserBearer)
	}
	if req.Prompt != "how does NVDA look?" {
		t.Errorf("turn request prompt = %q", req.Prompt)
	}
	if len(req.GatedTools) == 0 {
		t.Error("expected gated tools to be passed to the sidecar")
	}

	// The SDK session id was persisted for the next turn to resume.
	if store.sdkSession(res.ThreadID) != "claude:sdk-1" {
		t.Errorf("sdk session = %q, want claude:sdk-1", store.sdkSession(res.ThreadID))
	}

	// Persisted: user turn + final assistant turn.
	msgs := store.messages(res.ThreadID)
	if len(msgs) != 2 || msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Fatalf("unexpected stored messages: %+v", msgs)
	}
	if msgs[1].Content != "NVDA looks strong." {
		t.Errorf("stored assistant content = %q", msgs[1].Content)
	}
	// Turn meta (cost/usage + tool activity) is captured on the persisted message.
	if msgs[1].Meta == nil || msgs[1].Meta.CostUSD != 0.05 || msgs[1].Meta.NumTurns != 2 {
		t.Fatalf("assistant meta = %+v, want cost 0.05 / 2 turns", msgs[1].Meta)
	}
	if len(msgs[1].Meta.Activity) != 1 || msgs[1].Meta.Activity[0].Name != "search" || !msgs[1].Meta.Activity[0].OK {
		t.Fatalf("activity digest = %+v, want one ok search", msgs[1].Meta.Activity)
	}
}

func TestCopilotSendPassesSelectedModel(t *testing.T) {
	const userID = "11111111-1111-1111-1111-111111111111"
	agent := &fakeAgent{events: []copilotagent.Event{{Type: "done"}}, started: make(chan struct{})}
	svc := NewCopilotService(CopilotDeps{Store: newFakeStore(), Agent: agent})

	_, err := svc.SendMessage(context.Background(), CopilotSendInput{
		Principal: Principal{UserID: userID},
		Bearer:    "tok",
		Text:      "use sonnet",
		Model:     "claude:claude-sonnet-5",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	select {
	case <-agent.started:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for copilot turn to start")
	}
	if got := agent.request().Model; got != "claude:claude-sonnet-5" {
		t.Fatalf("turn request model = %q, want claude:claude-sonnet-5", got)
	}
	if got := agent.request().Provider; got != "claude" {
		t.Fatalf("turn request provider = %q, want claude", got)
	}
}

func TestCopilotSendPassesOpenAIProviderModel(t *testing.T) {
	const userID = "11111111-1111-1111-1111-111111111111"
	agent := &fakeAgent{events: []copilotagent.Event{{Type: "done"}}, started: make(chan struct{})}
	svc := NewCopilotService(CopilotDeps{Store: newFakeStore(), Agent: agent})

	_, err := svc.SendMessage(context.Background(), CopilotSendInput{
		Principal: Principal{UserID: userID},
		Bearer:    "tok",
		Text:      "use openai",
		Model:     "openai:gpt-5.1",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	select {
	case <-agent.started:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for copilot turn to start")
	}
	if got := agent.request().Provider; got != "openai" {
		t.Fatalf("turn request provider = %q, want openai", got)
	}
	if got := agent.request().Model; got != "openai:gpt-5.1" {
		t.Fatalf("turn request model = %q, want openai:gpt-5.1", got)
	}
}

func TestCopilotSendDoesNotResumeDifferentProviderSession(t *testing.T) {
	const userID = "11111111-1111-1111-1111-111111111111"
	store := newFakeStore()
	threadID := "thread-1"
	if err := store.CreateThread(context.Background(), threadID, userID, "thread"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetThreadSDKSession(context.Background(), threadID, "claude:sdk-old"); err != nil {
		t.Fatal(err)
	}
	agent := &fakeAgent{events: []copilotagent.Event{{Type: "done"}}, started: make(chan struct{})}
	svc := NewCopilotService(CopilotDeps{Store: store, Agent: agent})

	_, err := svc.SendMessage(context.Background(), CopilotSendInput{
		Principal: Principal{UserID: userID},
		Bearer:    "tok",
		ThreadID:  threadID,
		Text:      "switch providers",
		Model:     "openai:gpt-5.1",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	select {
	case <-agent.started:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for copilot turn to start")
	}
	if got := agent.request().ResumeSessionID; got != "" {
		t.Fatalf("resume session = %q, want empty for provider switch", got)
	}
}

func TestCopilotSendPassesSelectedEffort(t *testing.T) {
	const userID = "11111111-1111-1111-1111-111111111111"
	agent := &fakeAgent{events: []copilotagent.Event{{Type: "done"}}, started: make(chan struct{})}
	svc := NewCopilotService(CopilotDeps{Store: newFakeStore(), Agent: agent})

	_, err := svc.SendMessage(context.Background(), CopilotSendInput{
		Principal: Principal{UserID: userID},
		Bearer:    "tok",
		Text:      "use max effort",
		Effort:    "max",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	select {
	case <-agent.started:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for copilot turn to start")
	}
	if got := agent.request().Effort; got != "max" {
		t.Fatalf("turn request effort = %q, want max", got)
	}
}

func TestCopilotSendNormalizesLegacyModelID(t *testing.T) {
	const userID = "11111111-1111-1111-1111-111111111111"
	agent := &fakeAgent{events: []copilotagent.Event{{Type: "done"}}, started: make(chan struct{})}
	svc := NewCopilotService(CopilotDeps{Store: newFakeStore(), Agent: agent})

	_, err := svc.SendMessage(context.Background(), CopilotSendInput{
		Principal: Principal{UserID: userID},
		Bearer:    "tok",
		Text:      "legacy model",
		Model:     "opus5",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	select {
	case <-agent.started:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for copilot turn to start")
	}
	if got := agent.request().Model; got != "claude:claude-opus-5" {
		t.Fatalf("turn request model = %q, want claude:claude-opus-5", got)
	}
}

func TestCopilotSendRejectsUnsupportedModel(t *testing.T) {
	const userID = "11111111-1111-1111-1111-111111111111"
	svc := NewCopilotService(CopilotDeps{Store: newFakeStore(), Agent: &fakeAgent{}})

	_, err := svc.SendMessage(context.Background(), CopilotSendInput{
		Principal: Principal{UserID: userID},
		Bearer:    "tok",
		Text:      "bad model",
		Model:     "claude-typo",
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported copilot model") {
		t.Fatalf("err = %v, want unsupported model", err)
	}
}

func TestCopilotSendRejectsUnsupportedEffort(t *testing.T) {
	const userID = "11111111-1111-1111-1111-111111111111"
	svc := NewCopilotService(CopilotDeps{Store: newFakeStore(), Agent: &fakeAgent{}})

	_, err := svc.SendMessage(context.Background(), CopilotSendInput{
		Principal: Principal{UserID: userID},
		Bearer:    "tok",
		Text:      "bad effort",
		Effort:    "heroic",
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported copilot effort") {
		t.Fatalf("err = %v, want unsupported effort", err)
	}
}

func TestCopilotStatusReportsModelCatalog(t *testing.T) {
	svc := NewCopilotService(CopilotDeps{Store: newFakeStore(), Agent: &fakeAgent{}, Model: "claude-sonnet-5"})

	status := svc.Status(context.Background())

	if status.DefaultModel != "claude:claude-sonnet-5" {
		t.Fatalf("default model = %q, want claude:claude-sonnet-5", status.DefaultModel)
	}
	if status.DefaultEffort != "high" {
		t.Fatalf("default effort = %q, want high", status.DefaultEffort)
	}
	if len(status.Models) < 2 {
		t.Fatalf("expected model options, got %+v", status.Models)
	}
	if len(status.Efforts) != 5 {
		t.Fatalf("expected effort options, got %+v", status.Efforts)
	}
	if status.Models[0].ID != "claude:claude-opus-5" || status.Models[0].Label != "Claude Opus 5" {
		t.Fatalf("model catalog should expose API ids with friendly labels, got %+v", status.Models[0])
	}
}

// TestCopilotSendRequiresBearer — the sidecar's MCP calls are scoped by the forwarded
// bearer, so a send without one is rejected before any turn starts.
func TestCopilotSendRequiresBearer(t *testing.T) {
	svc := NewCopilotService(CopilotDeps{
		Store: newFakeStore(), Agent: &fakeAgent{}, Realtime: NewInMemoryRealtimeEventService(NewSubscriptionKeyResolver()),
	})
	_, err := svc.SendMessage(context.Background(), CopilotSendInput{Principal: Principal{UserID: "u1"}, Text: "hi"})
	if err == nil {
		t.Fatal("expected an error when no bearer is forwarded")
	}
}

func TestCopilotThreadManagementRenamesAndArchives(t *testing.T) {
	store := newFakeStore()
	const userID = "u1"
	const threadID = "t1"
	if err := store.CreateThread(context.Background(), threadID, userID, "Old title"); err != nil {
		t.Fatalf("seed thread: %v", err)
	}
	svc := NewCopilotService(CopilotDeps{
		Store: store, Agent: &fakeAgent{}, Realtime: NewInMemoryRealtimeEventService(NewSubscriptionKeyResolver()),
	})

	renamed, err := svc.RenameThread(context.Background(), userID, threadID, "  New title  ")
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if renamed.Title != "New title" {
		t.Fatalf("renamed title = %q, want trimmed title", renamed.Title)
	}
	pinned, err := svc.SetThreadPinned(context.Background(), userID, threadID, true)
	if err != nil {
		t.Fatalf("pin: %v", err)
	}
	if !pinned.Pinned {
		t.Fatal("thread should be pinned")
	}
	unpinned, err := svc.SetThreadPinned(context.Background(), userID, threadID, false)
	if err != nil {
		t.Fatalf("unpin: %v", err)
	}
	if unpinned.Pinned {
		t.Fatal("thread should be unpinned")
	}
	if _, err := svc.RenameThread(context.Background(), userID, threadID, "  "); err == nil {
		t.Fatal("blank rename should fail")
	}
	if _, err := svc.RenameThread(context.Background(), "other", threadID, "Nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("non-owner rename err = %v, want ErrNotFound", err)
	}

	if err := svc.ArchiveThread(context.Background(), userID, threadID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if _, err := svc.RenameThread(context.Background(), userID, threadID, "After archive"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("archived thread rename err = %v, want ErrNotFound", err)
	}
	if _, err := svc.SetThreadPinned(context.Background(), userID, threadID, true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("archived thread pin err = %v, want ErrNotFound", err)
	}
}

func TestCopilotToolSummariesAreBoundedAndRedacted(t *testing.T) {
	input := json.RawMessage(`{"query":"nvda","bearer":"secret-token","nested":{"password":"pw"}}`)
	got := toolInputSummary("search", input)
	if got != "query: nvda" {
		t.Fatalf("primary summary = %q, want query only", got)
	}
	generic := toolInputSummary("unknown_tool", json.RawMessage(`{"token":"secret-token","path":"src/index.tsx"}`))
	if strings.Contains(generic, "secret-token") {
		t.Fatalf("summary leaked a token: %q", generic)
	}
	if !strings.Contains(generic, "[redacted]") || !strings.Contains(generic, "src/index.tsx") {
		t.Fatalf("generic summary did not preserve safe context/redaction: %q", generic)
	}
	result := toolResultSummary(strings.Repeat("x", maxToolSummaryChars+20), false)
	if len(result) <= maxToolSummaryChars || !strings.HasSuffix(result, "…") {
		t.Fatalf("result summary was not capped: len=%d %q", len(result), result)
	}
}

func TestCopilotShardGuidanceUsesCapabilityDiscovery(t *testing.T) {
	prompt := (&defaultCopilotService{}).systemPrompt(CopilotSurface{})
	if !strings.Contains(prompt, "get_authoring_guide") || !strings.Contains(prompt, "page_id") {
		t.Fatal("Copilot must discover both new and existing shard capabilities")
	}
	for _, staleAPI := range []string{"useKV", "useShardState", "useNode", "useResource", "bridge/1", "bridge/2"} {
		if strings.Contains(prompt, staleAPI) {
			t.Fatalf("static prompt duplicates capability-specific guidance: %s", staleAPI)
		}
	}
}

func TestCopilotSystemPromptAdvertisesRichDirectives(t *testing.T) {
	prompt := (&defaultCopilotService{}).systemPrompt(CopilotSurface{})
	for _, want := range []string{
		":aladin-ticker[NVDA]",
		":aladin-artifact[Research note]",
		":aladin-entity[NVIDIA]",
		"Inline references use ONE colon",
		"Block references use TWO colons",
		"rich directives are block-only",
		"Do not wrap a directive in a markdown link",
		"::aladin-artifact",
		"::aladin-activity",
		"::aladin-actions",
		"::aladin-approval",
		"::aladin-diff",
		"::aladin-shard-preview",
		"::aladin-error-recovery",
		"never put HTML, JavaScript, CSS, external URLs, or secrets",
		"Rich directive trigger rules",
		"After create_app, read_file, write_file, edit_file, build_app",
		"When a gated action is pending, approved, rejected, expired, or failed",
		"include aladin-error-recovery with the exact error text",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("system prompt missing %q", want)
		}
	}
}

// TestCopilotCancelStops — a running turn cancels cleanly (a done event, no error), the
// sidecar is told to abort, and a non-owner cannot cancel it.
func TestCopilotCancelStops(t *testing.T) {
	const userID = "22222222-2222-2222-2222-222222222222"
	store := newFakeStore()
	realtime := NewInMemoryRealtimeEventService(NewSubscriptionKeyResolver())
	agent := &fakeAgent{block: true, started: make(chan struct{})}
	svc := NewCopilotService(CopilotDeps{Store: store, Agent: agent, Realtime: realtime})

	keys := []SubscriptionKey{{TenantID: userID, Stream: WorkspaceStream, ResourceKind: copilotResourceKind, ResourceID: AnyResource}}
	events, unsubscribe, err := realtime.Subscribe(context.Background(), keys, "")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer unsubscribe()

	res, err := svc.SendMessage(context.Background(), CopilotSendInput{Principal: Principal{UserID: userID}, Bearer: "tok", Text: "hi"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	select {
	case <-agent.started:
	case <-time.After(2 * time.Second):
		t.Fatal("agent turn never started")
	}

	if err := svc.Cancel(context.Background(), "someone-else", res.SessionID); err == nil {
		t.Error("expected a non-owner cancel to be rejected")
	}
	if err := svc.Cancel(context.Background(), userID, res.SessionID); err != nil {
		t.Fatalf("owner cancel: %v", err)
	}

	gotDone, gotError := false, false
	deadline := time.After(2 * time.Second)
	for !gotDone {
		select {
		case ev := <-events:
			switch ev.Type {
			case copilotResourceKind + ".done":
				gotDone = true
			case copilotResourceKind + ".error":
				gotError = true
			}
		case <-deadline:
			t.Fatal("timed out waiting for copilot.done after cancel")
		}
	}
	if gotError {
		t.Error("a cancelled turn should not emit an error event")
	}
}

// TestCopilotProposesGatedAction — a gated tool proposal registers a pending action and
// emits proposed_action; a non-owner cannot approve; reject clears the registry and relays
// the rejection to the sidecar.
func TestCopilotProposesGatedAction(t *testing.T) {
	const userID = "33333333-3333-3333-3333-333333333333"
	store := newFakeStore()
	realtime := NewInMemoryRealtimeEventService(NewSubscriptionKeyResolver())
	agent := &fakeAgent{events: []copilotagent.Event{
		{Type: "session", SessionID: "sdk-2"},
		{Type: "proposed_action", ApprovalID: "a1", Tool: "delete_file", Input: []byte(`{"path":"old.tsx"}`)},
		{Type: "message", Text: "I proposed deleting it."},
		{Type: "done", SessionID: "sdk-2"},
	}}
	svc := NewCopilotService(CopilotDeps{Store: store, Agent: agent, Realtime: realtime})

	keys := []SubscriptionKey{{TenantID: userID, Stream: WorkspaceStream, ResourceKind: copilotResourceKind, ResourceID: AnyResource}}
	events, unsubscribe, err := realtime.Subscribe(context.Background(), keys, "")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer unsubscribe()

	if _, err := svc.SendMessage(context.Background(), CopilotSendInput{Principal: Principal{UserID: userID}, Bearer: "tok", Text: "delete old.tsx"}); err != nil {
		t.Fatalf("send: %v", err)
	}

	var actionID, summary string
	deadline := time.After(3 * time.Second)
drain:
	for {
		select {
		case ev := <-events:
			if ev.Type == copilotResourceKind+".proposed_action" {
				if p, ok := ev.Payload.(copilotProposedPayload); ok {
					actionID, summary = p.ActionID, p.Summary
				}
			}
			if ev.Type == copilotResourceKind+".done" {
				break drain
			}
		case <-deadline:
			t.Fatal("timed out waiting for done")
		}
	}
	if actionID != "a1" {
		t.Fatalf("expected proposed_action a1, got %q", actionID)
	}
	if !strings.Contains(summary, "old.tsx") {
		t.Errorf("summary should name the file, got %q", summary)
	}

	impl := svc.(*defaultCopilotService)
	n := impl.tools.PendingCount()
	if n != 1 {
		t.Fatalf("expected 1 pending action, got %d", n)
	}

	if err := svc.ApproveAction(context.Background(), "someone-else", actionID); err == nil {
		t.Error("a non-owner approve should be rejected")
	}
	if err := svc.RejectAction(context.Background(), userID, actionID); err != nil {
		t.Fatalf("reject: %v", err)
	}
	n = impl.tools.PendingCount()
	if n != 0 {
		t.Fatalf("expected pending cleared after reject, got %d", n)
	}
	// The rejection was relayed to the sidecar to release the held tool.
	calls := agent.resolveCalls()
	if len(calls) != 1 || calls[0].approvalID != "a1" || calls[0].approved {
		t.Fatalf("expected one reject relayed to the sidecar, got %+v", calls)
	}
}

// TestCopilotInterruptedStreamFails — a stream that ends without its terminal `done`
// (sidecar died mid-turn) surfaces as an error, not a silent success.
func TestCopilotInterruptedStreamFails(t *testing.T) {
	const userID = "44444444-4444-4444-4444-444444444444"
	store := newFakeStore()
	realtime := NewInMemoryRealtimeEventService(NewSubscriptionKeyResolver())
	agent := &fakeAgent{events: []copilotagent.Event{
		{Type: "session", SessionID: "sdk-3"},
		{Type: "token", Delta: "partial"},
		// no done — the stream just ends
	}}
	svc := NewCopilotService(CopilotDeps{Store: store, Agent: agent, Realtime: realtime})

	keys := []SubscriptionKey{{TenantID: userID, Stream: WorkspaceStream, ResourceKind: copilotResourceKind, ResourceID: AnyResource}}
	events, unsubscribe, err := realtime.Subscribe(context.Background(), keys, "")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer unsubscribe()

	if _, err := svc.SendMessage(context.Background(), CopilotSendInput{Principal: Principal{UserID: userID}, Bearer: "tok", Text: "hi"}); err != nil {
		t.Fatalf("send: %v", err)
	}

	gotError, gotDone := false, false
	deadline := time.After(3 * time.Second)
	for !gotDone {
		select {
		case ev := <-events:
			switch ev.Type {
			case copilotResourceKind + ".error":
				gotError = true
			case copilotResourceKind + ".done":
				gotDone = true
			}
		case <-deadline:
			t.Fatal("timed out waiting for done after an interrupted stream")
		}
	}
	if !gotError {
		t.Error("an interrupted stream (no terminal done) should emit an error event")
	}
}

// TestCopilotRejectsConcurrentTurnOnThread — one turn per thread: a second send while a
// turn runs on the same thread gets ErrConflict; a different thread is unaffected; after
// cancel the thread accepts again.
func TestCopilotRejectsConcurrentTurnOnThread(t *testing.T) {
	const userID = "55555555-5555-5555-5555-555555555555"
	store := newFakeStore()
	realtime := NewInMemoryRealtimeEventService(NewSubscriptionKeyResolver())
	agent := &fakeAgent{block: true, started: make(chan struct{})}
	svc := NewCopilotService(CopilotDeps{Store: store, Agent: agent, Realtime: realtime})

	res, err := svc.SendMessage(context.Background(), CopilotSendInput{Principal: Principal{UserID: userID}, Bearer: "tok", Text: "first"})
	if err != nil {
		t.Fatalf("first send: %v", err)
	}
	select {
	case <-agent.started:
	case <-time.After(2 * time.Second):
		t.Fatal("turn never started")
	}

	// Same thread while running → conflict, and the rejected message is NOT persisted.
	if _, err := svc.SendMessage(context.Background(), CopilotSendInput{
		Principal: Principal{UserID: userID}, Bearer: "tok", Text: "second", ThreadID: res.ThreadID,
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("concurrent same-thread send error = %v, want ErrConflict", err)
	}
	if got := len(store.messages(res.ThreadID)); got != 1 {
		t.Fatalf("rejected send persisted a message: %d stored, want 1", got)
	}

	// A different thread is unaffected.
	if _, err := svc.SendMessage(context.Background(), CopilotSendInput{
		Principal: Principal{UserID: userID}, Bearer: "tok", Text: "elsewhere",
	}); err != nil {
		t.Fatalf("other-thread send: %v", err)
	}

	// After cancel, the original thread accepts again (poll: deregistration is async).
	if err := svc.Cancel(context.Background(), userID, res.SessionID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		_, err = svc.SendMessage(context.Background(), CopilotSendInput{
			Principal: Principal{UserID: userID}, Bearer: "tok", Text: "again", ThreadID: res.ThreadID,
		})
		if err == nil || !errors.Is(err, ErrConflict) || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("send after cancel: %v", err)
	}
}

// TestCopilotNotConfigured — no agent ⇒ Configured()=false and SendMessage rejects cleanly.
func TestCopilotNotConfigured(t *testing.T) {
	svc := NewCopilotService(CopilotDeps{Store: newFakeStore(), Realtime: NewInMemoryRealtimeEventService(NewSubscriptionKeyResolver())})
	if svc.Configured() {
		t.Fatal("expected Configured()=false with no agent")
	}
	_, err := svc.SendMessage(context.Background(), CopilotSendInput{
		Principal: Principal{UserID: "u1"}, Bearer: "tok", Text: "hi",
	})
	if err == nil {
		t.Fatal("expected an error when copilot is not configured")
	}
}

// stubArtifacts satisfies ArtifactService by embedding it (every other method is nil and
// would panic if called — this test only needs Get).
type stubArtifacts struct {
	ArtifactService
	art ArtifactResponse
}

func (s stubArtifacts) Get(context.Context, string) (ArtifactResponse, error) { return s.art, nil }

// A `file` artifact is a SOURCE, not an editable page. Before this branch existed a file fell
// through to the page case and the agent was told "EDIT THIS page — get_page then
// insert_blocks", i.e. to rewrite the user's own PDF as a BlockNote document. Every such call
// can only fail, and it is the opposite of what a source is for.
func TestCopilotSurfaceContextFileIsNotEditableAsAPage(t *testing.T) {
	svc := &defaultCopilotService{
		CopilotDeps: CopilotDeps{Artifacts: stubArtifacts{art: ArtifactResponse{
			Type: "file", Title: "Hamilton ch. 19",
		}}},
	}
	got := svc.surfaceContext(context.Background(), "u1", CopilotSurface{Kind: "artifact", ID: "artifact-1"})

	// Assert on the INSTRUCTION, not on tool names: the fixed message deliberately names
	// get_page/insert_blocks inside a prohibition, which is more useful to the model than a
	// vague "don't edit it". What must never appear is the affirmative page-editing framing.
	for _, banned := range []string{"To change it, EDIT THIS", "Do NOT create a new page"} {
		if strings.Contains(got, banned) {
			t.Fatalf("a file artifact must not be described as editable; found %q in %q", banned, got)
		}
	}
	if !strings.Contains(got, "Never call") {
		t.Fatalf("expected an explicit prohibition on the page-edit tools, got %q", got)
	}
	for _, want := range []string{"read_document", "search_document", "cite page numbers"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected document-retrieval guidance %q, got %q", want, got)
		}
	}
}

// The shard and page branches must keep their edit instructions — the fix above is a new
// branch, not a change to the existing ones.
func TestCopilotSurfaceContextShardAndPageStillEditable(t *testing.T) {
	shard := &defaultCopilotService{
		CopilotDeps: CopilotDeps{Artifacts: stubArtifacts{art: ArtifactResponse{Type: "app", Title: "Collar payoff"}}},
	}
	if got := shard.surfaceContext(context.Background(), "u1", CopilotSurface{Kind: "artifact", ID: "a1"}); !strings.Contains(got, "EDIT THIS shard") {
		t.Fatalf("shard should still be editable as a shard, got %q", got)
	}

	page := &defaultCopilotService{
		CopilotDeps: CopilotDeps{Artifacts: stubArtifacts{art: ArtifactResponse{Type: "page", Title: "Notes"}}},
	}
	if got := page.surfaceContext(context.Background(), "u1", CopilotSurface{Kind: "artifact", ID: "a2"}); !strings.Contains(got, "EDIT THIS page") {
		t.Fatalf("page should still be editable as a page, got %q", got)
	}
}
