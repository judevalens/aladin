import {
  AlertTriangle,
  Archive,
  ArrowUp,
  BarChart3,
  Check,
  ChevronDown,
  FileText,
  Focus,
  MessageSquare,
  Pencil,
  Pin,
  Plus,
  Search,
  Sparkles,
  Square,
  UserRound,
  X,
  type LucideIcon,
} from "lucide-react";
import { Icon } from "@/components/ui/icon";
import { useEffect, useLayoutEffect, useRef, useState, type MouseEvent } from "react";
import type { CopilotSurface } from "@/repos/copilot/copilot-repo";
import type { CopilotProposal, CopilotThreadView, CopilotToolRun } from "@/app/state/copilot-slice";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { ARTIFACT_ICONS } from "@/modules/workspace/ui/kind-icons";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useAppStore } from "@/app/state/store";
import { useCopilot } from "@/modules/copilot/hooks/use-copilot";
import { useCitationNav } from "@/modules/copilot/hooks/use-citation-nav";
import { CopilotMarkdown, StreamCaret } from "@/modules/copilot/ui/copilot-markdown";
import type {
  CopilotCitation,
  CopilotMessageMeta,
  CopilotMessageView,
} from "@/app/state/copilot-slice";
import { cn } from "@/lib/utils";

const DOCK_WIDTH = 384;
const COMPOSER_MIN_HEIGHT = 44;
const COMPOSER_MAX_HEIGHT = 144;

/**
 * The Copilot dock — Aladin's default agentic AI surface. Mounted once in the workspace shell
 * so the conversation survives navigation; collapsed to zero width (not unmounted) while closed.
 * Surface-aware: every turn carries what the user is looking at.
 */
