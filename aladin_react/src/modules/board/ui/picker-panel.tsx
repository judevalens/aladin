import type { InsertRow } from "./insert-popover";
import { DOCK_PATHS, DockIcon } from "./dock-icons";

/**
 * The [+] picker — a panel floating above the dock, never a full-screen sheet. It clamps
 * to the viewport and scrolls its rows, so a long folder cannot run off the top of an iPad.
 */
export function PickerPanel({
  rows,
  emptyNote,
  onPaste,
  onClose,
}: {
  rows: InsertRow[];
  emptyNote: string | null;
  onPaste: () => void;
  onClose: () => void;
}) {
  return (
    <div className="board-island board-island--popover board-edge-above-dock pointer-events-auto absolute left-1/2 flex w-[464px] max-w-[calc(100vw-44px)] -translate-x-1/2 flex-col overflow-hidden">
      <div className="flex h-12 shrink-0 items-center gap-2.5 border-b border-line-2 pl-4 pr-1">
        <span className="text-ink-4">
          <DockIcon d={DOCK_PATHS.search} size={15} strokeWidth={2} />
        </span>
        <span className="text-board-label text-ink-4">search your workspace</span>
        <span className="ml-auto font-mono text-board-meta text-ink-4">this folder · then everywhere</span>
        <button
          type="button"
          aria-label="Close"
          onClick={onClose}
          className="board-tile grid h-11 w-11 shrink-0 place-items-center rounded-control text-ink-3 hover:bg-hover"
        >
          <DockIcon d={DOCK_PATHS.close} size={15} strokeWidth={2} />
        </button>
      </div>
      <div className="max-h-[min(52vh,420px)] overflow-y-auto overscroll-contain">
        {rows.map((row) => (
          <button
            key={row.key}
            type="button"
            onClick={row.onPick}
            className="flex h-12 w-full items-center gap-3 px-4 hover:bg-hover active:bg-sel"
          >
            <span className="grid w-[19px] shrink-0 place-items-center text-ink-3">{row.icon}</span>
            <span className="min-w-0 truncate text-board-row text-ink">{row.title}</span>
            <span
              className={`ml-auto shrink-0 font-mono text-board-meta ${
                row.metaTone === "amber" ? "text-amber" : "text-ink-4"
              }`}
            >
              {row.meta}
            </span>
          </button>
        ))}
        {emptyNote ? (
          <div className="px-4 py-3 font-mono text-board-meta text-ink-4">{emptyNote}</div>
        ) : null}
      </div>
      <button
        type="button"
        onClick={onPaste}
        className="flex h-12 w-full shrink-0 items-center gap-3 border-t border-line-2 px-4 hover:bg-hover active:bg-sel"
      >
        <span className="text-board-learn grid w-[19px] shrink-0 place-items-center">
          <DockIcon d={DOCK_PATHS.clipboard} size={17} />
        </span>
        <span className="text-board-row text-ink-2">Paste from clipboard</span>
        <span className="ml-auto shrink-0 font-mono text-board-meta text-ink-4">becomes an excerpt</span>
      </button>
    </div>
  );
}
