import { diffWords } from "diff";
import { ChevronDown, ChevronRight, RefreshCcw, Sparkles, X } from "lucide-react";
import { useEffect, useState } from "react";
import { useAppComposition } from "@/app/composition/app-composition";
import { usePageHistory } from "@/modules/pages/hooks/use-page-history";
import type { PageDiff, PageEditEntry } from "@/repos/pages/page-attribution-repo";

function initials(name: string): string {
  const at = name.indexOf("@");
  const base = at > 0 ? name.slice(0, at) : name;
  const parts = base.split(/[.\s_-]+/).filter(Boolean);
  const chars = (parts[0]?.[0] ?? "?") + (parts[1]?.[0] ?? "");
  return chars.toUpperCase();
}

function colorFor(name: string): string {
  let hash = 0;
  for (let i = 0; i < name.length; i += 1) hash = (hash * 31 + name.charCodeAt(i)) | 0;
  return `hsl(${Math.abs(hash) % 360} 60% 45%)`;
}

function relTime(iso: string): string {
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return "";
  const diff = Date.now() - t;
  const m = Math.floor(diff / 60000);
  if (m < 1) return "just now";
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  return new Date(iso).toLocaleDateString(undefined, { month: "short", day: "numeric" });
}

// Word-level diff of two markdown snapshots, rendered inline.
function DiffView({ pageId, entryId }: { pageId: string; entryId: string }) {
  const { repos } = useAppComposition();
  const [diff, setDiff] = useState<PageDiff | null>(null);

  useEffect(() => {
    let cancelled = false;
    void repos.pages.getDiff(pageId, entryId).then((d) => {
      if (!cancelled) setDiff(d);
    });
    return () => {
      cancelled = true;
    };
  }, [repos, pageId, entryId]);

  if (!diff) {
    return <div className="px-3 py-2 text-xs text-[#a8a29e]">Loading diff…</div>;
  }
  if (!diff.before && !diff.after) {
    return <div className="px-3 py-2 text-xs text-[#a8a29e]">No snapshot for this edit yet.</div>;
  }
  const parts = diffWords(diff.before, diff.after);
  return (
    <pre className="mx-1 mb-1 max-h-64 overflow-auto whitespace-pre-wrap rounded-md border border-[#f2f0ee] bg-[#fafaf9] p-2 text-[11px] leading-5 text-[#57534e]">
      {parts.map((p, i) => (
        <span
          key={i}
          className={
            p.added
              ? "rounded bg-green-100 text-green-800"
              : p.removed
                ? "rounded bg-red-100 text-red-700 line-through"
                : ""
          }
        >
          {p.value}
        </span>
      ))}
    </pre>
  );
}

function HistoryRow({ entry, expanded }: { entry: PageEditEntry; expanded: boolean }) {
  const isAgent = entry.editorKind === "agent";
  return (
    <div className="flex items-start gap-2 px-2 py-2">
      <div className="mt-1.5 text-[#a8a29e]">
        {expanded ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
      </div>
      <div
        className="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-[10px] font-semibold text-white"
        style={{ backgroundColor: colorFor(entry.editorName) }}
      >
        {isAgent ? <Sparkles className="h-3.5 w-3.5" /> : initials(entry.editorName)}
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-1.5">
          <span className="truncate text-sm text-[#292524]">{entry.editorName}</span>
          {isAgent ? (
            <span className="rounded-full bg-[#f3e8ff] px-1.5 py-px text-[10px] font-medium text-[#7e22ce]">
              agent
            </span>
          ) : null}
        </div>
        <div className="text-xs text-[#78716c]">
          edited · {relTime(entry.occurredAt)}
          {entry.edits > 1 ? ` · ${entry.edits} changes` : ""}
        </div>
      </div>
    </div>
  );
}

export function PageHistoryPanel({
  pageId,
  onClose,
}: {
  pageId: string;
  onClose: () => void;
}) {
  const { entries, loading, refetch } = usePageHistory(pageId);
  const [expandedId, setExpandedId] = useState<string | null>(null);

  return (
    <div className="absolute right-0 top-0 z-20 flex h-full w-80 flex-col border-l border-[#e7e5e4] bg-white shadow-lg">
      <div className="flex items-center justify-between border-b border-[#f2f0ee] px-4 py-3">
        <div className="text-sm font-medium text-[#292524]">History</div>
        <div className="flex items-center gap-1">
          <button onClick={refetch} title="Refresh" className="rounded p-1 text-[#78716c] hover:bg-[#f5f5f4]">
            <RefreshCcw className="h-4 w-4" />
          </button>
          <button onClick={onClose} title="Close" className="rounded p-1 text-[#78716c] hover:bg-[#f5f5f4]">
            <X className="h-4 w-4" />
          </button>
        </div>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto py-1">
        {loading ? (
          <div className="px-4 py-4 text-sm text-[#a8a29e]">Loading…</div>
        ) : entries.length === 0 ? (
          <div className="px-4 py-4 text-sm text-[#a8a29e]">No edits recorded yet.</div>
        ) : (
          entries.map((e) => {
            const open = expandedId === e.id;
            return (
              <div key={e.id} className="border-b border-[#f7f6f5] last:border-b-0">
                <button
                  onClick={() => setExpandedId(open ? null : e.id)}
                  className="w-full rounded text-left hover:bg-[#faf9f8]"
                >
                  <HistoryRow entry={e} expanded={open} />
                </button>
                {open ? <DiffView pageId={pageId} entryId={e.id} /> : null}
              </div>
            );
          })
        )}
      </div>
    </div>
  );
}
