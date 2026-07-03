import { Link2, ThumbsDown, ThumbsUp, X } from "lucide-react";

import type { ClaimConnection } from "@/modules/graph/graph-pane-types";

// The Y3 "Connect" result dock: for each claim the page just produced, how many discovered
// sources support vs contradict it — the payoff of writing a hunch into the you-stream.
export function ConnectionsPanel({
  connections,
  loading,
  onClose,
}: {
  connections: ClaimConnection[];
  loading: boolean;
  onClose: () => void;
}) {
  return (
    <div className="absolute right-0 top-0 z-20 flex h-full w-80 flex-col border-l border-line bg-panel shadow-panel">
      <div className="flex items-center justify-between border-b border-line px-4 py-3">
        <div className="flex items-center gap-1.5 text-sm font-medium text-ink">
          <Link2 className="h-4 w-4 text-ink-3" />
          Connections
        </div>
        <button
          onClick={onClose}
          title="Close"
          className="rounded p-1 text-ink-3 hover:bg-[rgb(var(--hover))] hover:text-ink"
        >
          <X className="h-4 w-4" />
        </button>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto py-1">
        {loading ? (
          <div className="px-4 py-4 text-sm text-ink-4">Connecting…</div>
        ) : connections.length === 0 ? (
          <div className="px-4 py-6 text-sm text-ink-4">
            No claims to connect yet. Tag entities (<span className="text-ink-3">@</span>) and write a
            hunch, then hit Connect — support and contradiction will surface here.
          </div>
        ) : (
          connections.map((c) => <ConnectionCard key={c.claimId} conn={c} />)
        )}
      </div>
    </div>
  );
}

function ConnectionCard({ conn }: { conn: ClaimConnection }) {
  return (
    <div className="border-b border-line px-4 py-3 last:border-b-0">
      <div className="text-sm leading-snug text-ink">{conn.text}</div>
      <div className="mt-2 flex items-center gap-3 text-xs">
        <span className="rounded-chip bg-raise px-1.5 py-px text-ink-3">{conn.polarity}</span>
        <span className="flex items-center gap-1 text-for" title="sources that support this">
          <ThumbsUp className="h-3.5 w-3.5" />
          {conn.support}
        </span>
        <span className="flex items-center gap-1 text-against" title="sources that contradict this">
          <ThumbsDown className="h-3.5 w-3.5" />
          {conn.contradict}
        </span>
      </div>
    </div>
  );
}
