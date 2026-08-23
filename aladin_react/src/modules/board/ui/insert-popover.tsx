import type { ReactNode } from "react";

export interface InsertRow {
  key: string;
  icon: ReactNode;
  title: string;
  meta: string;
  /** "on this board" rows read amber and fly to the object instead of inserting. */
  metaTone?: "amber";
  onPick: () => void;
}

const POPOVER_WIDTH = 396;
const EDGE = 12;

/**
 * The hold-to-insert popover — anchored where the plane was held (~400ms), so the drop
 * point survives ("lands here"). Rows come from the caller; dismissal is the chrome's
 * (any pointer-down on the plane). It flips left of the point when the right edge would
 * leave the viewport, and sits above it when the bottom would.
 */
export function InsertPopover({
  x,
  y,
  viewportWidth,
  viewportHeight,
  rows,
  footer,
}: {
  /** Viewport coordinates of the held point. */
  x: number;
  y: number;
  viewportWidth: number;
  viewportHeight: number;
  rows: InsertRow[];
  footer: string;
}) {
  const estimatedHeight = 44 + rows.length * 48 + 40;
  const left =
    x + 34 + POPOVER_WIDTH + EDGE > viewportWidth
      ? Math.max(EDGE, x - 34 - POPOVER_WIDTH)
      : x + 34;
  const top =
    y + 26 + estimatedHeight + EDGE > viewportHeight
      ? Math.max(EDGE, y - 26 - estimatedHeight)
      : y + 26;

  return (
    <>
      <div
        className="pointer-events-none absolute h-[52px] w-[52px] rounded-full border-2 border-amber bg-amber-soft"
        style={{ left: x - 26, top: y - 26 }}
      />
      <div
        className="board-island board-island--popover pointer-events-auto absolute overflow-hidden"
        style={{ left, top, width: Math.min(POPOVER_WIDTH, viewportWidth - 2 * EDGE) }}
      >
        <div className="flex h-11 items-center gap-2.5 border-b border-line-2 px-4">
          <span className="text-board-label text-ink-4">filter — or just pick</span>
          <span className="ml-auto font-mono text-board-meta text-ink-4">lands here</span>
        </div>
        {rows.map((row) => (
          <button
            key={row.key}
            type="button"
            onClick={row.onPick}
            className="flex h-12 w-full items-center gap-3 px-4 hover:bg-hover active:bg-sel"
          >
            <span className="grid w-[19px] shrink-0 place-items-center text-ink-3">{row.icon}</span>
            <span className="min-w-0 truncate text-board-row text-ink">{row.title}</span>
            <span className="ml-auto shrink-0 font-mono text-board-meta text-ink-4">{row.meta}</span>
          </button>
        ))}
        <div className="border-t border-line-2 px-4 py-2.5 font-mono text-board-meta text-ink-4">
          {footer}
        </div>
      </div>
    </>
  );
}
