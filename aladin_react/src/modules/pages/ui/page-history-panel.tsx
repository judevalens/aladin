import { RefreshCcw, Sparkles, X } from "lucide-react";
import { usePageHistory } from "@/modules/pages/hooks/use-page-history";
import type { PageEditEntry } from "@/repos/pages/page-attribution-repo";

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

function HistoryRow({ entry }: { entry: PageEditEntry }) {
  const isAgent = entry.editorKind === "agent";
  return (
    <div className="flex items-start gap-3 rounded-md px-2 py-2 hover:bg-[#faf9f8]">
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
  return (
    <div className="absolute right-0 top-0 z-20 flex h-full w-72 flex-col border-l border-[#e7e5e4] bg-white shadow-lg">
      <div className="flex items-center justify-between border-b border-[#f2f0ee] px-4 py-3">
        <div className="text-sm font-medium text-[#292524]">History</div>
        <div className="flex items-center gap-1">
          <button
            onClick={refetch}
            title="Refresh"
            className="rounded p-1 text-[#78716c] hover:bg-[#f5f5f4]"
          >
            <RefreshCcw className="h-4 w-4" />
          </button>
          <button
            onClick={onClose}
            title="Close"
            className="rounded p-1 text-[#78716c] hover:bg-[#f5f5f4]"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto p-2">
        {loading ? (
          <div className="px-2 py-4 text-sm text-[#a8a29e]">Loading…</div>
        ) : entries.length === 0 ? (
          <div className="px-2 py-4 text-sm text-[#a8a29e]">No edits recorded yet.</div>
        ) : (
          entries.map((e, i) => <HistoryRow key={`${e.occurredAt}-${i}`} entry={e} />)
        )}
      </div>
    </div>
  );
}
