package repo_test

import (
	"context"
	"os"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/repo"
	coreservice "aladin/backend_v2/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCopilotStoreRoundTrip(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("no TEST_DATABASE_URL")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	store := repo.NewCopilotPostgres(pool)
	userID := uuid.NewString()
	otherUser := uuid.NewString()
	threadID := uuid.NewString()
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM copilot_threads WHERE id = $1::uuid`, threadID) })

	if err := store.CreateThread(ctx, threadID, userID, "how does NVDA look?"); err != nil {
		t.Fatalf("create thread: %v", err)
	}

	if err := store.AppendMessage(ctx, coreservice.StoredCopilotMessage{
		ID: uuid.NewString(), ThreadID: threadID, Role: "user", Content: "how does NVDA look?",
	}); err != nil {
		t.Fatalf("append user: %v", err)
	}
	if err := store.AppendMessage(ctx, coreservice.StoredCopilotMessage{
		ID: uuid.NewString(), ThreadID: threadID, Role: "assistant", Content: "Strong.",
		Citations: []coreservice.Citation{{Kind: "ticker", ID: "NVDA", Title: "NVDA"}},
		Meta: &coreservice.CopilotMessageMeta{
			NumTurns: 3, InputTokens: 100, OutputTokens: 40, CostUSD: 0.12,
			Activity: []coreservice.CopilotActivityItem{{Name: "search", OK: true}},
		},
	}); err != nil {
		t.Fatalf("append assistant: %v", err)
	}

	msgs, err := store.ListMessages(ctx, threadID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Fatalf("unexpected roles: %q, %q", msgs[0].Role, msgs[1].Role)
	}
	if len(msgs[1].Citations) != 1 || msgs[1].Citations[0].ID != "NVDA" {
		t.Fatalf("citation did not round-trip: %+v", msgs[1].Citations)
	}
	if len(msgs[0].Citations) != 0 {
		t.Fatalf("user message should have no citations, got %+v", msgs[0].Citations)
	}
	// Turn meta round-trips on the assistant message; absent on the user turn.
	if msgs[0].Meta != nil {
		t.Fatalf("user message should have no meta, got %+v", msgs[0].Meta)
	}
	if msgs[1].Meta == nil || msgs[1].Meta.CostUSD != 0.12 || len(msgs[1].Meta.Activity) != 1 {
		t.Fatalf("assistant meta did not round-trip: %+v", msgs[1].Meta)
	}

	// Ownership is enforced on GetThread.
	if _, ok, err := store.GetThread(ctx, userID, threadID); err != nil || !ok {
		t.Fatalf("owner GetThread: ok=%v err=%v", ok, err)
	}

	// SDK session id round-trips (empty until stamped; re-stamped after each turn).
	th, _, err := store.GetThread(ctx, userID, threadID)
	if err != nil || th.SDKSessionID != "" {
		t.Fatalf("fresh thread sdk session = %q err=%v, want empty", th.SDKSessionID, err)
	}
	if err := store.SetThreadSDKSession(ctx, threadID, "sess-abc"); err != nil {
		t.Fatalf("set sdk session: %v", err)
	}
	th, _, err = store.GetThread(ctx, userID, threadID)
	if err != nil || th.SDKSessionID != "sess-abc" {
		t.Fatalf("sdk session = %q err=%v, want sess-abc", th.SDKSessionID, err)
	}
	if err := store.SetThreadSDKSession(ctx, threadID, ""); err != nil {
		t.Fatalf("clear sdk session: %v", err)
	}
	th, _, err = store.GetThread(ctx, userID, threadID)
	if err != nil || th.SDKSessionID != "" {
		t.Fatalf("cleared sdk session = %q err=%v, want empty", th.SDKSessionID, err)
	}
	if _, ok, err := store.GetThread(ctx, otherUser, threadID); err != nil || ok {
		t.Fatalf("non-owner GetThread should be not-found: ok=%v err=%v", ok, err)
	}
	pinned, ok, err := store.SetThreadPinned(ctx, userID, threadID, true)
	if err != nil || !ok {
		t.Fatalf("pin thread: ok=%v err=%v", ok, err)
	}
	if !pinned.Pinned {
		t.Fatalf("pinned thread = %+v, want pinned", pinned)
	}
	if _, ok, err := store.SetThreadPinned(ctx, otherUser, threadID, false); err != nil || ok {
		t.Fatalf("non-owner pin should be not-found: ok=%v err=%v", ok, err)
	}

	threads, err := store.ListThreads(ctx, userID)
	if err != nil {
		t.Fatalf("list threads: %v", err)
	}
	found := false
	for _, th := range threads {
		if th.ID == threadID && th.Title == "how does NVDA look?" && th.Pinned {
			found = true
		}
	}
	if !found {
		t.Fatalf("thread not in ListThreads: %+v", threads)
	}
	if ok, err := store.ArchiveThread(ctx, userID, threadID); err != nil || !ok {
		t.Fatalf("archive thread: ok=%v err=%v", ok, err)
	}
	threads, err = store.ListThreads(ctx, userID)
	if err != nil {
		t.Fatalf("list after archive: %v", err)
	}
	for _, th := range threads {
		if th.ID == threadID {
			t.Fatalf("archived thread should be hidden from ListThreads: %+v", threads)
		}
	}
}
