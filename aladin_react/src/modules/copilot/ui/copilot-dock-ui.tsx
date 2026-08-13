import { AlertTriangle, ArrowUp, Check, ChevronDown, Plus, Sparkles, Square, X } from "lucide-react";
import { Icon } from "@/components/ui/icon";
import { useEffect, useRef, useState } from "react";
import type { CopilotSurface } from "@/repos/copilot/copilot-repo";
import type { CopilotProposal, CopilotToolRun } from "@/app/state/copilot-slice";
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
    approveProposal,
    rejectProposal,
    loadThreads,
    openThread,
    newThread,
    fetchHealthWarning,
    queuedText,
  } = useCopilot();

  const [input, setInput] = useState("");
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

  // Auto-grow the textarea with its content (reset after send), capped by max-h.
  const autogrow = () => {
    const el = inputRef.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = `${Math.min(el.scrollHeight, 160)}px`;
  };
  const resetComposerHeight = () => {
    const el = inputRef.current;
    if (el) el.style.height = "auto";
  };

  const submit = () => {
    if (!input.trim()) return;
    const text = input;
    if (busy) {
      // Queue-of-one while a turn runs — sends automatically when it finishes.
      useAppStore.getState().queueCopilotText(text);
      setInput("");
      resetComposerHeight();
      return;
    }
    setInput("");
    resetComposerHeight();
    void send(text).then((failedText) => {
      // A failed send returns the text — put it back so nothing is lost.
      if (failedText) setInput((current) => current || failedText);
    });
  };

  const surfaceLabel = describeSurface(surface);

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
            <DropdownMenuContent align="start" className="max-h-80 w-64 overflow-y-auto">
              <DropdownMenuLabel>Threads</DropdownMenuLabel>
              <DropdownMenuSeparator />
              {threads.length === 0 ? (
                <DropdownMenuItem disabled>No saved threads</DropdownMenuItem>
              ) : (
                threads.map((t) => (
                  <DropdownMenuItem key={t.id} onClick={() => void openThread(t.id)}>
                    <span className="truncate">{t.title || "Untitled"}</span>
                  </DropdownMenuItem>
                ))
              )}
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
            <MessageBubble key={m.id} message={m} />
          ))}

          {status === "streaming" && streaming ? (
            <AssistantBubble content={streaming} citations={[]} streaming />
          ) : null}

          {activeProposals.map((p) => (
            <ProposalCard
              key={p.actionId}
              proposal={p}
              onApprove={() => approveProposal(p.actionId)}
              onReject={() => rejectProposal(p.actionId)}
            />
          ))}

          {busy && toolTrail.length > 0 ? <ToolTrail trail={toolTrail} /> : null}

          {awaitingApproval ? (
            <p className="font-mono text-meta text-amber">waiting for your approval…</p>
          ) : activeTool ? (
            <p className="animate-pulse font-mono text-meta text-ink-4">{activeTool}…</p>
          ) : thinking ? (
            <p className="animate-pulse font-mono text-meta text-ink-4">reasoning…</p>
          ) : status === "sending" ? (
            <p className="animate-pulse font-mono text-meta text-ink-4">thinking…</p>
          ) : null}

          {error ? (
            <div className="rounded-card border border-against/40 bg-against/10 px-3 py-2">
              <p className="text-small text-against">{error}</p>
              {errorCode === "max_turns" ? (
                <button
                  type="button"
                  onClick={() => void send("continue")}
                  className="mt-1.5 rounded-chip border border-line px-2.5 py-1 text-meta text-ink-2 transition-colors hover:border-amber-line hover:text-ink"
                >
                  Continue where it left off
                </button>
              ) : null}
            </div>
          ) : null}
        </div>

        {unpinned && busy ? (
          <div className="pointer-events-none relative">
            <button
              type="button"
              onClick={repin}
              className="pointer-events-auto absolute bottom-2 left-1/2 -translate-x-1/2 rounded-chip border border-line bg-raise px-2.5 py-1 font-mono text-meta text-ink-2 shadow-panel transition-colors hover:border-amber-line hover:text-ink"
            >
              ↓ latest
            </button>
          </div>
        ) : null}

        {/* Composer */}
        <div className="shrink-0 border-t border-line p-2.5">
          {healthWarning ? (
            <div className="mb-1.5 flex items-start justify-between gap-2 rounded-card border border-amber-line bg-amber-soft/40 px-2.5 py-1.5">
              <p className="text-meta text-ink-2">{healthWarning}</p>
              <button
                type="button"
                onClick={() => setHealthWarning(null)}
                aria-label="Dismiss warning"
                className="text-ink-4 hover:text-ink"
              >
                <Icon as={X} size="inline" mark />
              </button>
            </div>
          ) : null}
          {queuedText ? (
            <div className="mb-1.5 flex items-center justify-between gap-2 rounded-card border border-line bg-raise px-2.5 py-1.5">
              <p className="truncate font-mono text-meta text-ink-3">
                queued — sends when the copilot finishes: “{queuedText}”
              </p>
              <button
                type="button"
                onClick={() => useAppStore.getState().queueCopilotText(null)}
                aria-label="Remove queued message"
                className="text-ink-4 hover:text-ink"
              >
                <Icon as={X} size="inline" mark />
              </button>
            </div>
          ) : null}
          <div className="group rounded-card border border-line bg-field transition-colors focus-within:border-amber-line">
            {surfaceLabel ? (
              <div className="flex items-center gap-1.5 border-b border-line/60 px-3 pb-1 pt-1.5">
                <span className="size-1 shrink-0 rounded-full bg-amber" />
                <span className="truncate font-mono text-meta text-ink-4">
                  asking about {surfaceLabel}
                </span>
              </div>
            ) : null}
            <div className="flex items-end gap-1.5 px-3 py-2.5">
              <textarea
                ref={inputRef}
                value={input}
                onChange={(e) => {
                  setInput(e.target.value);
                  autogrow();
                }}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && !e.shiftKey) {
                    e.preventDefault();
                    submit();
                  } else if (e.key === "Escape") {
                    e.preventDefault();
                    setOpen(false);
                  }
                }}
                rows={1}
                placeholder={composerPlaceholder(busy, surfaceLabel)}
                className="max-h-40 min-h-[22px] flex-1 resize-none bg-transparent text-body leading-relaxed text-ink outline-none placeholder:text-ink-4"
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
                  disabled={!input.trim()}
                  aria-label={busy ? "Queue message" : "Send"}
                  title={busy ? "Queue — sends when the turn finishes" : "Send"}
                  className={cn("grid size-8 place-items-center rounded-chip transition-all",
                    busy
                      ? "border border-line bg-raise text-ink-2 hover:border-amber-line hover:text-amber"
                      : "bg-amber text-primary-foreground disabled:opacity-30",
                  )}
                >
                  <Icon as={ArrowUp} mark />
                </button>
              </div>
            </div>
            {/* Keyboard hint — only while composing, so it never adds idle noise. */}
            <div className="hidden items-center justify-end gap-2 px-3 pb-1.5 group-focus-within:flex">
              <span className="font-mono text-meta text-ink-4">
                {busy ? "⏎ queue" : "⏎ send"} · ⇧⏎ newline · esc close
              </span>
            </div>
          </div>
        </div>
      </aside>
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

