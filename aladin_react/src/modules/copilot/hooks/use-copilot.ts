import { useCallback, useEffect } from "react";
import { useAppComposition } from "@/app/composition/app-composition";
import { useAppStore } from "@/app/state/store";
import { useCurrentSurface } from "@/modules/copilot/hooks/use-current-surface";

/**
 * How long the in-flight turn may go without a single live event before the watchdog
 * suspects the completion was lost (WS gap) and reconciles against GetThread. Long tool
 * calls are fine — every tool emits copilot.tool first, which stamps the clock; the one
 * legit long silence is an approval hold, and the watchdog skips those.
 */
const turnSilenceMs = 60_000;
const watchdogIntervalMs = 15_000;

/** Pure predicate so the watchdog rule is unit-testable. */
export function turnLooksStuck(input: {
  status: string;
  lastEventAt: number | null;
  now: number;
  awaitingApproval: boolean;
}): boolean {
  if (input.status !== "streaming") return false;
  if (input.awaitingApproval) return false;
  if (input.lastEventAt === null) return false;
  return input.now - input.lastEventAt > turnSilenceMs;
}

/**
 * Binds the copilot dock to its repo + store: sending a turn (surface-aware), loading threads,
 * switching threads, approvals, and stuck-turn recovery. Streaming updates arrive via the
 * copilot realtime handler → slice; this hook kicks off requests and exposes reactive state.
 */
