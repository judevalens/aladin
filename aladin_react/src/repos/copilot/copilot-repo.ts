import type { ApiClient } from "@/shared/api/client";
import type { ArtifactKind } from "@/shared/api/models";
import type {
  CopilotCitation,
  CopilotMessageMeta,
  CopilotMessageView,
  CopilotThreadView,
} from "@/app/state/copilot-slice";

/** What the user is looking at when they ask — makes the agent context-aware. */
export interface CopilotSurface {
  kind: string;
  id?: string;
  symbol?: string;
  label?: string;
  artifactKind?: ArtifactKind;
}

export interface CopilotSendRequest {
  threadId?: string;
  text: string;
  model?: string;
  effort?: string;
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
  meta?: CopilotMessageMeta | null;
}

export interface CopilotRepo {
  /** Start/continue a thread. Returns immediately; the answer streams over the realtime WS. */
  sendMessage(req: CopilotSendRequest): Promise<CopilotSendResult>;
  /** Stop an in-flight turn (halts the backend work + cost). */
  cancel(sessionId: string): Promise<void>;
  /** Approve / reject a proposed destructive action. */
  approveAction(actionId: string): Promise<void>;
  rejectAction(actionId: string): Promise<void>;
  listThreads(): Promise<CopilotThreadView[]>;
  getThread(threadId: string): Promise<CopilotThreadDetail>;
  renameThread(threadId: string, title: string): Promise<CopilotThreadView>;
  archiveThread(threadId: string): Promise<void>;
  setThreadPinned(threadId: string, pinned: boolean): Promise<CopilotThreadView>;
  /** Preflight health: is the sidecar up and its MCP tool server reachable? */
  getStatus(): Promise<CopilotStatus>;
}

export interface CopilotStatus {
  configured: boolean;
  sidecar: boolean;
  mcp: boolean;
  defaultModel?: string;
  models?: CopilotModelOption[] | null;
  defaultEffort?: string;
  efforts?: CopilotEffortOption[] | null;
}

export interface CopilotModelOption {
  id: string;
  label: string;
  description?: string;
}

export interface CopilotEffortOption {
  id: string;
  label: string;
  description?: string;
}

function toMessageView(m: CopilotMessageWire): CopilotMessageView {
  return {
    id: m.id,
    role: m.role === "assistant" ? "assistant" : "user",
    content: m.content,
    citations: m.citations ?? [],
    meta: m.meta ?? undefined,
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
          model: req.model,
          effort: req.effort,
          surface: req.surface,
        }),
      }),

    cancel: (sessionId) =>
      client
        .fetch<{ ok: boolean }>("/api/copilot/cancel", {
          method: "POST",
          body: JSON.stringify({ sessionId }),
        })
        .then(() => undefined),

    approveAction: (actionId) =>
      client
        .fetch<{ ok: boolean }>(`/api/copilot/action/${encodeURIComponent(actionId)}/approve`, { method: "POST" })
        .then(() => undefined),

    rejectAction: (actionId) =>
      client
        .fetch<{ ok: boolean }>(`/api/copilot/action/${encodeURIComponent(actionId)}/reject`, { method: "POST" })
        .then(() => undefined),

    getStatus: () => client.fetch<CopilotStatus>("/api/copilot/status"),

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

    renameThread: (threadId, title) =>
      client.fetch<CopilotThreadView>(`/api/copilot/threads/${encodeURIComponent(threadId)}`, {
        method: "PATCH",
        body: JSON.stringify({ title }),
      }),

    archiveThread: (threadId) =>
      client
        .fetch<{ ok: boolean }>(`/api/copilot/threads/${encodeURIComponent(threadId)}`, {
          method: "DELETE",
        })
        .then(() => undefined),

    setThreadPinned: (threadId, pinned) =>
      client.fetch<CopilotThreadView>(`/api/copilot/threads/${encodeURIComponent(threadId)}/pin`, {
        method: "POST",
        body: JSON.stringify({ pinned }),
      }),
  };
}
