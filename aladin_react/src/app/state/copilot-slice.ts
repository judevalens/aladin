import type { StateCreator } from "zustand";

/** One grounding source the assistant used — rendered as a clickable chip. */
export interface CopilotCitation {
  kind: string; // ticker | company | person | entity | page | shard
  id: string;
  title: string;
}

/** Per-assistant-turn metadata: what the turn did and what it cost. */
export interface CopilotMessageMeta {
  numTurns?: number;
  inputTokens?: number;
  outputTokens?: number;
  costUsd?: number;
  activity?: { name: string; ok: boolean }[];
}

/** One visible turn in a copilot thread. */
export interface CopilotMessageView {
  id: string;
  role: "user" | "assistant";
  content: string;
  citations: CopilotCitation[];
  meta?: CopilotMessageMeta;
}

/** A thread as shown in the thread switcher. */
export interface CopilotThreadView {
  id: string;
  title: string;
  updatedAt: string;
}

export type CopilotStatus = "idle" | "sending" | "streaming";

/** One tool invocation in the live turn's activity trail. */
export interface CopilotToolRun {
  name: string;
  label: string;
  status: "running" | "ok" | "error";
}

const maxToolTrail = 40;

/**
 * A gated action the agent proposed. The backend HOLDS the tool call open until the
 * user approves/rejects (or the hold times out), so the proposal has a real lifecycle:
 *   pending → approving/rejecting (POST in flight) → approved/rejected (action_result)
 *   pending/approving/rejecting → expired (turn ended/errored/stopped — hold is gone)
 * Proposals are kept across thread switches (the hold survives server-side) and
 * rendered filtered by threadId.
 */
export interface CopilotProposal {
  actionId: string;
  threadId: string;
  sessionId: string;
  tool: string;
  summary: string;
  status: "pending" | "approving" | "rejecting" | "approved" | "rejected" | "expired";
  message?: string;
}

const localStorageKey = "aladin.copilot.activeThreadId";

function readPersistedThreadId(): string | null {
  try {
    return window.localStorage.getItem(localStorageKey);
  } catch {
    return null;
  }
}

function persistThreadId(threadId: string | null) {
  try {
    if (threadId) window.localStorage.setItem(localStorageKey, threadId);
    else window.localStorage.removeItem(localStorageKey);
  } catch {
    // storage unavailable — rehydrate is best-effort
  }
}

/** Expire any unresolved proposals belonging to a session whose turn just ended. */
function expireForSession(
  proposals: CopilotProposal[],
  sessionId: string | null,
  message: string,
): CopilotProposal[] {
  if (!sessionId) return proposals;
  return proposals.map((p) =>
    p.sessionId === sessionId &&
    (p.status === "pending" || p.status === "approving" || p.status === "rejecting")
      ? { ...p, status: "expired" as const, message }
      : p,
  );
}

export interface CopilotSlice {
  copilotOpen: boolean;
  copilotThreads: CopilotThreadView[];
  /** The active thread id, or null for a fresh (unsaved) conversation. */
  activeThreadId: string | null;
  /** Messages of the active thread. */
  copilotMessages: CopilotMessageView[];
  /** Live-streaming assistant text for the in-flight turn (before the final message lands). */
  copilotStreaming: string;
  copilotStatus: CopilotStatus;
  /** Name of the tool the agent is currently running, for a "· searching…" affordance. */
  copilotActiveTool: string | null;
  /** Every tool the in-flight turn has run so far, with per-tool outcome. */
  copilotToolTrail: CopilotToolRun[];
  /** The model is inside a thinking block (no visible tokens yet). */
  copilotThinking: boolean;
  /** The session id of the in-flight turn — gates streaming events to the right turn. */
  copilotSessionId: string | null;
  copilotError: string | null;
  /** Machine code for the error (e.g. "max_turns" renders a one-click Continue). */
  copilotErrorCode: string | null;
  /** Gated actions the agent proposed (all threads; render filtered by threadId). */
  copilotProposals: CopilotProposal[];
  /** Timestamp of the last live event for the in-flight turn — feeds the stuck-turn watchdog. */
  copilotLastEventAt: number | null;
  /** Nonce bumped on every WS reconnect — triggers a thread reconcile while streaming. */
  copilotWsReconnects: number;
  /** One message queued while a turn runs — auto-sent when the turn finishes. */
  copilotQueuedText: string | null;