export function CopilotDockUI() {
  const {
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
    queueFollowup,
    approveProposal,
    rejectProposal,
    loadThreads,
    openThread,
    renameThread,
    archiveThread,
    setThreadPinned,
    newThread,
    fetchHealthWarning,
    queuedText,
    draftText,
    setDraftText,
    realtimeState,
  } = useCopilot();

  const [threadQuery, setThreadQuery] = useState("");
  const scrollRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const busy = status === "sending" || status === "streaming";
  // Proposals are kept across thread switches; render only the active thread's.
  const activeProposals = proposals.filter((p) => p.threadId === activeThreadId);
  // The backend holds a gated tool open while any proposal is unactioned — show a
  // persistent (non-pulsing) line so the paused turn doesn't read as idle.
  const awaitingApproval = activeProposals.some(
    (p) => p.status === "pending" || p.status === "approving",
  );

  // Load the thread list + focus the composer when the dock opens; on a cold open
  // (fresh app run) rehydrate the last active thread so a reload mid-conversation
  // lands back where the user was. Also preflight the sidecar/MCP health so a dead
  // tool server warns before the user types (fire-and-forget; send is never blocked).
  const [healthWarning, setHealthWarning] = useState<string | null>(null);
  useEffect(() => {
    if (!open) return;
    void loadThreads();
    inputRef.current?.focus();
    const store = useAppStore.getState();
    if (!store.activeThreadId && store.copilotMessages.length === 0) {
      const persisted = store.persistedCopilotThreadId();
      if (persisted) void openThread(persisted);
    }
    void fetchHealthWarning().then(setHealthWarning);
  }, [open, loadThreads, openThread, fetchHealthWarning]);

  // Keep the transcript pinned to the latest turn / streamed token — but only while
  // the user is actually at the bottom. Scrolling up to read history unpins; the
  // "↓ latest" chip re-pins.
  const pinnedRef = useRef(true);
  const [unpinned, setUnpinned] = useState(false);
  useEffect(() => {
    const el = scrollRef.current;
    if (el && pinnedRef.current) el.scrollTop = el.scrollHeight;
  }, [messages, streaming, activeTool, open]);
  const onTranscriptScroll = () => {
    const el = scrollRef.current;
    if (!el) return;
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 48;
    pinnedRef.current = atBottom;
    setUnpinned(!atBottom);
  };
  const repin = () => {
    const el = scrollRef.current;
    pinnedRef.current = true;
    setUnpinned(false);
    if (el) el.scrollTop = el.scrollHeight;
  };

  // Keep the composer stable: a comfortable two-line minimum, capped growth, then
  // internal scrolling. Measuring in a layout effect avoids height flicker while typing.
  useLayoutEffect(() => {
    const el = inputRef.current;
    if (!el) return;
    el.style.height = `${COMPOSER_MIN_HEIGHT}px`;
    const next = Math.min(Math.max(el.scrollHeight, COMPOSER_MIN_HEIGHT), COMPOSER_MAX_HEIGHT);
    el.style.height = `${next}px`;
    el.style.overflowY = el.scrollHeight > COMPOSER_MAX_HEIGHT ? "auto" : "hidden";
  }, [draftText, open]);

  const submit = () => {
    if (!draftText.trim()) return;
    const text = draftText;
    if (busy) {
      // Queue-of-one while a turn runs — sends automatically when it finishes.
      queueFollowup(text);
      setDraftText("");
      return;
    }
    setDraftText("");
    void send(text).then((failedText) => {
      // A failed send returns the text — put it back so nothing is lost.
      if (failedText) {
        const current = useAppStore.getState().copilotDraftFor(useAppStore.getState().activeThreadId);
        if (!current) setDraftText(failedText);
      }
    });
  };

  const sendPrompt = (text: string) => {
    const prompt = text.trim();
    if (!prompt) return;
    if (busy) {
      queueFollowup(prompt);
      return;
    }
    void send(prompt);
  };

  const surfaceLabel = describeSurface(surface);
  const surfaceScope = scopeForSurface(surface, surfaceLabel);

  return (
    <div className="shrink-0 overflow-hidden" style={{ width: open ? DOCK_WIDTH : 0 }}>
      <aside
        className="flex h-full flex-col border-l border-line bg-panel"
        style={{ width: DOCK_WIDTH }}
        aria-label="Copilot"
      >
        {/* Header */}
        <div className="flex h-11 shrink-0 items-center gap-2 border-b border-line px-3">
          <Icon as={Sparkles} className="text-amber" />
          <span className="text-body font-semibold text-ink">Copilot</span>

          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                className="ml-1 flex items-center gap-1 rounded-chip px-1.5 py-0.5 font-mono text-meta text-ink-3 hover:bg-raise hover:text-ink"
                aria-label="Threads"
              >
                {activeThread(threads, activeThreadId) ?? "New chat"}
                <Icon as={ChevronDown} size="inline" mark />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" className="w-72">
              <DropdownMenuLabel className="flex items-center justify-between gap-2">
                <span>Threads</span>
                <button
                  type="button"
                  onClick={newThread}
                  className="grid size-6 place-items-center rounded-tap text-ink-3 hover:bg-raise hover:text-ink"
                  aria-label="New chat"
                >
                  <Icon as={Plus} size="inline" mark />
                </button>
              </DropdownMenuLabel>
              <div className="px-2 pb-2">
                <div className="flex items-center gap-1.5 rounded-card border border-line bg-field px-2 py-1.5">
                  <Icon as={Search} size="inline" mark className="shrink-0 text-ink-4" />
                  <input
                    value={threadQuery}
                    onChange={(e) => setThreadQuery(e.target.value)}
                    onKeyDown={(e) => e.stopPropagation()}
                    placeholder="Search threads"
                    className="min-w-0 flex-1 bg-transparent text-small text-ink outline-none placeholder:text-ink-4"
                  />
                </div>
              </div>
              <DropdownMenuSeparator />
              <ThreadMenuItems
                threads={threads}
                query={threadQuery}
                activeThreadId={activeThreadId}
                proposals={proposals}
                status={status}
                onOpenThread={openThread}
                onRenameThread={renameThread}
                onArchiveThread={archiveThread}
                onSetThreadPinned={setThreadPinned}
              />
            </DropdownMenuContent>
          </DropdownMenu>

          <div className="ml-auto flex items-center gap-0.5">
            <button
              type="button"
              onClick={newThread}
              aria-label="New chat"
              title="New chat"
              className="grid size-6 place-items-center rounded-tap text-ink-3 hover:bg-[rgb(var(--hover))] hover:text-ink"
            >
              <Icon as={Plus} />
            </button>
            <button
              type="button"
              onClick={() => setOpen(false)}
              aria-label="Close copilot"
              title="Close"
              className="grid size-6 place-items-center rounded-tap text-ink-3 hover:bg-[rgb(var(--hover))] hover:text-ink"
            >
              <Icon as={X} />
            </button>
          </div>
        </div>

        {/* Transcript */}
        <div
          ref={scrollRef}
          onScroll={onTranscriptScroll}
          className="relative min-h-0 flex-1 space-y-4 overflow-y-auto px-3 py-4"
        >
          {messages.length === 0 && !streaming ? (
            <div className="mt-6 flex flex-col items-center gap-3 px-4 text-center">
              {/* Empty-state illustration, not chrome — deliberately off the <Icon>
                  scale, so §5 rule 9's grep flags it on purpose. */}
              <Sparkles className="size-5 text-ink-4" strokeWidth={1.5} />
              <p className="text-body text-ink-3">
                Ask about your research — grounded in your Aladin data.
              </p>
              {surfaceLabel ? (
                <p className="font-mono text-meta text-ink-4">Looking at {surfaceLabel}</p>
              ) : null}
              <div className="mt-1 flex flex-col items-stretch gap-1.5">
                {suggestionsFor(surface).map((s) => (
                  <button
                    key={s}
                    type="button"
                    onClick={() => void send(s)}
                    className="rounded-chip border border-line px-3 py-1.5 text-left text-small text-ink-2 transition-colors hover:border-amber-line hover:text-ink"
                  >
                    {s}
                  </button>
                ))}
              </div>
            </div>
          ) : null}

          {messages.map((m) => (
            <MessageBubble key={m.id} message={m} onPrompt={sendPrompt} />
          ))}

          {status === "streaming" && streaming ? (
            <AssistantBubble content={streaming} citations={[]} streaming onPrompt={sendPrompt} />
          ) : null}

          {activeProposals.map((p) => (
            <ProposalCard
              key={p.actionId}
              proposal={p}
              onApprove={() => approveProposal(p.actionId)}
              onReject={() => rejectProposal(p.actionId)}
            />
          ))}

          {busy && toolTrail.length > 0 ? <ActivityTimeline trail={toolTrail} /> : null}

          {awaitingApproval ? (
            <p className="font-mono text-meta text-amber">waiting for your approval…</p>
          ) : thinking ? (
            <p className="animate-pulse font-mono text-meta text-ink-4">reasoning…</p>
          ) : status === "sending" ? (
            <p className="animate-pulse font-mono text-meta text-ink-4">thinking…</p>
          ) : null}

          <CopilotErrorBanner error={error} code={errorCode} onContinue={() => void send("continue")} />
        </div>

        <LatestTranscriptButton visible={unpinned && busy} onClick={repin} />

        {/* Composer */}
        <div className="shrink-0 border-t border-line p-2.5">
          <HealthWarningBanner message={healthWarning} onDismiss={() => setHealthWarning(null)} />
          <QueuedFollowupBanner text={queuedText} onClear={() => queueFollowup(null)} />
          <RealtimeStatusBanner state={realtimeState} />
          <div className="group rounded-card border border-line bg-field transition-colors focus-within:border-amber-line">
            {surfaceScope ? <ScopeChip scope={surfaceScope} onNewThread={newThread} /> : null}
            <div className="flex items-end gap-2 px-3 py-2.5">
              <textarea
                ref={inputRef}
                value={draftText}
                onChange={(e) => setDraftText(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && !e.shiftKey) {
                    e.preventDefault();
                    submit();
                  } else if (e.key === "Escape") {
                    e.preventDefault();
                    setOpen(false);
                  }
                }}
                rows={2}
                aria-label="Message Copilot"
                placeholder={composerPlaceholder(busy, surfaceLabel)}
                className="min-h-[44px] flex-1 resize-none bg-transparent py-0.5 text-body leading-relaxed text-ink outline-none placeholder:text-ink-4"
              />
              <div className="flex shrink-0 items-center gap-1">
                {busy ? (
                  <button
                    type="button"
                    onClick={stop}
                    aria-label="Stop the current turn"
                    title="Stop"
                    className="grid size-8 place-items-center rounded-chip border border-line bg-raise text-ink-2 transition-colors hover:border-against/50 hover:text-against"
                  >
                    <Icon as={Square} size="inline" mark className="fill-current" />
                  </button>
                ) : null}
                <button
                  type="button"
                  onClick={submit}
                  disabled={!draftText.trim()}
                  aria-label={busy ? "Queue message" : "Send"}
                  title={busy ? "Queue — sends when the turn finishes" : "Send"}
                  className={cn(
                    "grid size-8 place-items-center rounded-chip transition-all",
                    busy
                      ? "border border-line bg-raise text-ink-2 hover:border-amber-line hover:text-amber"
                      : "bg-amber text-primary-foreground disabled:opacity-30",
                  )}
                >
                  <Icon as={ArrowUp} mark />
                </button>
              </div>
            </div>
          </div>
        </div>
      </aside>
    </div>
  );
}

