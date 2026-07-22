import type { StateCreator } from "zustand";

/** One grounding source the assistant used — rendered as a clickable chip. */
export interface CopilotCitation {
  kind: string; // ticker | company | person | entity | page | shard
  id: string;
  title: string;
}

/** One visible turn in a copilot thread. */
export interface CopilotMessageView {
  id: string;
  role: "user" | "assistant";
  content: string;
  citations: CopilotCitation[];
}

/** A thread as shown in the thread switcher. */
export interface CopilotThreadView {
  id: string;
  title: string;
  updatedAt: string;
}

export type CopilotStatus = "idle" | "sending" | "streaming";

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
  /** The session id of the in-flight turn — gates streaming events to the right turn. */
  copilotSessionId: string | null;
  copilotError: string | null;

  toggleCopilot: () => void;
  setCopilotOpen: (open: boolean) => void;
  setCopilotThreads: (threads: CopilotThreadView[]) => void;
  /** Switch to a persisted thread and hydrate its messages. */
  openCopilotThread: (threadId: string, messages: CopilotMessageView[]) => void;
  /** Start a fresh, empty conversation. */
  newCopilotThread: () => void;
  /** Optimistically append the user's turn before the request resolves. */
  appendCopilotUserMessage: (text: string) => void;
  /** After the send resolves: bind the thread + session and enter the streaming state. */
  beginCopilotTurn: (threadId: string, sessionId: string) => void;
  appendCopilotToken: (sessionId: string, delta: string) => void;
  setCopilotTool: (sessionId: string, name: string) => void;
  finishCopilotMessage: (sessionId: string, message: CopilotMessageView) => void;
  endCopilotTurn: (sessionId: string) => void;
  setCopilotError: (sessionId: string, message: string) => void;
  /** User hit stop: keep whatever streamed so far as a message and end the turn. */
  stopCopilotTurn: () => void;
}

export const createCopilotSlice: StateCreator<CopilotSlice, [], [], CopilotSlice> = (set) => ({
  copilotOpen: false,
  copilotThreads: [],
  activeThreadId: null,
  copilotMessages: [],
  copilotStreaming: "",
  copilotStatus: "idle",
  copilotActiveTool: null,
  copilotSessionId: null,
  copilotError: null,

  toggleCopilot: () => set((state) => ({ copilotOpen: !state.copilotOpen })),
  setCopilotOpen: (open) => set({ copilotOpen: open }),
  setCopilotThreads: (threads) => set({ copilotThreads: threads }),

  openCopilotThread: (threadId, messages) =>
    set({
      activeThreadId: threadId,
      copilotMessages: messages,
      copilotStreaming: "",
      copilotStatus: "idle",
      copilotActiveTool: null,
      copilotSessionId: null,
      copilotError: null,
    }),

  newCopilotThread: () =>
    set({
      activeThreadId: null,
      copilotMessages: [],
      copilotStreaming: "",
      copilotStatus: "idle",
      copilotActiveTool: null,
      copilotSessionId: null,
      copilotError: null,
    }),

  appendCopilotUserMessage: (text) =>
    set((state) => ({
      copilotMessages: [
        ...state.copilotMessages,
        { id: `local-${Date.now()}`, role: "user", content: text, citations: [] },
      ],
      copilotStatus: "sending",
      copilotError: null,
    })),

  beginCopilotTurn: (threadId, sessionId) =>
    set({
      activeThreadId: threadId,
      copilotSessionId: sessionId,
      copilotStatus: "streaming",
      copilotStreaming: "",
      copilotActiveTool: null,
    }),

  appendCopilotToken: (sessionId, delta) =>
    set((state) => {
      if (state.copilotSessionId !== sessionId) return {};
      return { copilotStreaming: state.copilotStreaming + delta, copilotActiveTool: null };
    }),

  setCopilotTool: (sessionId, name) =>
    set((state) => (state.copilotSessionId === sessionId ? { copilotActiveTool: name } : {})),

  finishCopilotMessage: (sessionId, message) =>
    set((state) => {
      if (state.copilotSessionId !== sessionId) return {};
      return {
        copilotMessages: [...state.copilotMessages, message],
        copilotStreaming: "",
        copilotActiveTool: null,
      };
    }),

  endCopilotTurn: (sessionId) =>
    set((state) => {
      if (state.copilotSessionId !== sessionId) return {};
      return { copilotStatus: "idle", copilotSessionId: null, copilotActiveTool: null };
    }),

  setCopilotError: (sessionId, message) =>
    set((state) => {
      if (state.copilotSessionId !== sessionId) return {};
      return { copilotError: message, copilotStatus: "idle", copilotActiveTool: null, copilotStreaming: "" };
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
      };
    }),
});