  toggleCopilot: () => void;
  setCopilotOpen: (open: boolean) => void;
  setCopilotThreads: (threads: CopilotThreadView[]) => void;
  /** Switch to a persisted thread and hydrate its messages. */
  openCopilotThread: (threadId: string, messages: CopilotMessageView[]) => void;
  /** Start a fresh, empty conversation. */
  newCopilotThread: () => void;
  /** Optimistically append the user's turn before the request resolves; returns the local id. */
  appendCopilotUserMessage: (text: string) => string;
  /** Remove an optimistic message after a failed send (its text goes back to the composer). */
  dropCopilotLocalMessage: (id: string) => void;
  /** After the send resolves: bind the thread + session and enter the streaming state. */
  beginCopilotTurn: (threadId: string, sessionId: string) => void;
  appendCopilotToken: (sessionId: string, delta: string) => void;
  setCopilotTool: (sessionId: string, name: string, label: string) => void;
  /** A tool finished — mark its trail entry with the outcome. */
  finishCopilotTool: (sessionId: string, name: string, ok: boolean) => void;
  /** The model entered a thinking block (cleared by the next token/tool/message). */
  setCopilotThinking: (sessionId: string) => void;
  finishCopilotMessage: (sessionId: string, message: CopilotMessageView) => void;
  endCopilotTurn: (sessionId: string) => void;
  setCopilotError: (sessionId: string, message: string, code?: string) => void;
  /** User hit stop: keep whatever streamed so far as a message and end the turn. */
  stopCopilotTurn: () => void;
  addCopilotProposal: (proposal: {
    actionId: string;
    threadId: string;
    sessionId: string;
    tool: string;
    summary: string;
  }) => void;
  /** Local lifecycle transitions (approving/rejecting/revert-to-pending with a note). */
  markCopilotProposal: (
    actionId: string,
    status: "pending" | "approving" | "rejecting",
    message?: string,
  ) => void;
  resolveCopilotProposal: (actionId: string, ok: boolean, message: string) => void;
  /**
   * Reconcile the active thread against a fresh GetThread fetch (watchdog / WS-reconnect
   * recovery). Replaces messages; if the fetched tail holds an assistant answer the live
   * stream never delivered, the in-flight turn is considered finished server-side and the
   * streaming state is cleared.
   */
  reconcileCopilotThread: (threadId: string, messages: CopilotMessageView[]) => void;
  noteCopilotWsReconnect: () => void;
  /** Queue (or replace) one message to auto-send when the running turn finishes. */
  queueCopilotText: (text: string | null) => void;
  /** Take the queued message (clearing it) — used by the auto-send effect. */
  takeCopilotQueuedText: () => string | null;
  /** The thread id persisted from the previous app run (for reload rehydrate). */
  persistedCopilotThreadId: () => string | null;
}