/**
 * Compact per-turn activity: consecutive runs of the same tool collapse to one chip
 * with a count; a colored dot marks the outcome (running pulses, ok/for, error/against).
 */
function ToolTrail({ trail }: { trail: CopilotToolRun[] }) {
  const groups: { label: string; count: number; status: CopilotToolRun["status"] }[] = [];
  for (const run of trail) {
    const last = groups[groups.length - 1];
    if (last && last.label === run.label) {
      last.count += 1;
      // A group's status is its worst/latest interesting state.
      last.status = run.status === "running" ? "running" : run.status === "error" ? "error" : last.status;
    } else {
      groups.push({ label: run.label, count: 1, status: run.status });
    }
  }
  return (
    <div className="flex flex-wrap gap-1.5">
      {groups.map((g, i) => (
        <span
          key={`${g.label}-${i}`}
          className="flex items-center gap-1.5 rounded-chip border border-line px-2 py-0.5 font-mono text-meta text-ink-3"
        >
          <span
            className={cn("size-1.5 rounded-full",
              g.status === "running" && "animate-pulse bg-amber",
              g.status === "ok" && "bg-for",
              g.status === "error" && "bg-against",
            )}
          />
          {g.label}
          {g.count > 1 ? ` ×${g.count}` : ""}
        </span>
      ))}
    </div>
  );
}

function ProposalCard({
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

function suggestionsFor(surface: CopilotSurface): string[] {
  switch (surface.kind) {
    case "ticker": {
      const s = surface.symbol ?? "this ticker";
      return [`What's my thesis on ${s}?`, `How does ${s} look technically?`, `Any recent notes on ${s}?`];
    }
    case "entity":
      return ["What do I know about this?", "What's it connected to?"];
    case "artifact":
    case "page":
    case "shard":
      return [
        "Summarize what I'm looking at",
        "What are the key claims here?",
        "Turn this into an interactive shard",
      ];
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
function composerPlaceholder(busy: boolean, surfaceLabel: string | null): string {
  if (busy) return "Type a follow-up — sends when this turn finishes…";
  if (surfaceLabel) return `Ask about ${surfaceLabel}…`;
  return "Ask the copilot…";
}

function describeSurface(surface: { kind: string; symbol?: string; label?: string }): string | null {
  if (surface.kind === "ticker" && surface.symbol) return surface.symbol;
  if (surface.kind === "entity") return surface.label ?? "this entity";
  if (surface.kind === "artifact" || surface.kind === "page" || surface.kind === "shard")
    return "this item";
  if (surface.kind === "markets") return "Markets";
  return null;
}

function MessageBubble({ message }: { message: CopilotMessageView }) {
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
    <AssistantBubble content={message.content} citations={message.citations} meta={message.meta} />
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
      const last = groups[groups.length - 1];
      if (last && last.label === item.name) {
        last.count += 1;
        last.failed = last.failed || !item.ok;
      } else {
        groups.push({ label: item.name, count: 1, failed: !item.ok });
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

function AssistantBubble({
  content,
  citations,
  meta,
  streaming,
}: {
  content: string;
  citations: CopilotCitation[];
  meta?: CopilotMessageMeta;
  streaming?: boolean;
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
        <CopilotMarkdown text={content} />
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
