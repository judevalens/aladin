import { ArrowUp, ChevronDown, Plus, Sparkles, X } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useCopilot } from "@/modules/copilot/hooks/use-copilot";
import { useCitationNav } from "@/modules/copilot/hooks/use-citation-nav";
import type { CopilotCitation, CopilotMessageView } from "@/app/state/copilot-slice";
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
    error,
    surface,
    send,
    loadThreads,
    openThread,
    newThread,
  } = useCopilot();

  const [input, setInput] = useState("");
  const scrollRef = useRef<HTMLDivElement>(null);
  const busy = status === "sending" || status === "streaming";

  // Load the thread list when the dock first opens.
  useEffect(() => {
    if (open) void loadThreads();
  }, [open, loadThreads]);

  // Keep the transcript pinned to the latest turn / streamed token.
  useEffect(() => {
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [messages, streaming, activeTool, open]);

  const submit = () => {
    if (!input.trim() || busy) return;
    void send(input);
    setInput("");
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
          <Sparkles className="size-4 text-amber" strokeWidth={1.75} />
          <span className="font-display text-sm font-semibold text-ink">Copilot</span>

          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                className="ml-1 flex items-center gap-1 rounded-chip px-1.5 py-0.5 font-mono text-[10px] text-ink-3 hover:bg-raise hover:text-ink"
                aria-label="Threads"
              >
                {activeThread(threads, activeThreadId) ?? "New chat"}
                <ChevronDown className="size-3" strokeWidth={2} />
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
              className="grid size-6 place-items-center rounded text-ink-3 hover:bg-[rgb(var(--hover))] hover:text-ink"
            >
              <Plus className="size-4" strokeWidth={1.75} />
            </button>
            <button
              type="button"
              onClick={() => setOpen(false)}
              aria-label="Close copilot"
              title="Close"
              className="grid size-6 place-items-center rounded text-ink-3 hover:bg-[rgb(var(--hover))] hover:text-ink"
            >
              <X className="size-4" strokeWidth={1.75} />
            </button>
          </div>
        </div>

        {/* Transcript */}
        <div ref={scrollRef} className="min-h-0 flex-1 space-y-4 overflow-y-auto px-3 py-4">
          {messages.length === 0 && !streaming ? (
            <div className="mt-6 flex flex-col items-center gap-2 px-4 text-center">
              <Sparkles className="size-5 text-ink-4" strokeWidth={1.5} />
              <p className="text-[13px] text-ink-3">
                Ask about your research — grounded in your Aladin data.
              </p>
              {surfaceLabel ? (
                <p className="font-mono text-[10px] text-ink-4">Looking at {surfaceLabel}</p>
              ) : null}
            </div>
          ) : null}

          {messages.map((m) => (
            <MessageBubble key={m.id} message={m} />
          ))}

          {status === "streaming" && streaming ? (
            <AssistantBubble content={streaming} citations={[]} streaming />
          ) : null}

          {activeTool ? (
            <p className="font-mono text-[10px] text-ink-4">running {activeTool}…</p>
          ) : status === "sending" ? (
            <p className="font-mono text-[10px] text-ink-4">thinking…</p>
          ) : null}

          {error ? (
            <p className="rounded-card border border-against/40 bg-against/10 px-3 py-2 text-[12px] text-against">
              {error}
            </p>
          ) : null}
        </div>

        {/* Composer */}
        <div className="shrink-0 border-t border-line p-2.5">
          {surfaceLabel ? (
            <div className="mb-1.5 flex items-center gap-1 px-1 font-mono text-[10px] text-ink-4">
              <span className="size-1 rounded-full bg-amber" />
              {surfaceLabel}
            </div>
          ) : null}
          <div className="flex items-end gap-2 rounded-card border border-line bg-field px-2.5 py-2 focus-within:border-amber-line">
            <textarea
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && !e.shiftKey) {
                  e.preventDefault();
                  submit();
                }
              }}
              rows={1}
              placeholder="Ask the copilot…"
              className="max-h-32 min-h-[20px] flex-1 resize-none bg-transparent text-[13px] leading-snug text-ink outline-none placeholder:text-ink-4"
            />
            <button
              type="button"
              onClick={submit}
              disabled={!input.trim() || busy}
              aria-label="Send"
              className="grid size-7 shrink-0 place-items-center rounded-chip bg-amber text-[#0f0f12] transition-opacity disabled:opacity-40"
            >
              <ArrowUp className="size-4" strokeWidth={2} />
            </button>
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
        <div className="max-w-[85%] whitespace-pre-wrap rounded-card bg-raise px-3 py-2 text-[13px] text-ink">
          {message.content}
        </div>
      </div>
    );
  }
  return <AssistantBubble content={message.content} citations={message.citations} />;
}

function AssistantBubble({
  content,
  citations,
  streaming,
}: {
  content: string;
  citations: CopilotCitation[];
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
      <div className="whitespace-pre-wrap text-[13px] leading-relaxed text-ink-2">
        {content}
        {streaming ? <span className="ml-0.5 inline-block h-3.5 w-[2px] animate-pulse bg-amber align-middle" /> : null}
      </div>
      {unique.length > 0 ? (
        <div className="flex flex-wrap gap-1.5">
          {unique.map((c) => (
            <button
              key={`${c.kind}|${c.id}`}
              type="button"
              onClick={() => navCitation(c)}
              className="max-w-[180px] truncate rounded-chip border border-line px-2 py-0.5 font-mono text-[10px] text-ink-3 transition-colors hover:border-amber-line hover:text-ink"
              title={`${c.kind}: ${c.title}`}
            >
              {c.title}
            </button>
          ))}
        </div>
      ) : null}
    </div>
  );
}
