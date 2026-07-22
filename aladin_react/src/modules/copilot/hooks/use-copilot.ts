import { useCallback } from "react";
import { useAppComposition } from "@/app/composition/app-composition";
import { useAppStore } from "@/app/state/store";
import { useCurrentSurface } from "@/modules/copilot/hooks/use-current-surface";

/**
 * Binds the copilot dock to its repo + store: sending a turn (surface-aware), loading threads,
 * and switching threads. Streaming updates arrive via the copilot realtime handler → slice, so
 * this hook only kicks off requests and exposes reactive state.
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
  const error = useAppStore((s) => s.copilotError);
  const proposals = useAppStore((s) => s.copilotProposals);

  const loadThreads = useCallback(async () => {
    try {
      const list = await repos.copilot.listThreads();
      useAppStore.getState().setCopilotThreads(list);
    } catch {
      // non-fatal: the switcher just stays as-is
    }
  }, [repos.copilot]);

  const send = useCallback(
    async (rawText: string) => {
      const text = rawText.trim();
      if (!text) return;
      const store = useAppStore.getState();
      const threadId = store.activeThreadId ?? undefined;
      store.appendCopilotUserMessage(text);
      try {
        const result = await repos.copilot.sendMessage({ threadId, text, surface });
        useAppStore.getState().beginCopilotTurn(result.threadId, result.sessionId);
        void loadThreads();
      } catch (err) {
        const message = err instanceof Error ? err.message : "Couldn't send that.";
        useAppStore.setState({ copilotError: message, copilotStatus: "idle" });
      }
    },
    [repos.copilot, surface, loadThreads],
  );

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

  const stop = useCallback(() => {
    const sessionId = useAppStore.getState().copilotSessionId;
    if (sessionId) void repos.copilot.cancel(sessionId).catch(() => {});
    useAppStore.getState().stopCopilotTurn();
  }, [repos.copilot]);

  const approveProposal = useCallback(
    (actionId: string) => void repos.copilot.approveAction(actionId).catch(() => {}),
    [repos.copilot],
  );

  const rejectProposal = useCallback(
    (actionId: string) => {
      useAppStore.getState().resolveCopilotProposal(actionId, false, "Dismissed.");
      void repos.copilot.rejectAction(actionId).catch(() => {});
    },
    [repos.copilot],
  );

  const newThread = useCallback(() => useAppStore.getState().newCopilotThread(), []);
  const setOpen = useCallback((next: boolean) => useAppStore.getState().setCopilotOpen(next), []);

  return {
    open,
    setOpen,
    threads,
    activeThreadId,
    messages,
    streaming,
    status,
    activeTool,
    error,
    surface,
    proposals,
    send,
    stop,
    approveProposal,
    rejectProposal,
    loadThreads,
    openThread,
    newThread,
  };
}
