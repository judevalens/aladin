import { describe, expect, it } from "vitest";
import { create } from "zustand";
import { createCopilotSlice, type CopilotSlice } from "@/app/state/copilot-slice";
import { turnLooksStuck } from "@/modules/copilot/hooks/use-copilot";

function makeStore() {
  return create<CopilotSlice>()((...args) => createCopilotSlice(...args));
}

function startTurn(store: ReturnType<typeof makeStore>, threadId = "t1", sessionId = "s1") {
  store.getState().beginCopilotTurn(threadId, sessionId);
}

const proposal = (over: Partial<Parameters<CopilotSlice["addCopilotProposal"]>[0]> = {}) => ({
  actionId: "a1",
  threadId: "t1",
  sessionId: "s1",
  tool: "publish_app",
  summary: "Publish the shard",
  ...over,
});

describe("copilot proposal lifecycle", () => {
  it("adds proposals only for the in-flight session (second-window gate)", () => {
    const store = makeStore();
    startTurn(store);
    store.getState().addCopilotProposal(proposal());
    // A proposal from some other session (another window's turn) is ignored.
    store.getState().addCopilotProposal(proposal({ actionId: "a2", sessionId: "other" }));
    expect(store.getState().copilotProposals.map((p) => p.actionId)).toEqual(["a1"]);
  });

  it("expires unresolved proposals when the turn ends", () => {
    const store = makeStore();
    startTurn(store);
    store.getState().addCopilotProposal(proposal());
    store.getState().endCopilotTurn("t1", "s1");
    expect(store.getState().copilotProposals[0]?.status).toBe("expired");
  });

  it("expires unresolved proposals on error and on stop", () => {
    const store = makeStore();
    startTurn(store);
    store.getState().addCopilotProposal(proposal());
    store.getState().setCopilotError("t1", "s1", "boom");
    expect(store.getState().copilotProposals[0]?.status).toBe("expired");

    const store2 = makeStore();
    startTurn(store2);
    store2.getState().addCopilotProposal(proposal());
    store2.getState().stopCopilotTurn();
    expect(store2.getState().copilotProposals[0]?.status).toBe("expired");
    expect(store2.getState().copilotProposals[0]?.message).toBe("Cancelled.");
  });

  it("keeps settled proposals settled when the turn ends", () => {
    const store = makeStore();
    startTurn(store);
    store.getState().addCopilotProposal(proposal());
    store.getState().resolveCopilotProposal("a1", true, "Done.");
    store.getState().endCopilotTurn("t1", "s1");
    expect(store.getState().copilotProposals[0]?.status).toBe("approved");
  });

  it("survives a thread switch and resolves after approving", () => {
    const store = makeStore();
    startTurn(store);
    store.getState().addCopilotProposal(proposal());
    store.getState().markCopilotProposal("a1", "approving");
    // Late action_result still resolves an approving proposal.
    store.getState().resolveCopilotProposal("a1", true, "Done.");
    expect(store.getState().copilotProposals[0]?.status).toBe("approved");
  });

  it("reverts approving back to pending with a note on POST failure", () => {
    const store = makeStore();
    startTurn(store);
    store.getState().addCopilotProposal(proposal());
    store.getState().markCopilotProposal("a1", "approving");
    store.getState().markCopilotProposal("a1", "pending", "Couldn't reach the server — try again.");
    const p = store.getState().copilotProposals[0];
    expect(p?.status).toBe("pending");
    expect(p?.message).toContain("try again");
  });

  it("keeps unresolved proposals actionable when a thread is reopened", () => {
    const store = makeStore();
    startTurn(store);
    store.getState().addCopilotProposal(proposal());
    // Simulate switching away (session gate lost) then reopening the thread: the
    // server-side approval hold can still be alive, so the dock must not kill the card.
    store.getState().openCopilotThread("t2", []);
    store.getState().openCopilotThread("t1", []);
    expect(store.getState().copilotProposals[0]?.status).toBe("pending");
  });

  it("binds early stream events that arrive before the send response resolves", () => {
    const store = makeStore();
    store.getState().appendCopilotUserMessage("hello");
    store.getState().appendCopilotToken("t-new", "s-new", "early ");
    store.getState().setCopilotTool("t-new", "s-new", "search", "Searching your workspace");
    const s = store.getState();
    expect(s.activeThreadId).toBe("t-new");
    expect(s.copilotSessionId).toBe("s-new");
    expect(s.copilotStatus).toBe("streaming");
    expect(s.copilotStreaming).toBe("early ");
    expect(s.copilotToolTrail[0]?.label).toBe("Searching your workspace");
  });
});