interface ScopeSummary {
  title: string;
  kind: string;
  icon: LucideIcon;
  rows: { label: string; value: string }[];
}

function ScopeChip({
  scope,
  onNewThread,
}: {
  scope: ScopeSummary;
  onNewThread: () => void;
}) {
  return (
    <div className="border-b border-line/60 px-2.5 py-1.5">
      <Popover>
        <PopoverTrigger asChild>
          <button
            type="button"
            className="flex max-w-full items-center gap-1.5 rounded-chip px-1.5 py-0.5 text-left transition-colors hover:bg-raise"
            aria-label={`Current scope: ${scope.title}`}
          >
            <Icon as={scope.icon} size="inline" mark className="shrink-0 text-amber" />
            <span className="shrink-0 font-mono text-meta text-ink-4">asking about</span>
            <span className="min-w-0 truncate font-mono text-meta text-ink-2">{scope.title}</span>
            <Icon as={ChevronDown} size="inline" mark className="shrink-0 text-ink-4" />
          </button>
        </PopoverTrigger>
        <PopoverContent className="w-72 p-3">
          <div className="flex items-start gap-2">
            <Icon as={scope.icon} mark className="mt-0.5 shrink-0 text-amber" />
            <div className="min-w-0 flex-1">
              <p className="truncate text-small font-semibold text-ink">{scope.title}</p>
              <p className="font-mono text-meta text-ink-4">{scope.kind}</p>
            </div>
          </div>
          {scope.rows.length > 0 ? (
            <dl className="mt-2 space-y-1 rounded-card border border-line bg-field p-2">
              {scope.rows.map((row) => (
                <div key={row.label} className="grid grid-cols-[64px_minmax(0,1fr)] gap-2 font-mono text-meta">
                  <dt className="text-ink-4">{row.label}</dt>
                  <dd className="truncate text-ink-2" title={row.value}>
                    {row.value}
                  </dd>
                </div>
              ))}
            </dl>
          ) : null}
          <button
            type="button"
            onClick={onNewThread}
            className="mt-2 inline-flex items-center gap-1.5 rounded-chip border border-line px-2.5 py-1 text-small text-ink-2 transition-colors hover:border-amber-line hover:text-ink"
          >
            <Icon as={Plus} size="inline" mark />
            New chat
          </button>
        </PopoverContent>
      </Popover>
    </div>
  );
}