export function useCopilot() {
  const { repos } = useAppComposition();
  const surface = useCurrentSurface();

  const open = useAppStore((s) => s.copilotOpen);
  const threads = useAppStore((s) => s.copilotThreads);
  const activeThreadId = useAppStore((s) => s.activeThreadId);
  const messages = useAppStore((s) => s.copilotMessages);
  const streaming = useAppStore((s) => s.copilotStreaming);
  const status = useAppStore((s) => s.copilotStatus);
  const activeTool = useAppStore((s) => s.copilotActiveTool);
  const toolTrail = useAppStore((s) => s.copilotToolTrail);
  const thinking = useAppStore((s) => s.copilotThinking);
  const error = useAppStore((s) => s.copilotError);
  const errorCode = useAppStore((s) => s.copilotErrorCode);
  const proposals = useAppStore((s) => s.copilotProposals);
  const wsReconnects = useAppStore((s) => s.copilotWsReconnects);
  const draftText = useAppStore((s) => s.copilotDraftFor(s.activeThreadId));
  const realtimeState = useAppStore((s) => s.copilotRealtimeState);

  const loadThreads = useCallback(async () => {
    try {
      const list = await repos.copilot.listThreads();
      useAppStore.getState().setCopilotThreads(list);
    } catch {
      // non-fatal: the switcher just stays as-is
    }
  }, [repos.copilot]);

  /**
   * send kicks off a turn. On failure the optimistic user message is removed and the
   * text is returned so the composer can restore it; returns null on success.
   */
  const send = useCallback(
    async (rawText: string): Promise<string | null> => {
      const text = rawText.trim();
      if (!text) return null;
      const store = useAppStore.getState();
      const threadId = store.activeThreadId ?? undefined;
      const localId = store.appendCopilotUserMessage(text);
      try {
        const result = await repos.copilot.sendMessage({ threadId, text, surface });
        useAppStore.getState().beginCopilotTurn(result.threadId, result.sessionId);
        void loadThreads();
        return null;
      } catch (err) {
        const message = err instanceof Error ? err.message : "Couldn't send that.";
        const s = useAppStore.getState();
        s.dropCopilotLocalMessage(localId);
        useAppStore.setState({ copilotError: message, copilotErrorCode: null, copilotStatus: "idle" });
        return text;
      }
    },
    [repos.copilot, surface, loadThreads],
  );

  const reconcileActiveThread = useCallback(async () => {
    const threadId = useAppStore.getState().activeThreadId;
    if (!threadId) return;
    try {
      const detail = await repos.copilot.getThread(threadId);
      useAppStore.getState().reconcileCopilotThread(threadId, detail.messages);
    } catch {
      // transient — the watchdog will retry on its next tick
    }
  }, [repos.copilot]);

  const openThread = useCallback(
    async (threadId: string) => {
      try {
        const detail = await repos.copilot.getThread(threadId);
        useAppStore.getState().openCopilotThread(threadId, detail.messages);
      } catch {
        // ignore — keep the current view
      }
    },
    [repos.copilot],
  );

  const renameThread = useCallback(
    async (threadId: string, title: string) => {
      const trimmed = title.trim();
      if (!trimmed) return false;
      try {
        const thread = await repos.copilot.renameThread(threadId, trimmed);
        useAppStore.getState().renameCopilotThreadLocal(thread);
        void loadThreads();
        return true;
      } catch {
        useAppStore.setState({
          copilotError: "Couldn't rename that thread — try again.",
          copilotErrorCode: null,
        });
        return false;
      }
    },
    [repos.copilot, loadThreads],
  );

  const archiveThread = useCallback(
    async (threadId: string) => {
      try {
        await repos.copilot.archiveThread(threadId);
        useAppStore.getState().archiveCopilotThreadLocal(threadId);
        void loadThreads();
        return true;
      } catch {
        useAppStore.setState({
          copilotError: "Couldn't archive that thread — try again.",
          copilotErrorCode: null,
        });
        return false;
      }
    },
    [repos.copilot, loadThreads],
  );

  const stop = useCallback(() => {
    const sessionId = useAppStore.getState().copilotSessionId;
    if (sessionId) {
      // Retry once; on double failure tell the user the turn may still be running.
      void repos.copilot
        .cancel(sessionId)
        .catch(() => repos.copilot.cancel(sessionId))
        .catch(() => {
          useAppStore.setState({
            copilotError: "Couldn't stop the turn — it may still be running.",
            copilotErrorCode: null,
          });
        });
    }
    useAppStore.getState().stopCopilotTurn();
  }, [repos.copilot]);

  const approveProposal = useCallback(
    (actionId: string) => {
      const store = useAppStore.getState();
      store.markCopilotProposal(actionId, "approving");
      void repos.copilot.approveAction(actionId).catch(() => {
        useAppStore
          .getState()
          .markCopilotProposal(actionId, "pending", "Couldn't reach the server — try again.");
      });
      // Success resolution arrives via copilot.action_result once the tool runs.
    },
    [repos.copilot],
  );

  const rejectProposal = useCallback(
    (actionId: string) => {
      const store = useAppStore.getState();
      store.markCopilotProposal(actionId, "rejecting");
      void repos.copilot
        .rejectAction(actionId)
        .then(() => {
          // The server's action_result broadcast will confirm; resolve locally too so
          // the card settles even if that event is lost.
          useAppStore.getState().resolveCopilotProposal(actionId, false, "Dismissed.");
        })
        .catch(() => {
          useAppStore
            .getState()
            .markCopilotProposal(actionId, "pending", "Couldn't reach the server — try again.");
        });
    },
    [repos.copilot],
  );

  // Stuck-turn watchdog: if the live stream has gone silent for too long (and we're not
  // deliberately holding for an approval), the completion probably fell into a WS gap —
  // reconcile against the server's durable record.
  useEffect(() => {
    if (status !== "streaming") return;
    const interval = window.setInterval(() => {
      const s = useAppStore.getState();
      const awaitingApproval = s.copilotProposals.some(
        (p) =>
          p.sessionId === s.copilotSessionId &&
          (p.status === "pending" || p.status === "approving"),
      );
      if (
        turnLooksStuck({
          status: s.copilotStatus,
          lastEventAt: s.copilotLastEventAt,
          now: Date.now(),
          awaitingApproval,
        })
      ) {
        void reconcileActiveThread();
      }
    }, watchdogIntervalMs);
    return () => window.clearInterval(interval);
  }, [status, reconcileActiveThread]);

  // WS reconnected while a turn was streaming: events may have been lost in the gap —
  // reconcile immediately rather than waiting out the watchdog.
  useEffect(() => {
    if (wsReconnects === 0) return;
    if (useAppStore.getState().copilotStatus === "streaming") {
      void reconcileActiveThread();
    }
  }, [wsReconnects, reconcileActiveThread]);

  // Queue-of-one: a message typed during a turn auto-sends when the turn finishes.
  const queuedText = useAppStore((s) => s.copilotQueuedText);
  useEffect(() => {
    if (status !== "idle" || queuedText === null) return;
    const text = useAppStore.getState().takeCopilotQueuedText();
    if (text) void send(text);
  }, [status, queuedText, send]);

  /** Preflight health → a user-facing warning line, or null when all is well/unknown. */
  const fetchHealthWarning = useCallback(async (): Promise<string | null> => {
    try {
      const s = await repos.copilot.getStatus();
      if (!s.configured) return "The copilot is not configured on this backend.";
      if (!s.sidecar) return "The copilot agent is unreachable — answers will fail.";
      if (!s.mcp) return "The copilot's tool server is unreachable — answers may fail. Is `make mcp` running?";
      return null;
    } catch {
      return null; // status endpoint itself failing is not worth a scare banner
    }
  }, [repos.copilot]);

  const newThread = useCallback(() => useAppStore.getState().newCopilotThread(), []);
  const setOpen = useCallback((next: boolean) => useAppStore.getState().setCopilotOpen(next), []);
  const setDraftText = useCallback(
    (text: string) =>
      useAppStore.getState().setCopilotDraft(useAppStore.getState().activeThreadId, text),
    [],
  );

  return {
    open,
    setOpen,
    threads,
    activeThreadId,
    messages,
    streaming,
    status,
    activeTool,
    toolTrail,
    thinking,
    error,
    errorCode,
    surface,
    proposals,
    send,
    stop,
    approveProposal,
    rejectProposal,
    loadThreads,
    openThread,
    renameThread,
    archiveThread,
    newThread,
    fetchHealthWarning,
    queuedText,
    draftText,
    setDraftText,
    realtimeState,
  };
}
