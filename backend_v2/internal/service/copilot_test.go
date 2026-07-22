package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"aladin/backend_v2/internal/llm"
)

// fakeChatAgent replays scripted turns: each turn may stream text via onEvent and returns
// an assistant message (with tool calls or final content).
type fakeChatAgent struct {
	turns []func(onEvent func(llm.ChatEvent)) llm.ChatMessage
	calls int
}

func (f *fakeChatAgent) Chat(_ context.Context, _ []llm.ChatMessage, _ []llm.ChatToolDef, onEvent func(llm.ChatEvent)) (llm.ChatMessage, error) {
	if f.calls >= len(f.turns) {
		return llm.ChatMessage{Role: llm.ChatRoleAssistant, Content: "done"}, nil
	}
	turn := f.turns[f.calls]
	f.calls++
	return turn(onEvent), nil
}

type fakeSearch struct {
	mu     sync.Mutex
	called bool
}

func (f *fakeSearch) Search(_ context.Context, _ string, _ string, _ int) (SearchResponse, error) {
	f.mu.Lock()
	f.called = true
	f.mu.Unlock()
	return SearchResponse{Sections: []SearchSection{{
		Type:  "entity",
		Label: "Entities",
		Hits:  []SearchHit{{Kind: "entity", ID: "e1", Title: "NVIDIA"}},
	}}}, nil
}

func (f *fakeSearch) wasCalled() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.called
}

type fakeCopilotStore struct {
	mu      sync.Mutex
	owners  map[string]string
	msgs    map[string][]CopilotMessage
	touches int
}

func newFakeStore() *fakeCopilotStore {
	return &fakeCopilotStore{owners: map[string]string{}, msgs: map[string][]CopilotMessage{}}
}

func (s *fakeCopilotStore) CreateThread(_ context.Context, id, userID, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.owners[id] = userID
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
	if s.owners[threadID] != userID {
		return CopilotThread{}, false, nil
	}
	return CopilotThread{ID: threadID}, true, nil
}
func (s *fakeCopilotStore) AppendMessage(_ context.Context, m StoredCopilotMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.msgs[m.ThreadID] = append(s.msgs[m.ThreadID], CopilotMessage{
		ID: m.ID, Role: m.Role, Content: m.Content, Citations: m.Citations,
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

// TestCopilotToolLoopStreams drives the full agent loop with a fake LLM: turn 1 calls the
// search tool, turn 2 answers. It asserts the tool ran, the answer + citations were
// persisted, and token/message/done events streamed over the realtime hub.
func TestCopilotToolLoopStreams(t *testing.T) {
	const userID = "11111111-1111-1111-1111-111111111111"

	search := &fakeSearch{}
	store := newFakeStore()
	realtime := NewInMemoryRealtimeEventService(NewSubscriptionKeyResolver())

	agent := &fakeChatAgent{turns: []func(func(llm.ChatEvent)) llm.ChatMessage{
		// Turn 1: request the search tool.
		func(_ func(llm.ChatEvent)) llm.ChatMessage {
			return llm.ChatMessage{Role: llm.ChatRoleAssistant, ToolCalls: []llm.ChatToolCall{
				{ID: "call_1", Name: "search", Arguments: `{"query":"nvda"}`},
			}}
		},
		// Turn 2: stream a token and finish.
		func(onEvent func(llm.ChatEvent)) llm.ChatMessage {
			onEvent(llm.ChatEvent{Kind: llm.ChatEventText, Text: "NVDA looks strong."})
			return llm.ChatMessage{Role: llm.ChatRoleAssistant, Content: "NVDA looks strong."}
		},
	}}

	svc := NewCopilotService(CopilotDeps{
		Store:    store,
		Agent:    agent,
		Realtime: realtime,
		Search:   search,
	})

	// Subscribe to this user's copilot events before sending.
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
		Text:      "how does NVDA look?",
		Surface:   CopilotSurface{Kind: "ticker", Symbol: "NVDA"},
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if res.ThreadID == "" || res.SessionID == "" {
		t.Fatalf("expected thread + session ids, got %+v", res)
	}

	// Drain events until done (or timeout).
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

	if !search.wasCalled() {
		t.Error("expected the search tool to run")
	}
	if !gotToken {
		t.Error("expected a copilot.token event")
	}
	if !gotMessage {
		t.Error("expected a copilot.message event")
	}
	if !gotDone {
		t.Error("expected a copilot.done event")
	}
	if messagePayload.Content != "NVDA looks strong." {
		t.Errorf("message content = %q, want %q", messagePayload.Content, "NVDA looks strong.")
	}
	if len(messagePayload.Citations) != 1 || messagePayload.Citations[0].ID != "e1" {
		t.Errorf("expected one citation to e1, got %+v", messagePayload.Citations)
	}

	// Persisted: user turn + final assistant turn.
	msgs := store.messages(res.ThreadID)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 stored messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Errorf("unexpected roles: %q, %q", msgs[0].Role, msgs[1].Role)
	}
	if msgs[1].Content != "NVDA looks strong." {
		t.Errorf("stored assistant content = %q", msgs[1].Content)
	}
}

// TestCopilotNotConfigured — no agent ⇒ Configured()=false and SendMessage rejects cleanly.
func TestCopilotNotConfigured(t *testing.T) {
	svc := NewCopilotService(CopilotDeps{Store: newFakeStore(), Realtime: NewInMemoryRealtimeEventService(NewSubscriptionKeyResolver())})
	if svc.Configured() {
		t.Fatal("expected Configured()=false with no agent")
	}
	_, err := svc.SendMessage(context.Background(), CopilotSendInput{
		Principal: Principal{UserID: "u1"}, Text: "hi",
	})
	if err == nil {
		t.Fatal("expected an error when copilot is not configured")
	}
}