export function RealtimeStatusBanner({ state }: { state: "connecting" | "open" | "closed" }) {
  if (state === "open") return null;
  return (
    <div className="mb-1.5 flex items-center gap-2 rounded-card border border-line bg-raise px-2.5 py-1.5">
      <span
        aria-hidden
        className={cn(
          "size-1.5 rounded-full",
          state === "connecting" ? "animate-pulse bg-amber" : "bg-against",
        )}
      />
      <p className="font-mono text-meta text-ink-3">
        {state === "connecting" ? "reconnecting stream…" : "stream offline — reconnecting"}
      </p>
    </div>
  );
}

export function LatestTranscriptButton({
  visible,
  onClick,
}: {
  visible: boolean;
  onClick: () => void;
}) {
  if (!visible) return null;
  return (
    <div className="pointer-events-none relative">
      <button
        type="button"
        onClick={onClick}
        className="pointer-events-auto absolute bottom-2 left-1/2 -translate-x-1/2 rounded-chip border border-line bg-raise px-2.5 py-1 font-mono text-meta text-ink-2 shadow-panel transition-colors hover:border-amber-line hover:text-ink"
      >
        ↓ latest
      </button>
    </div>
  );
}

export function CopilotErrorBanner({
  error,
  code,
  onContinue,
}: {
  error: string | null;
  code: string | null;
  onContinue: () => void;
}) {
  if (!error) return null;
  return (
    <div className="rounded-card border border-against/40 bg-against/10 px-3 py-2">
      <p className="text-small text-against">{error}</p>
      {code === "max_turns" ? (
        <button
          type="button"
          onClick={onContinue}
          className="mt-1.5 rounded-chip border border-line px-2.5 py-1 text-meta text-ink-2 transition-colors hover:border-amber-line hover:text-ink"
        >
          Continue where it left off
        </button>
      ) : null}
    </div>
  );
}

export function HealthWarningBanner({
  message,
  onDismiss,
}: {
  message: string | null;
  onDismiss: () => void;
}) {
  if (!message) return null;
  return (
    <div className="mb-1.5 flex items-start justify-between gap-2 rounded-card border border-amber-line bg-amber-soft/40 px-2.5 py-1.5">
      <p className="text-meta text-ink-2">{message}</p>
      <button
        type="button"
        onClick={onDismiss}
        aria-label="Dismiss warning"
        className="text-ink-4 hover:text-ink"
      >
        <Icon as={X} size="inline" mark />
      </button>
    </div>
  );
}

export function QueuedFollowupBanner({
  text,
  onClear,
}: {
  text: string | null;
  onClear: () => void;
}) {
  if (!text) return null;
  return (
    <div className="mb-1.5 flex items-center justify-between gap-2 rounded-card border border-line bg-raise px-2.5 py-1.5">
      <p className="truncate font-mono text-meta text-ink-3">
        queued — sends when the copilot finishes: “{text}”
      </p>
      <button
        type="button"
        onClick={onClear}
        aria-label="Remove queued message"
        className="text-ink-4 hover:text-ink"
      >
        <Icon as={X} size="inline" mark />
      </button>
    </div>
  );
}

function activeThread(
  threads: { id: string; title: string }[],
  activeId: string | null,
): string | null {
  if (!activeId) return null;
  const found = threads.find((t) => t.id === activeId);
  return found ? found.title || "Untitled" : null;
}

