import { useBoardStatus } from "../domain/board-status";

/**
 * Bottom-left: the synced board's connection, only when there is something to say. Online
 * is silence; offline says edits are queued (they rebase on reconnect); a terminal sync
 * error names its cause and offers Retry — tldraw never redials those on its own. Local
 * boards (the spike) show nothing.
 */
export function StatusPill() {
  const status = useBoardStatus();
  if (status.mode === "local" || status.state === "online" || status.state === "loading") {
    return null;
  }
  if (status.state === "offline") {
    return <Pill tone="quiet">offline — edits sync when you're back</Pill>;
  }
  return (
    <Pill tone="against">
      <span className="min-w-0 truncate">{status.reason ?? "not connected"}</span>
      <button
        type="button"
        onClick={status.retry}
        className="board-tile h-11 shrink-0 rounded-control px-3 font-display text-ink hover:bg-hover"
      >
        Retry
      </button>
    </Pill>
  );
}

function Pill({ tone, children }: { tone: "quiet" | "against"; children: React.ReactNode }) {
  return (
    <div
      role="status"
      className={`board-island board-island--pill board-flank pointer-events-auto absolute left-5.5 flex h-11 max-w-[60vw] items-center gap-2 pl-3.5 pr-1.5 font-mono text-small ${
        tone === "against" ? "text-against" : "text-ink-3"
      }`}
    >
      {children}
    </div>
  );
}