export const createCopilotSlice: StateCreator<CopilotSlice, [], [], CopilotSlice> = (set, get) => ({
  copilotOpen: false,
  copilotThreads: [],
  activeThreadId: null,
  copilotMessages: [],
  copilotStreaming: "",
  copilotStatus: "idle",
  copilotActiveTool: null,
  copilotToolTrail: [],
  copilotThinking: false,
  copilotSessionId: null,
  copilotError: null,
  copilotErrorCode: null,
  copilotProposals: [],
  copilotLastEventAt: null,
  copilotWsReconnects: 0,
  copilotQueuedText: null,

  toggleCopilot: () => set((state) => ({ copilotOpen: !state.copilotOpen })),
  setCopilotOpen: (open) => set({ copilotOpen: open }),
  setCopilotThreads: (threads) => set({ copilotThreads: threads }),

  openCopilotThread: (threadId, messages) =>
    set((state) => {
      persistThreadId(threadId);
      return {
        activeThreadId: threadId,
        copilotMessages: messages,
        copilotStreaming: "",
        copilotStatus: "idle" as CopilotStatus,
        copilotActiveTool: null,
        copilotToolTrail: [],
        copilotThinking: false,
        copilotSessionId: null,
        copilotError: null,
        copilotErrorCode: null,
        copilotQueuedText: null,
        // Proposals survive thread switches (the server-side hold does too), but any
        // unresolved proposal for THIS thread from a session we no longer track can't
        // be settled by events anymore — expire it rather than show a dead button.
        copilotProposals: state.copilotProposals.map((p) =>
          p.threadId === threadId &&
          p.sessionId !== state.copilotSessionId &&
          (p.status === "pending" || p.status === "approving" || p.status === "rejecting")
            ? { ...p, status: "expired" as const, message: "That approval expired." }
            : p,
        ),
      };
    }),

  newCopilotThread: () => {
    persistThreadId(null);
    set({
      activeThreadId: null,
      copilotMessages: [],
      copilotStreaming: "",
      copilotStatus: "idle",
      copilotActiveTool: null,
      copilotToolTrail: [],
      copilotThinking: false,
      copilotSessionId: null,
      copilotError: null,
      copilotErrorCode: null,
      copilotQueuedText: null,
    });
  },

  appendCopilotUserMessage: (text) => {
    const id = `local-${Date.now()}`;
    set((state) => ({
      copilotMessages: [...state.copilotMessages, { id, role: "user", content: text, citations: [] }],
      copilotStatus: "sending",
      copilotError: null,
      copilotErrorCode: null,
    }));
    return id;
  },

  dropCopilotLocalMessage: (id) =>
    set((state) => ({
      copilotMessages: state.copilotMessages.filter((m) => m.id !== id),
    })),

  beginCopilotTurn: (threadId, sessionId) => {
    persistThreadId(threadId);
    set({
      activeThreadId: threadId,
      copilotSessionId: sessionId,
      copilotStatus: "streaming",
      copilotStreaming: "",
      copilotActiveTool: null,
      copilotToolTrail: [],
      copilotThinking: false,
      copilotLastEventAt: Date.now(),
    });
  },

  appendCopilotToken: (sessionId, delta) =>
    set((state) => {
      if (state.copilotSessionId !== sessionId) return {};
      return {
        copilotStreaming: state.copilotStreaming + delta,
        copilotActiveTool: null,
        copilotThinking: false,
        copilotLastEventAt: Date.now(),
      };
    }),

  setCopilotTool: (sessionId, name, label) =>
    set((state) => {
      if (state.copilotSessionId !== sessionId) return {};
      return {
        copilotActiveTool: label,
        copilotThinking: false,
        copilotToolTrail: [
          ...state.copilotToolTrail.slice(-maxToolTrail + 1),
          { name, label, status: "running" as const },
        ],
        copilotLastEventAt: Date.now(),
      };
    }),

  finishCopilotTool: (sessionId, name, ok) =>
    set((state) => {
      if (state.copilotSessionId !== sessionId) return {};
      // Mark the OLDEST unresolved run of this tool (results come back in order).
      let marked = false;
      const trail = state.copilotToolTrail.map((t) => {
        if (!marked && t.name === name && t.status === "running") {
          marked = true;
          return { ...t, status: ok ? ("ok" as const) : ("error" as const) };
        }
        return t;
      });
      const stillRunning = trail.some((t) => t.status === "running");
      return {
        copilotToolTrail: trail,
        copilotActiveTool: stillRunning ? state.copilotActiveTool : null,
        copilotLastEventAt: Date.now(),
      };
    }),

  setCopilotThinking: (sessionId) =>
    set((state) =>
      state.copilotSessionId === sessionId
        ? { copilotThinking: true, copilotLastEventAt: Date.now() }
        : {},
    ),

  finishCopilotMessage: (sessionId, message) =>
    set((state) => {
      if (state.copilotSessionId !== sessionId) return {};
      return {
        copilotMessages: [...state.copilotMessages, message],
        copilotStreaming: "",
        copilotActiveTool: null,
        copilotThinking: false,
        copilotLastEventAt: Date.now(),
      };
    }),

  endCopilotTurn: (sessionId) =>
    set((state) => {
      if (state.copilotSessionId !== sessionId) return {};
      return {
        copilotStatus: "idle" as CopilotStatus,
        copilotSessionId: null,
        copilotActiveTool: null,
        copilotThinking: false,
        copilotProposals: expireForSession(state.copilotProposals, sessionId, "That approval expired."),
      };
    }),

  setCopilotError: (sessionId, message, code) =>
    set((state) => {
      if (state.copilotSessionId !== sessionId) return {};
      return {
        copilotError: message,
        copilotErrorCode: code ?? null,
        copilotStatus: "idle" as CopilotStatus,
        copilotActiveTool: null,
        copilotThinking: false,
        copilotStreaming: "",
        copilotProposals: expireForSession(state.copilotProposals, sessionId, "That approval expired."),
      };
    }),

  stopCopilotTurn: () =>
    set((state) => {
      const partial = state.copilotStreaming.trim();
      const messages = partial
        ? [
            ...state.copilotMessages,
            { id: `local-stop-${Date.now()}`, role: "assistant" as const, content: state.copilotStreaming, citations: [] },
          ]
        : state.copilotMessages;
      return {
        copilotMessages: messages,
        copilotStreaming: "",
        copilotStatus: "idle" as CopilotStatus,
        copilotSessionId: null,
        copilotActiveTool: null,
        copilotThinking: false,
        // The sidecar denies held approvals silently on cancel — no action_result will come.
        copilotProposals: expireForSession(state.copilotProposals, state.copilotSessionId, "Cancelled."),
      };
    }),

  addCopilotProposal: (proposal) =>
    set((state) => {
      // Same gate as every other live event: only the in-flight turn may add proposals
      // (a second window streams its own turn; it must not grow cards here).
      if (state.copilotSessionId !== proposal.sessionId) return {};
      return {
        copilotProposals: [...state.copilotProposals, { ...proposal, status: "pending" as const }],
        copilotLastEventAt: Date.now(),
      };
    }),

  markCopilotProposal: (actionId, status, message) =>
    set((state) => ({
      copilotProposals: state.copilotProposals.map((p) =>
        p.actionId === actionId ? { ...p, status, message } : p,
      ),
    })),

  resolveCopilotProposal: (actionId, ok, message) =>
    set((state) => ({
      copilotProposals: state.copilotProposals.map((p) =>
        p.actionId === actionId
          ? { ...p, status: ok ? ("approved" as const) : ("rejected" as const), message }
          : p,
      ),
    })),

  reconcileCopilotThread: (threadId, messages) =>
    set((state) => {
      if (state.activeThreadId !== threadId) return {};
      const fetchedTail = messages[messages.length - 1];
      const knownIds = new Set(state.copilotMessages.map((m) => m.id));
      const turnLandedServerSide =
        fetchedTail?.role === "assistant" && !knownIds.has(fetchedTail.id);
      if (!turnLandedServerSide) {
        // Turn genuinely still running (or nothing new) — refresh history, keep the stream.
        return state.copilotStatus === "streaming"
          ? {}
          : { copilotMessages: messages };
      }
      return {
        copilotMessages: messages,
        copilotStreaming: "",
        copilotStatus: "idle" as CopilotStatus,
        copilotActiveTool: null,
        copilotThinking: false,
        copilotSessionId: null,
        copilotProposals: expireForSession(
          state.copilotProposals,
          state.copilotSessionId,
          "That approval expired.",
        ),
      };
    }),

  noteCopilotWsReconnect: () =>
    set((state) => ({ copilotWsReconnects: state.copilotWsReconnects + 1 })),

  queueCopilotText: (text) => set({ copilotQueuedText: text }),

  takeCopilotQueuedText: () => {
    const text = get().copilotQueuedText;
    if (text !== null) set({ copilotQueuedText: null });
    return text;
  },

  persistedCopilotThreadId: () => readPersistedThreadId(),
});