export function ThreadMenuItems({
  threads,
  query,
  activeThreadId,
  proposals,
  status,
  onOpenThread,
  onRenameThread,
  onArchiveThread,
  onSetThreadPinned,
}: {
  threads: CopilotThreadView[];
  query: string;
  activeThreadId: string | null;
  proposals: CopilotProposal[];
  status: "idle" | "sending" | "streaming";
  onOpenThread: (threadId: string) => void;
  onRenameThread: (threadId: string, title: string) => Promise<boolean>;
  onArchiveThread: (threadId: string) => Promise<boolean>;
  onSetThreadPinned: (threadId: string, pinned: boolean) => Promise<boolean>;
}) {
  const q = query.trim().toLowerCase();
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editingTitle, setEditingTitle] = useState("");
  const filtered = q
    ? threads.filter((t) => (t.title || "Untitled").toLowerCase().includes(q))
    : threads;

  if (threads.length === 0) {
    return <DropdownMenuItem disabled>No saved threads</DropdownMenuItem>;
  }
  if (filtered.length === 0) {
    return <DropdownMenuItem disabled>No matching threads</DropdownMenuItem>;
  }

  return (
    <div className="max-h-80 overflow-y-auto p-1">
      {filtered.map((thread) => {
        const active = thread.id === activeThreadId;
        const editing = thread.id === editingId;
        const pendingApprovals = proposals.filter(
          (p) =>
            p.threadId === thread.id &&
            (p.status === "pending" || p.status === "approving" || p.status === "rejecting"),
        ).length;
        const running = active && status !== "idle";
        const startRename = (event: MouseEvent) => {
          event.preventDefault();
          event.stopPropagation();
          setEditingId(thread.id);
          setEditingTitle(thread.title || "Untitled");
        };
        const submitRename = async () => {
          const ok = await onRenameThread(thread.id, editingTitle);
          if (ok) setEditingId(null);
        };
        return (
          <DropdownMenuItem
            key={thread.id}
            onSelect={(event) => {
              if (editing) {
                event.preventDefault();
                return;
              }
              void onOpenThread(thread.id);
            }}
            className={cn("flex items-start gap-2 rounded-card px-2 py-2", active && "bg-raise")}
          >
            <Icon
              as={MessageSquare}
              size="inline"
              className={cn("mt-0.5 shrink-0", active ? "text-amber" : "text-ink-4")}
            />
            {editing ? (
              <span className="flex min-w-0 flex-1 items-center gap-1.5">
                <input
                  autoFocus
                  value={editingTitle}
                  onChange={(event) => setEditingTitle(event.target.value)}
                  onClick={(event) => event.stopPropagation()}
                  onKeyDown={(event) => {
                    event.stopPropagation();
                    if (event.key === "Enter") {
                      event.preventDefault();
                      void submitRename();
                    }
                    if (event.key === "Escape") {
                      event.preventDefault();
                      setEditingId(null);
                    }
                  }}
                  className="min-w-0 flex-1 rounded-tap border border-line bg-field px-2 py-1 text-small text-ink outline-none focus:border-amber-line"
                />
                <button
                  type="button"
                  onClick={(event) => {
                    event.preventDefault();
                    event.stopPropagation();
                    void submitRename();
                  }}
                  className="grid size-6 place-items-center rounded-tap text-ink-3 hover:bg-raise hover:text-for"
                  aria-label="Save thread title"
                >
                  <Icon as={Check} size="inline" mark />
                </button>
                <button
                  type="button"
                  onClick={(event) => {
                    event.preventDefault();
                    event.stopPropagation();
                    setEditingId(null);
                  }}
                  className="grid size-6 place-items-center rounded-tap text-ink-3 hover:bg-raise hover:text-ink"
                  aria-label="Cancel rename"
                >
                  <Icon as={X} size="inline" mark />
                </button>
              </span>
            ) : (
              <>
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-small text-ink">{thread.title || "Untitled"}</span>
                  <span className="mt-0.5 flex items-center gap-1.5 font-mono text-meta text-ink-4">
                    {thread.pinned ? <StatusPill tone="ink" label="pinned" /> : null}
                    {running ? <StatusPill tone="amber" label="running" /> : null}
                    {pendingApprovals > 0 ? <StatusPill tone="against" label="approval" /> : null}
                    {!thread.pinned && !running && pendingApprovals === 0 ? <span>{formatThreadTime(thread.updatedAt)}</span> : null}
                  </span>
                </span>
                <span className="flex shrink-0 items-center gap-0.5">
                  <button
                    type="button"
                    onClick={(event) => {
                      event.preventDefault();
                      event.stopPropagation();
                      void onSetThreadPinned(thread.id, !thread.pinned);
                    }}
                    className={cn(
                      "grid size-6 place-items-center rounded-tap hover:bg-field",
                      thread.pinned ? "text-amber" : "text-ink-4 hover:text-ink",
                    )}
                    aria-label={thread.pinned ? "Unpin thread" : "Pin thread"}
                  >
                    <Icon as={Pin} size="inline" mark className={cn(thread.pinned && "fill-current")} />
                  </button>
                  <button
                    type="button"
                    onClick={startRename}
                    className="grid size-6 place-items-center rounded-tap text-ink-4 hover:bg-field hover:text-ink"
                    aria-label="Rename thread"
                  >
                    <Icon as={Pencil} size="inline" mark />
                  </button>
                  <button
                    type="button"
                    onClick={(event) => {
                      event.preventDefault();
                      event.stopPropagation();
                      void onArchiveThread(thread.id);
                    }}
                    className="grid size-6 place-items-center rounded-tap text-ink-4 hover:bg-field hover:text-against"
                    aria-label="Archive thread"
                  >
                    <Icon as={Archive} size="inline" mark />
                  </button>
                </span>
              </>
            )}
          </DropdownMenuItem>
        );
      })}
    </div>
  );
}

