import { useEffect, useState } from "react";

import { useBoardStatus } from "../domain/board-status";

/**
 * Bottom-left: the board's persistence state, only when there is something to say. "Saved"
 * is silence; a save in flight shows after a beat (so a keystroke never flickers it); a
 * failure says so and that it is retrying; a failed load offers Retry. The iPad's pane has
 * no header bar, so this is the only place the state is visible there.
 */
export function StatusPill() {
  const status = useBoardStatus();
  const busy = status.save === "dirty" || status.save === "saving";
  const showBusy = useDelayed(busy, 900);

  if (status.load === "failed") {
    return (
      <Pill tone="against">
        <span>couldn't load this board</span>
        <button
          type="button"
          onClick={status.retryLoad}
          className="board-tile h-11 rounded-control px-3 text-ink hover:bg-hover"
        >
          Retry
        </button>
      </Pill>
    );
  }
  if (status.load === "loading") return <Pill tone="quiet">loading…</Pill>;
  if (status.save === "error") return <Pill tone="against">couldn't save — retrying</Pill>;
  if (showBusy) return <Pill tone="quiet">saving…</Pill>;
  return null;
}

function Pill({ tone, children }: { tone: "quiet" | "against"; children: React.ReactNode }) {
  return (
    <div
      role="status"
      className={`board-island board-island--pill board-flank pointer-events-auto absolute left-5.5 flex h-11 items-center gap-2 pl-3.5 pr-1.5 font-mono text-small ${
        tone === "against" ? "text-against" : "text-ink-3"
      }`}
    >
      {children}
    </div>
  );
}

/** True once `value` has been true for `ms` straight; false immediately when it drops. */
function useDelayed(value: boolean, ms: number): boolean {
  const [delayed, setDelayed] = useState(false);
  useEffect(() => {
    if (!value) {
      setDelayed(false);
      return;
    }
    const handle = window.setTimeout(() => setDelayed(true), ms);
    return () => window.clearTimeout(handle);
  }, [value, ms]);
  return delayed;
}
