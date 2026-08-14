import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type {
  CopilotMessageView,
  CopilotProposal,
  CopilotThreadView,
} from "@/app/state/copilot-slice";
import type { CopilotSurface } from "@/repos/copilot/copilot-repo";
import { CopilotDockUI } from "@/modules/copilot/ui/copilot-dock-ui";

const mockedCopilot = vi.hoisted(() => ({
  current: null as ReturnType<typeof makeCopilotState> | null,
}));

vi.mock("@/modules/copilot/hooks/use-copilot", () => ({
  useCopilot: () => mockedCopilot.current,
}));

vi.mock("@/modules/copilot/hooks/use-citation-nav", () => ({
  useCitationNav: () => vi.fn(),
}));

describe("CopilotDockUI integration", () => {
  beforeEach(() => {
    mockedCopilot.current = makeCopilotState();
  });

  it("transitions from the empty transcript state to rendered messages", () => {
    const { rerender } = render(<CopilotDockUI />);

    expect(screen.getByText("Ask about your research — grounded in your Aladin data.")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Summarize what I'm looking at" })).toBeTruthy();

    mockedCopilot.current = makeCopilotState({
      messages: [assistantMessage("m1", "Existing answer.")],
    });
    rerender(<CopilotDockUI />);

    expect(screen.queryByText("Ask about your research — grounded in your Aladin data.")).toBeNull();
    expect(screen.getByText("Existing answer.")).toBeTruthy();
  });

  it("renders only proposals scoped to the active thread", () => {
    mockedCopilot.current = makeCopilotState({
      activeThreadId: "t-active",
      proposals: [
        proposal("t-active", "Publish active shard"),
        proposal("t-other", "Publish other shard"),
      ],
    });

    render(<CopilotDockUI />);

    expect(screen.getByText("Publish active shard")).toBeTruthy();
    expect(screen.queryByText("Publish other shard")).toBeNull();
    expect(screen.getByText("waiting for your approval…")).toBeTruthy();
  });

  it("shows and clears the latest control when a busy transcript is unpinned", () => {
    mockedCopilot.current = makeCopilotState({
      status: "streaming",
      streaming: "Still working.",
      messages: [assistantMessage("m1", "Earlier answer.")],
    });

    render(<CopilotDockUI />);
    const transcript = screen.getByRole("log", { name: "Copilot transcript" });
    Object.defineProperty(transcript, "scrollHeight", { configurable: true, value: 1000 });
    Object.defineProperty(transcript, "clientHeight", { configurable: true, value: 300 });
    transcript.scrollTop = 100;

    fireEvent.scroll(transcript);
    fireEvent.click(screen.getByRole("button", { name: /latest/i }));

    expect(screen.queryByRole("button", { name: /latest/i })).toBeNull();
    expect(transcript.scrollTop).toBe(1000);
  });

  it("renders the model switcher and changes the selected model", () => {
    const setSelectedModel = vi.fn();
    mockedCopilot.current = makeCopilotState({
      activeModel: "claude-opus-5",
      defaultModel: "claude-opus-5",
      setSelectedModel,
      modelOptions: [
        { id: "claude-opus-5", label: "Opus 5", description: "Deep work." },
        { id: "claude-sonnet-5", label: "Sonnet 5", description: "Fast work." },
      ],
    });

    render(<CopilotDockUI />);

    fireEvent.pointerDown(screen.getByRole("button", { name: "Copilot model" }));
    fireEvent.click(screen.getByText("Sonnet 5"));

    expect(setSelectedModel).toHaveBeenCalledWith("claude-sonnet-5");
  });
});

function makeCopilotState(overrides: Partial<CopilotHookState> = {}): CopilotHookState {
  return {
    open: true,
    setOpen: vi.fn(),
    threads: [{ id: "t-active", title: "Active thread", updatedAt: "2026-01-01T00:00:00Z" }],
    activeThreadId: "t-active",
    messages: [],
    streaming: "",
    status: "idle",
    activeTool: null,
    toolTrail: [],
    thinking: false,
    error: null,
    errorCode: null,
    surface: { kind: "page" },
    proposals: [],
    send: vi.fn(async () => null),
    stop: vi.fn(),
    queueFollowup: vi.fn(),
    approveProposal: vi.fn(),
    rejectProposal: vi.fn(),
    loadThreads: vi.fn(async () => undefined),
    openThread: vi.fn(async () => undefined),
    renameThread: vi.fn(async () => true),
    archiveThread: vi.fn(async () => true),
    setThreadPinned: vi.fn(async () => true),
    newThread: vi.fn(),
    fetchHealthWarning: vi.fn(async () => null),
    modelOptions: [{ id: "claude-opus-5", label: "Opus 5", description: "Deep work." }],
    activeModel: "claude-opus-5",
    defaultModel: "claude-opus-5",
    setSelectedModel: vi.fn(),
    queuedText: null,
    draftText: "",
    setDraftText: vi.fn(),
    realtimeState: "open",
    ...overrides,
  };
}

type CopilotHookState = {
  open: boolean;
  setOpen: (open: boolean) => void;
  threads: CopilotThreadView[];
  activeThreadId: string | null;
  messages: CopilotMessageView[];
  streaming: string;
  status: "idle" | "sending" | "streaming";
  activeTool: string | null;
  toolTrail: [];
  thinking: boolean;
  error: string | null;
  errorCode: string | null;
  surface: CopilotSurface;
  proposals: CopilotProposal[];
  send: (text: string) => Promise<string | null>;
  stop: () => void;
  queueFollowup: (text: string | null) => void;
  approveProposal: (actionId: string) => void;
  rejectProposal: (actionId: string) => void;
  loadThreads: () => Promise<void>;
  openThread: (threadId: string) => Promise<void>;
  renameThread: (threadId: string, title: string) => Promise<boolean>;
  archiveThread: (threadId: string) => Promise<boolean>;
  setThreadPinned: (threadId: string, pinned: boolean) => Promise<boolean>;
  newThread: () => void;
  fetchHealthWarning: () => Promise<string | null>;
  modelOptions: { id: string; label: string; description?: string }[];
  activeModel: string | null;
  defaultModel: string | null;
  setSelectedModel: (model: string) => void;
  queuedText: string | null;
  draftText: string;
  setDraftText: (text: string) => void;
  realtimeState: "connecting" | "open" | "closed";
};

function assistantMessage(id: string, content: string): CopilotMessageView {
  return { id, role: "assistant", content, citations: [] };
}

function proposal(threadId: string, summary: string): CopilotProposal {
  return {
    actionId: `proposal-${threadId}`,
    threadId,
    sessionId: "s1",
    tool: "publish_app",
    summary,
    status: "pending",
  };
}