function StatusPill({ tone, label }: { tone: "amber" | "against" | "ink"; label: string }) {
  return (
    <span
      className={cn(
        "rounded-chip px-1.5 py-0.5",
        tone === "amber" && "bg-amber-soft text-amber",
        tone === "against" && "bg-against/10 text-against",
        tone === "ink" && "bg-field text-ink-3",
      )}
    >
      {label}
    </span>
  );
}

function formatThreadTime(raw: string): string {
  const ts = Date.parse(raw);
  if (!Number.isFinite(ts)) return "saved";
  const diff = Date.now() - ts;
  if (diff < 60_000) return "just now";
  if (diff < 60 * 60_000) return `${Math.max(1, Math.floor(diff / 60_000))}m ago`;
  if (diff < 24 * 60 * 60_000) return `${Math.max(1, Math.floor(diff / (60 * 60_000)))}h ago`;
  return `${Math.max(1, Math.floor(diff / (24 * 60 * 60_000)))}d ago`;
}

/**
 * Compact live activity: consecutive runs of the same tool collapse to one row
 * with a count; the most recent few rows stay visible while the turn runs.
 */
export function ActivityTimeline({ trail }: { trail: CopilotToolRun[] }) {
  const groups: {
    label: string;
    count: number;
    status: CopilotToolRun["status"];
    inputSummary?: string;
    resultSummary?: string;
  }[] = [];
  for (const run of trail) {
    const last = groups[groups.length - 1];
    if (last && last.label === run.label) {
      last.count += 1;
      // A group's status is its worst/latest interesting state.
      last.status = run.status === "running" ? "running" : run.status === "error" ? "error" : last.status;
      last.inputSummary = run.inputSummary ?? last.inputSummary;
      last.resultSummary = run.resultSummary ?? last.resultSummary;
    } else {
      groups.push({
        label: run.label,
        count: 1,
        status: run.status,
        inputSummary: run.inputSummary,
        resultSummary: run.resultSummary,
      });
    }
  }
  const visible = groups.slice(-6);
  return (
    <div className="rounded-card border border-line bg-field px-2.5 py-2">
      <div className="mb-1.5 flex items-center justify-between gap-2">
        <span className="font-mono text-meta text-ink-4">activity</span>
        {groups.length > visible.length ? (
          <span className="font-mono text-meta text-ink-4">+{groups.length - visible.length}</span>
        ) : null}
      </div>
      <div className="space-y-1">
        {visible.map((g, i) => {
          const hasDetails = Boolean(g.inputSummary || g.resultSummary);
          const row = (
            <div className="flex items-center gap-2 text-small text-ink-2">
              <span
                className={cn(
                  "size-1.5 shrink-0 rounded-full",
                  g.status === "running" && "animate-pulse bg-amber",
                  g.status === "ok" && "bg-for",
                  g.status === "error" && "bg-against",
                )}
              />
              <span className="min-w-0 flex-1 truncate">{g.label}</span>
              {g.count > 1 ? <span className="font-mono text-meta text-ink-4">x{g.count}</span> : null}
            </div>
          );
          return hasDetails ? (
            <details key={`${g.label}-${i}`} className="rounded-tap open:bg-raise/40">
              <summary className="cursor-default list-none">{row}</summary>
              <div className="mt-1 space-y-0.5 border-l border-line pl-3 font-mono text-meta text-ink-4">
                {g.inputSummary ? <p className="truncate">in: {g.inputSummary}</p> : null}
                {g.resultSummary ? <p className="truncate">out: {g.resultSummary}</p> : null}
              </div>
            </details>
          ) : (
            <div key={`${g.label}-${i}`}>{row}</div>
          );
        })}
      </div>
    </div>
  );
}

export function ProposalCard({
  proposal,
  onApprove,
  onReject,
}: {
  proposal: CopilotProposal;
  onApprove: () => void;
  onReject: () => void;
}) {
  // Settled/expired proposals collapse to a one-line note.
  if (
    proposal.status === "approved" ||
    proposal.status === "rejected" ||
    proposal.status === "expired"
  ) {
    const approved = proposal.status === "approved";
    return (
      <p className="flex items-center gap-1.5 font-mono text-meta text-ink-4">
        <Icon as={Check} size="inline" mark className={cn(approved ? "text-for" : "text-ink-4")} />
        {proposal.message || (approved ? "Applied." : proposal.status === "expired" ? "That approval expired." : "Dismissed.")}
      </p>
    );
  }

  // pending: actionable. approving/rejecting: the POST is in flight — buttons lock.
  const inFlight = proposal.status !== "pending";
  return (
    <div className="rounded-card border border-amber-line bg-amber-soft/40 p-3">
      <div className="flex items-start gap-2">
        <Icon as={AlertTriangle} size="inline" mark className="mt-0.5 shrink-0 text-amber" />
        <div className="min-w-0 flex-1">
          <p className="text-small text-ink">{proposal.summary}</p>
          <p className="mt-0.5 font-mono text-meta text-ink-4">
            {proposal.status === "approving"
              ? "Approving…"
              : proposal.status === "rejecting"
                ? "Dismissing…"
                : "Needs your approval — the copilot is waiting"}
          </p>
        </div>
      </div>
      <div className="mt-2.5 flex gap-2">
        <button
          type="button"
          disabled={inFlight}
          onClick={onApprove}
          className="flex-1 rounded-chip bg-amber py-1.5 text-small font-semibold text-primary-foreground transition-opacity disabled:opacity-50"
        >
          Approve
        </button>
        <button
          type="button"
          disabled={inFlight}
          onClick={onReject}
          className="flex-1 rounded-chip border border-line py-1.5 text-small text-ink-2 transition-colors hover:text-ink disabled:opacity-50"
        >
          Reject
        </button>
      </div>
      {proposal.message ? (
        <p className="mt-1.5 font-mono text-meta text-against">{proposal.message}</p>
      ) : null}
    </div>
  );
}

