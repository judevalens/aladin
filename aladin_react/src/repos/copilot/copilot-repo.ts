import type { ApiClient } from "@/shared/api/client";
import type {
  CopilotCitation,
  CopilotMessageView,
  CopilotThreadView,
} from "@/app/state/copilot-slice";

/** What the user is looking at when they ask — makes the agent context-aware. */
export interface CopilotSurface {
  kind: string;
  id?: string;
  symbol?: string;
  label?: string;
}

export interface CopilotSendRequest {
  threadId?: string;
  text: string;
  surface?: CopilotSurface;
}

export interface CopilotSendResult {
  threadId: string;
  sessionId: string;
}

export interface CopilotThreadDetail {
  thread: CopilotThreadView;
  messages: CopilotMessageView[];
}

// The wire shape of a persisted message (createdAt is dropped from the view).
interface CopilotMessageWire {
  id: string;
  role: string;
  content: string;
  citations?: CopilotCitation[] | null;
}

export interface CopilotRepo {
  /** Start/continue a thread. Returns immediately; the answer streams over the realtime WS. */
  sendMessage(req: CopilotSendRequest): Promise<CopilotSendResult>;
  listThreads(): Promise<CopilotThreadView[]>;
  getThread(threadId: string): Promise<CopilotThreadDetail>;
}

function toMessageView(m: CopilotMessageWire): CopilotMessageView {
  return {
    id: m.id,
    role: m.role === "assistant" ? "assistant" : "user",
    content: m.content,
    citations: m.citations ?? [],
  };
}

export function createCopilotRepo(client: ApiClient): CopilotRepo {
  return {
    sendMessage: (req) =>
      client.fetch<CopilotSendResult>("/api/copilot/message", {
        method: "POST",
        body: JSON.stringify({
          threadId: req.threadId,
          text: req.text,
          surface: req.surface,
        }),
      }),

    listThreads: () =>
      client
        .fetch<{ threads: CopilotThreadView[] | null }>("/api/copilot/threads")
        .then((res) => res.threads ?? []),

    getThread: (threadId) =>
      client
        .fetch<{ thread: CopilotThreadView; messages: CopilotMessageWire[] | null }>(
          `/api/copilot/threads/${encodeURIComponent(threadId)}`,
        )
        .then((res) => ({
          thread: res.thread,
          messages: (res.messages ?? []).map(toMessageView),
        })),
  };
}
