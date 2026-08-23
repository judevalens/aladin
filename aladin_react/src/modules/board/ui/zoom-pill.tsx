import { useEffect, useRef, useState } from "react";
import { useEditor, useValue } from "tldraw";

import { DOCK_PATHS, DockIcon } from "./dock-icons";

const LONG_PRESS_MS = 500;

/**
 * Bottom-right zoom: `−` / `%` / `+`. The percentage is a button — tap for the menu
 * (Zoom to fit · 100% · Zoom to selection), hold to reset to 100%. Hidden entirely in
 * pencil mode (the parent decides).
 */
export function ZoomPill() {
  const editor = useEditor();
  const zoom = useValue("zoom", () => editor.getZoomLevel(), [editor]);
  const hasSelection = useValue("has-selection", () => editor.getSelectedShapeIds().length > 0, [editor]);
  const [menuOpen, setMenuOpen] = useState(false);
  const pressTimer = useRef<number | null>(null);
  const longPressed = useRef(false);

  // Any tap on the plane closes the menu, like every other floating thing on the board.
  useEffect(() => {
    if (!menuOpen) return;
    const close = (info: { name: string }) => {
      if (info.name === "pointer_down") setMenuOpen(false);
    };
    editor.on("event", close);
    return () => {
      editor.off("event", close);
    };
  }, [editor, menuOpen]);

  const pick = (fn: () => void) => {
    setMenuOpen(false);
    fn();
  };

  return (
    <div className="board-island board-island--pill board-flank pointer-events-auto absolute right-5.5 flex items-center gap-0.5 p-1">
      <button
        type="button"
        aria-label="Zoom out"
        onClick={() => editor.zoomOut(undefined, { animation: { duration: 120 } })}
        className="board-tile grid h-11 w-11 place-items-center rounded-control text-ink-3 hover:bg-hover hover:text-ink"
      >
        <DockIcon d={DOCK_PATHS.zoomOut} size={18} strokeWidth={2} />
      </button>
      <button
        type="button"
        aria-label="Zoom level — tap for options, hold to reset"
        aria-expanded={menuOpen}
        onPointerDown={() => {
          longPressed.current = false;
          pressTimer.current = window.setTimeout(() => {
            longPressed.current = true;
            pressTimer.current = null;
            editor.resetZoom(undefined, { animation: { duration: 160 } });
          }, LONG_PRESS_MS);
        }}
        onPointerUp={() => {
          if (pressTimer.current !== null) window.clearTimeout(pressTimer.current);
          pressTimer.current = null;
        }}
        onPointerLeave={() => {
          if (pressTimer.current !== null) window.clearTimeout(pressTimer.current);
          pressTimer.current = null;
        }}
        onClick={() => {
          if (longPressed.current) return;
          setMenuOpen((open) => !open);
        }}
        className={`board-tile h-11 min-w-[56px] rounded-control px-1 text-center font-mono text-small ${
          menuOpen ? "bg-sel text-ink" : "text-ink-3 hover:bg-hover hover:text-ink"
        }`}
      >
        {Math.round(zoom * 100)}%
      </button>
      <button
        type="button"
        aria-label="Zoom in"
        onClick={() => editor.zoomIn(undefined, { animation: { duration: 120 } })}
        className="board-tile grid h-11 w-11 place-items-center rounded-control text-ink-3 hover:bg-hover hover:text-ink"
      >
        <DockIcon d={DOCK_PATHS.zoomIn} size={18} strokeWidth={2} />
      </button>
      {menuOpen ? (
        <div
          role="menu"
          className="board-island board-island--popover absolute bottom-[calc(100%+10px)] right-0 flex w-72 flex-col overflow-hidden py-1"
        >
          <MenuRow
            label="Zoom to fit"
            hint="all objects"
            onPick={() => pick(() => editor.zoomToFit({ animation: { duration: 220 } }))}
          />
          <MenuRow
            label="100%"
            hint="1 : 1"
            onPick={() => pick(() => editor.resetZoom(undefined, { animation: { duration: 160 } }))}
          />
          <MenuRow
            label="Zoom to selection"
            hint={hasSelection ? "" : "nothing selected"}
            disabled={!hasSelection}
            onPick={() => pick(() => editor.zoomToSelection({ animation: { duration: 220 } }))}
          />
        </div>
      ) : null}
    </div>
  );
}

function MenuRow({
  label,
  hint,
  disabled,
  onPick,
}: {
  label: string;
  hint: string;
  disabled?: boolean;
  onPick: () => void;
}) {
  return (
    <button
      type="button"
      role="menuitem"
      disabled={disabled}
      onClick={onPick}
      className="flex h-11 w-full items-center gap-3 px-4 text-left text-board-row text-ink hover:bg-hover active:bg-sel disabled:text-ink-4 disabled:hover:bg-transparent"
    >
      <span className="whitespace-nowrap">{label}</span>
      <span className="ml-auto truncate font-mono text-board-meta text-ink-4">{hint}</span>
    </button>
  );
}