export function suggestionsFor(surface: CopilotSurface): string[] {
  switch (surface.kind) {
    case "ticker": {
      const s = surface.symbol ?? "this ticker";
      return [`What's my thesis on ${s}?`, `How does ${s} look technically?`, `Any recent notes on ${s}?`];
    }
    case "entity":
      return ["What do I know about this?", "What's it connected to?"];
    case "artifact":
    case "page":
    case "shard": {
      if (surface.artifactKind === "app") {
        return [
          "Summarize this shard",
          "What would make this clearer?",
          "Polish the interaction design",
        ];
      }
      if (surface.artifactKind === "file") {
        return [
          "Summarize this source",
          "What are the key claims here?",
          "Extract the useful citations",
        ];
      }
      return [
        "Summarize what I'm looking at",
        "What are the key claims here?",
        "Turn this into an interactive shard",
      ];
    }
    case "markets":
      return ["What am I watching?", "Anything notable in my watchlist?"];
    default:
      return [
        "What have I been researching?",
        "Summarize my recent insights",
        "Build a shard about a ticker I follow",
      ];
  }
}

/** Placeholder teaches the current mode: normal ask, surface-scoped ask, or queueing. */
export function composerPlaceholder(busy: boolean, surfaceLabel: string | null): string {
  if (busy) return "Type a follow-up — sends when this turn finishes…";
  if (surfaceLabel) return `Ask about ${surfaceLabel}…`;
  return "Ask the copilot…";
}

export function describeSurface(surface: CopilotSurface): string | null {
  if (surface.kind === "ticker" && surface.symbol) return surface.symbol;
  if (surface.kind === "entity") return surface.label ?? "this entity";
  if (surface.kind === "artifact" || surface.kind === "page" || surface.kind === "shard") {
    if (surface.label) return surface.label;
    return `this ${surfaceKindLabel(surface.artifactKind)}`;
  }
  if (surface.kind === "markets") return "Markets";
  return null;
}

export function scopeForSurface(surface: CopilotSurface, label: string | null): ScopeSummary | null {
  if (!label) return null;
  switch (surface.kind) {
    case "ticker": {
      const symbol = surface.symbol?.toUpperCase() ?? label;
      return {
        title: symbol,
        kind: "ticker",
        icon: BarChart3,
        rows: [{ label: "symbol", value: symbol }],
      };
    }
    case "entity":
      return {
        title: label,
        kind: "entity",
        icon: UserRound,
        rows: surface.id ? [{ label: "id", value: surface.id }] : [],
      };
    case "artifact":
    case "page":
    case "shard": {
      const kind = surfaceKindLabel(surface.artifactKind);
      return {
        title: label,
        kind,
        icon: surface.artifactKind ? ARTIFACT_ICONS[surface.artifactKind] : FileText,
        rows: [
          ...(surface.id ? [{ label: "id", value: surface.id }] : []),
          ...(surface.artifactKind ? [{ label: "type", value: surface.artifactKind }] : []),
        ],
      };
    }
    case "markets":
      return {
        title: "Markets",
        kind: "market surface",
        icon: BarChart3,
        rows: [{ label: "scope", value: "watchlists" }],
      };
    default:
      return {
        title: label,
        kind: surface.kind || "surface",
        icon: Focus,
        rows: [],
      };
  }
}

export function surfaceKindLabel(kind: CopilotSurface["artifactKind"]): string {
  switch (kind) {
    case "app":
      return "shard";
    case "file":
      return "source";
    case "link":
      return "link";
    case "voice":
      return "voice note";
    case "note":
      return "page";
    default:
      return "item";
  }
}

function MessageBubble({
  message,
  onPrompt,
}: {
  message: CopilotMessageView;
  onPrompt: (prompt: string) => void;
}) {
  if (message.role === "user") {
    return (
      <div className="flex justify-end">
        <div className="max-w-[85%] whitespace-pre-wrap rounded-card bg-raise px-3 py-2 text-body text-ink">
          {message.content}
        </div>
      </div>
    );
  }
  return (
    <AssistantBubble
      content={message.content}
      citations={message.citations}
      meta={message.meta}
      onPrompt={onPrompt}
    />
  );
}

