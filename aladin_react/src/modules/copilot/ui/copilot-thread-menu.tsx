import { Archive, Check, MessageSquare, Pencil, Pin, X } from "lucide-react";
import { Icon } from "@/components/ui/icon";
import { useState, type MouseEvent } from "react";
import type { CopilotProposal, CopilotThreadView } from "@/app/state/copilot-slice";
import { DropdownMenuItem, DropdownMenuLabel } from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";

/**
 * The thread dropdown: pick a conversation, rename it, pin it, archive it.
 *
 * `threadMenuSections` is the whole sorting rule and is pure, so the "which bucket does a
 * running thread with a pending approval land in" question is answered by a unit test rather
 * than by opening the menu.
 */

export function activeThread(
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
  const sections = threadMenuSections(filtered, activeThreadId, proposals, status);

  if (threads.length === 0) {
    return <DropdownMenuItem disabled>No saved threads</DropdownMenuItem>;
  }
  if (filtered.length === 0) {
    return <DropdownMenuItem disabled>No matching threads</DropdownMenuItem>;
  }

  return (
    <div className="max-h-80 overflow-y-auto p-1">
      {sections.map((section) => (
        <div key={section.key}>
          <DropdownMenuLabel className="px-2 py-1 font-mono text-meta uppercase text-ink-4">
            {section.label}
          </DropdownMenuLabel>
          {section.rows.map(({ thread, pendingApprovals, running }) => {
            const active = thread.id === activeThreadId;
            const editing = thread.id === editingId;
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
                        {!thread.pinned && !running && pendingApprovals === 0 ? (
                          <span>{formatThreadTime(thread.updatedAt)}</span>
                        ) : null}
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
      ))}
    </div>
  );
}

type ThreadMenuSection = {
  key: "active" | "approval" | "pinned" | "recent";
  label: string;
  rows: {
    thread: CopilotThreadView;
    pendingApprovals: number;
    running: boolean;
  }[];
};

export function threadMenuSections(
  threads: CopilotThreadView[],
  activeThreadId: string | null,
  proposals: CopilotProposal[],
  status: "idle" | "sending" | "streaming",
): ThreadMenuSection[] {
  const sections: ThreadMenuSection[] = [
    { key: "active", label: "Active", rows: [] },
    { key: "approval", label: "Needs Approval", rows: [] },
    { key: "pinned", label: "Pinned", rows: [] },
    { key: "recent", label: "Recent", rows: [] },
  ];
  for (const thread of threads) {
    const pendingApprovals = proposals.filter(
      (p) =>
        p.threadId === thread.id &&
        (p.status === "pending" || p.status === "approving" || p.status === "rejecting"),
    ).length;
    const running = thread.id === activeThreadId && status !== "idle";
    const row = { thread, pendingApprovals, running };
    if (running) {
      sections[0].rows.push(row);
    } else if (pendingApprovals > 0) {
      sections[1].rows.push(row);
    } else if (thread.pinned) {
      sections[2].rows.push(row);
    } else {
      sections[3].rows.push(row);
    }
  }
  return sections.filter((section) => section.rows.length > 0);
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
