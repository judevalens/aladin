import { ArrowUp, ChevronDown, LoaderCircle, Plus, Search, Sparkles, Square, X } from "lucide-react";
import { Icon } from "@/components/ui/icon";
import { useEffect, useLayoutEffect, useRef, useState } from "react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useAppStore } from "@/app/state/store";
import { useCopilot } from "@/modules/copilot/hooks/use-copilot";
import {
  EffortSwitcher,
  ModelSwitcher,
  ScopeChip,
} from "@/modules/copilot/ui/copilot-composer-controls";
import {
  CopilotErrorBanner,
  EmptyTranscriptState,
  HealthWarningBanner,
  LatestTranscriptButton,
  QueuedFollowupBanner,
  RealtimeStatusBanner,
} from "@/modules/copilot/ui/copilot-banners";
import { activeThread, ThreadMenuItems } from "@/modules/copilot/ui/copilot-thread-menu";
import {
  ActivityTimeline,
  AssistantBubble,
  MessageBubble,
  ProposalCard,
} from "@/modules/copilot/ui/copilot-transcript";
import {
  composerPlaceholder,
  describeSurface,
  scopeForSurface,
} from "@/modules/copilot/ui/copilot-surface";
import { cn } from "@/lib/utils";

const DOCK_WIDTH = 384;
const COMPOSER_MIN_HEIGHT = 44;
const COMPOSER_MAX_HEIGHT = 144;

/**
 * The Copilot dock — Aladin's default agentic AI surface. Mounted once in the workspace shell
 * so the conversation survives navigation; collapsed to zero width (not unmounted) while closed.
 * Surface-aware: every turn carries what the user is looking at.
 *
 * This file is the shell: the store wiring, the scroll/composer behaviour, and the layout that
 * arranges the pieces. The pieces themselves live next door — `copilot-transcript` renders turns,
 * `copilot-thread-menu` the thread dropdown, `copilot-banners` the transient notices,
 * `copilot-composer-controls` the model/effort/scope chrome, and `copilot-surface` turns the
 * current surface into copy.
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
    modelOptions,
    activeModel,
    defaultModel,
    setSelectedModel,
    effortOptions,
    activeEffort,
    defaultEffort,
    setSelectedEffort,
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
  }, [messages, streaming, activeTool, toolTrail, proposals, open]);
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
    <div className="shrink-0 overflow-hidden" style={{ width: open ? DOCK_WIDTH : 0, maxWidth: "100vw" }}>
      <aside
        className="flex h-full flex-col border-l border-line bg-panel"
        style={{ width: DOCK_WIDTH, maxWidth: "100vw" }}
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
                className="ml-1 flex max-w-[118px] items-center gap-1 rounded-chip px-1.5 py-0.5 font-mono text-meta text-ink-3 hover:bg-raise hover:text-ink"
                aria-label="Threads"
              >
                <span className="min-w-0 truncate">{activeThread(threads, activeThreadId) ?? "New chat"}</span>
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
              className="grid size-6 place-items-center rounded-tap text-ink-3 hover:bg-hover hover:text-ink"
            >
              <Icon as={Plus} />
            </button>
            <button
              type="button"
              onClick={() => setOpen(false)}
              aria-label="Close copilot"
              title="Close"
              className="grid size-6 place-items-center rounded-tap text-ink-3 hover:bg-hover hover:text-ink"
            >
              <Icon as={X} />
            </button>
          </div>
        </div>

        {/* Transcript */}
        <div
          ref={scrollRef}
          onScroll={onTranscriptScroll}
          role="log"
          aria-label="Copilot transcript"
          aria-live="polite"
          aria-busy={busy}
          className="relative min-h-0 flex-1 space-y-4 overflow-y-auto px-3 py-4"
        >
          {messages.length === 0 && !streaming ? (
            <EmptyTranscriptState
              surface={surface}
              surfaceLabel={surfaceLabel}
              onPrompt={(prompt) => void send(prompt)}
            />
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

          <CopilotErrorBanner error={error} code={errorCode} onContinue={() => void send("continue")} />
        </div>

        <LatestTranscriptButton visible={unpinned && busy} onClick={repin} />

        {busy || awaitingApproval ? (
          <div
            role="status"
            aria-label="Copilot progress"
            className={cn(
              "flex min-h-7 shrink-0 items-center gap-1.5 px-3 pb-2 font-mono text-meta",
              awaitingApproval ? "text-amber" : "text-ink-3",
            )}
          >
            {!awaitingApproval ? <Icon as={LoaderCircle} size="inline" mark className="animate-spin motion-reduce:animate-none" /> : null}
            <span>{awaitingApproval ? "waiting for your approval…" : thinking ? "reasoning…" : status === "sending" ? "thinking…" : "working…"}</span>
          </div>
        ) : null}

        {/* Composer */}
        <div className="shrink-0 border-t border-line p-2.5">
          <HealthWarningBanner message={healthWarning} onDismiss={() => setHealthWarning(null)} />
          <QueuedFollowupBanner text={queuedText} onClear={() => queueFollowup(null)} />
          <RealtimeStatusBanner state={realtimeState} />
          <div className="group rounded-card border border-line bg-field transition-colors focus-within:border-amber-line">
            {surfaceScope ? <ScopeChip scope={surfaceScope} onNewThread={newThread} /> : null}
            <div className="px-3 pt-2.5">
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
                className="min-h-[44px] w-full resize-none bg-transparent py-0.5 text-body leading-relaxed text-ink outline-none placeholder:text-ink-4"
              />
            </div>
            <div className="flex items-center justify-end gap-1.5 px-2.5 pb-2 pt-1">
              <ModelSwitcher
                models={modelOptions}
                activeModel={activeModel}
                defaultModel={defaultModel}
                onSelect={setSelectedModel}
              />
              <EffortSwitcher
                efforts={effortOptions}
                activeEffort={activeEffort}
                defaultEffort={defaultEffort}
                onSelect={setSelectedEffort}
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
