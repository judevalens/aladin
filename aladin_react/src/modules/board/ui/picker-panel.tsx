import type { InsertRow } from "./insert-popover";
import { DOCK_PATHS, DockIcon } from "./dock-icons";

/**
 * The [+] picker — a 464px panel floating above the dock, never a full-screen sheet.
 * Artifact rows arrive with the content plane (P3); the paste door works from day one.
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
    <div className="board-glass-popover pointer-events-auto absolute bottom-[calc(92px+var(--host-bottom-inset,0px))] left-1/2 w-[464px] -translate-x-1/2 overflow-hidden">
      <div className="flex h-[46px] items-center gap-2.5 border-b border-line-2 px-[15px]">
        <span className="text-ink-4">
          <DockIcon d={DOCK_PATHS.search} size={15} strokeWidth={2} />
        </span>
        <span className="text-[14px] text-ink-4">search your workspace</span>
        <span className="ml-auto font-mono text-[10px] text-ink-4">this folder · then everywhere</span>
        <button
          type="button"
          aria-label="Close"
          onClick={onClose}
          className="-mr-1.5 grid h-[30px] w-[30px] place-items-center rounded-[9px] text-ink-3 hover:bg-hover"
        >
          <DockIcon d={DOCK_PATHS.close} size={15} strokeWidth={2} />
        </button>
      </div>
      {rows.map((row) => (
        <button
          key={row.key}
          type="button"
          onClick={row.onPick}
          className="flex h-12 w-full items-center gap-3 px-[15px] hover:bg-hover"
        >
          <span className="grid w-[19px] shrink-0 place-items-center text-ink-3">{row.icon}</span>
          <span className="min-w-0 truncate text-[15px] text-ink">{row.title}</span>
          <span
            className={`ml-auto shrink-0 font-mono text-[10px] ${
              row.metaTone === "amber" ? "text-amber" : "text-ink-4"
            }`}
          >
            {row.meta}
          </span>
        </button>
      ))}
      {emptyNote ? (
        <div className="px-[15px] py-3 font-mono text-[10px] text-ink-4">{emptyNote}</div>
      ) : null}
      <button
        type="button"
        onClick={onPaste}
        className="flex h-12 w-full items-center gap-3 border-t border-line-2 px-[15px] hover:bg-hover"
      >
        <span className="grid w-[19px] shrink-0 place-items-center" style={{ color: "var(--board-learn)" }}>
          <DockIcon d={DOCK_PATHS.clipboard} size={17} />
        </span>
        <span className="text-[15px] text-ink-2">Paste from clipboard</span>
        <span className="ml-auto shrink-0 font-mono text-[10px] text-ink-4">becomes an excerpt</span>
      </button>
    </div>
  );
}