describe("copilot reconcile (watchdog / reconnect recovery)", () => {
  it("ends the turn when the fetched tail holds an unseen assistant answer", () => {
    const store = makeStore();
    startTurn(store);
    store.getState().addCopilotProposal(proposal());
    store.getState().appendCopilotToken("t1", "s1", "partial…");
    store.getState().reconcileCopilotThread("t1", [
      { id: "u1", role: "user", content: "hi", citations: [] },
      { id: "srv-9", role: "assistant", content: "final answer", citations: [] },
    ]);
    const s = store.getState();
    expect(s.copilotStatus).toBe("idle");
    expect(s.copilotSessionId).toBeNull();
    expect(s.copilotStreaming).toBe("");
    expect(s.copilotMessages.map((m) => m.id)).toEqual(["u1", "srv-9"]);
    expect(s.copilotProposals[0]?.status).toBe("expired");
  });

  it("leaves a genuinely-running turn alone", () => {
    const store = makeStore();
    startTurn(store);
    store.getState().appendCopilotToken("t1", "s1", "partial…");
    store.getState().reconcileCopilotThread("t1", [
      { id: "u1", role: "user", content: "hi", citations: [] },
    ]);
    const s = store.getState();
    expect(s.copilotStatus).toBe("streaming");
    expect(s.copilotStreaming).toBe("partial…");
  });

  it("ignores a reconcile for a thread that is no longer active", () => {
    const store = makeStore();
    startTurn(store, "t1", "s1");
    store.getState().reconcileCopilotThread("t-other", [
      { id: "x", role: "assistant", content: "elsewhere", citations: [] },
    ]);
    expect(store.getState().copilotStatus).toBe("streaming");
  });

  it("bumps the reconnect nonce", () => {
    const store = makeStore();
    store.getState().noteCopilotWsReconnect();
    store.getState().noteCopilotWsReconnect();
    expect(store.getState().copilotWsReconnects).toBe(2);
  });
});

describe("turnLooksStuck", () => {
  const base = { status: "streaming", lastEventAt: 0, now: 120_000, awaitingApproval: false };

  it("flags a long-silent streaming turn", () => {
    expect(turnLooksStuck(base)).toBe(true);
  });

  it("never flags while idle, during an approval hold, or with recent events", () => {
    expect(turnLooksStuck({ ...base, status: "idle" })).toBe(false);
    expect(turnLooksStuck({ ...base, awaitingApproval: true })).toBe(false);
    expect(turnLooksStuck({ ...base, lastEventAt: 100_000 })).toBe(false);
    expect(turnLooksStuck({ ...base, lastEventAt: null })).toBe(false);
  });
});

describe("queue-of-one", () => {
  it("takes the queued text exactly once", () => {
    const store = makeStore();
    store.getState().queueCopilotText("follow-up");
    expect(store.getState().takeCopilotQueuedText()).toBe("follow-up");
    expect(store.getState().takeCopilotQueuedText()).toBeNull();
  });

  it("clears the queue when the conversation changes", () => {
    const store = makeStore();
    store.getState().queueCopilotText("follow-up");
    store.getState().newCopilotThread();
    expect(store.getState().copilotQueuedText).toBeNull();
  });
});