/**
 * turnDigest compresses a turn's activity + cost into one muted footer line, e.g.
 * "searched ·2 · wrote shard code ·3 · built ✓ — $0.14 · 23 steps". Empty for
 * turns with no meta (legacy rows, plain Q&A with no tools and no usage).
 */
export function turnDigest(meta: CopilotMessageMeta | undefined): string {
  if (!meta) return "";
  const parts: string[] = [];
  if (meta.activity && meta.activity.length > 0) {
    const groups: { label: string; count: number; failed: boolean }[] = [];
    for (const item of meta.activity) {
      const label = toolDigestLabel(item.name);
      const last = groups[groups.length - 1];
      if (last && last.label === label) {
        last.count += 1;
        last.failed = last.failed || !item.ok;
      } else {
        groups.push({ label, count: 1, failed: !item.ok });
      }
    }
    parts.push(
      groups
        .map((g) => `${g.label}${g.count > 1 ? ` ×${g.count}` : ""}${g.failed ? " ✗" : ""}`)
        .join(" · "),
    );
  }
  const tail: string[] = [];
  if (meta.costUsd && meta.costUsd > 0) tail.push(`$${meta.costUsd.toFixed(2)}`);
  if (meta.numTurns && meta.numTurns > 1) tail.push(`${meta.numTurns} steps`);
  if (tail.length > 0) parts.push(tail.join(" · "));
  return parts.join(" — ");
}

function toolDigestLabel(name: string): string {
  switch (name) {
    case "search":
      return "searched workspace";
    case "get_entity":
      return "read entity";
    case "get_insights":
      return "read insights";
    case "list_artifacts":
      return "listed artifacts";
    case "get_artifact":
      return "read artifact";
    case "get_page":
    case "list_pages":
      return "read page";
    case "get_watchlist":
      return "checked watchlist";
    case "get_browser_tree":
    case "list_folders":
      return "browsed workspace";
    case "search_pages":
      return "searched pages";
    case "get_bars":
      return "read price history";
    case "get_quote":
      return "fetched quote";
    case "get_news":
      return "read news";
    case "get_movers":
      return "scanned movers";
    case "get_most_actives":
      return "checked most-actives";
    case "get_account":
      return "read account";
    case "get_positions":
      return "read positions";
    case "create_alert":
      return "set alert";
    case "list_alerts":
      return "checked alerts";
    case "delete_alert":
      return "removed alert";
    case "create_app":
      return "created shard";
    case "list_dir":
    case "read_file":
      return "read shard files";
    case "write_file":
    case "edit_file":
      return "wrote shard code";
    case "install_lib":
      return "added dependency";
    case "build_app":
      return "built shard";
    case "delete_file":
      return "deleted shard file";
    case "publish_app":
      return "published shard";
    case "preview_open":
    case "preview_navigate":
    case "preview_snapshot":
    case "preview_screenshot":
    case "preview_eval":
    case "preview_click":
    case "preview_console":
    case "preview_close":
    case "preview_restart":
      return "previewed shard";
    case "create_page":
      return "created page";
    case "insert_blocks":
    case "update_block":
    case "update_page":
      return "edited page";
    case "delete_block":
      return "removed block";
    case "add_to_watchlist":
      return "updated watchlist";
    case "list_watchlists":
      return "listed watchlists";
    case "create_watchlist":
      return "created watchlist";
    case "draw_edge":
      return "linked entities";
    default:
      return name.replaceAll("_", " ");
  }
}

function AssistantBubble({
  content,
  citations,
  meta,
  streaming,
  onPrompt,
}: {
  content: string;
  citations: CopilotCitation[];
  meta?: CopilotMessageMeta;
  streaming?: boolean;
  onPrompt: (prompt: string) => void;
}) {
  const navCitation = useCitationNav();
  // Dedupe citations by kind+id for display.
  const seen = new Set<string>();
  const unique = citations.filter((c) => {
    const key = `${c.kind}|${c.id}`;
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
  return (
    <div className="flex flex-col gap-1.5">
      <div>
        <CopilotMarkdown text={content} onPrompt={onPrompt} />
        {streaming ? <StreamCaret /> : null}
      </div>
      {unique.length > 0 ? (
        <div className="flex flex-wrap gap-1.5">
          {unique.map((c) => (
            <button
              key={`${c.kind}|${c.id}`}
              type="button"
              onClick={() => navCitation(c)}
              className="max-w-[180px] truncate rounded-chip border border-line px-2 py-0.5 font-mono text-meta text-ink-3 transition-colors hover:border-amber-line hover:text-ink"
              title={`${c.kind}: ${c.title}`}
            >
              {c.title}
            </button>
          ))}
        </div>
      ) : null}
      {!streaming && turnDigest(meta) ? (
        <p className="font-mono text-meta text-ink-4">{turnDigest(meta)}</p>
      ) : null}
    </div>
  );
}