describe("copilot drafts", () => {
  it("keeps unsent composer text per thread", () => {
    const store = makeStore();
    store.getState().openCopilotThread("t1", []);
    store.getState().setCopilotDraft("t1", "draft one");
    store.getState().openCopilotThread("t2", []);
    store.getState().setCopilotDraft("t2", "draft two");

    expect(store.getState().copilotDraftFor("t1")).toBe("draft one");
    expect(store.getState().copilotDraftFor("t2")).toBe("draft two");
  });

  it("starts an explicit new thread with a blank draft", () => {
    const store = makeStore();
    store.getState().setCopilotDraft(null, "new thread draft");
    store.getState().newCopilotThread();

    expect(store.getState().copilotDraftFor(null)).toBe("");
  });
});

describe("copilot thread management", () => {
  it("renames a thread in the local switcher list", () => {
    const store = makeStore();
    store.getState().setCopilotThreads([
      { id: "t1", title: "Old", updatedAt: "2026-01-01T00:00:00Z" },
      { id: "t2", title: "Other", updatedAt: "2026-01-02T00:00:00Z" },
    ]);

    store.getState().renameCopilotThreadLocal({
      id: "t1",
      title: "New",
      updatedAt: "2026-01-03T00:00:00Z",
    });

    expect(store.getState().copilotThreads.map((t) => t.title)).toEqual(["New", "Other"]);
  });

  it("archives the active thread locally and returns to a fresh chat", () => {
    const store = makeStore();
    store.getState().setCopilotThreads([
      { id: "t1", title: "Active", updatedAt: "2026-01-01T00:00:00Z" },
      { id: "t2", title: "Other", updatedAt: "2026-01-02T00:00:00Z" },
    ]);
    store.getState().openCopilotThread("t1", [
      { id: "m1", role: "user", content: "hello", citations: [] },
    ]);

    store.getState().archiveCopilotThreadLocal("t1");

    expect(store.getState().copilotThreads.map((t) => t.id)).toEqual(["t2"]);
    expect(store.getState().activeThreadId).toBeNull();
    expect(store.getState().copilotMessages).toEqual([]);
  });

  it("keeps pinned threads ahead of normal recency", () => {
    const store = makeStore();
    store.getState().setCopilotThreads([
      { id: "old-pinned", title: "Pinned", updatedAt: "2026-01-01T00:00:00Z", pinned: true },
      { id: "new-normal", title: "Recent", updatedAt: "2026-01-03T00:00:00Z" },
      { id: "old-normal", title: "Old", updatedAt: "2026-01-02T00:00:00Z" },
    ]);

    expect(store.getState().copilotThreads.map((t) => t.id)).toEqual([
      "old-pinned",
      "new-normal",
      "old-normal",
    ]);

    store.getState().updateCopilotThreadLocal({
      id: "new-normal",
      title: "Recent",
      updatedAt: "2026-01-03T00:00:00Z",
      pinned: true,
    });

    expect(store.getState().copilotThreads.map((t) => t.id)).toEqual([
      "new-normal",
      "old-pinned",
      "old-normal",
    ]);
  });
});

describe("turnDigest", () => {
  it("collapses consecutive tools and appends cost + steps", async () => {
    const { turnDigest } = await import("@/modules/copilot/ui/copilot-dock-ui");
    expect(
      turnDigest({
        numTurns: 23,
        costUsd: 0.14,
        activity: [
          { name: "search", ok: true },
          { name: "write_file", ok: true },
          { name: "write_file", ok: false },
          { name: "build_app", ok: true },
        ],
      }),
    ).toBe("searched workspace · wrote shard code ×2 ✗ · built shard — $0.14 · 23 steps");
    expect(turnDigest(undefined)).toBe("");
  });
});

describe("optimistic send bookkeeping", () => {
  it("drops a local message by id (failed send)", () => {
    const store = makeStore();
    const id = store.getState().appendCopilotUserMessage("hello");
    expect(store.getState().copilotMessages).toHaveLength(1);
    store.getState().dropCopilotLocalMessage(id);
    expect(store.getState().copilotMessages).toHaveLength(0);
  });
});
