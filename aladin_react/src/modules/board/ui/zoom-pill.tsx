import { useEditor, useValue } from "tldraw";

import { DOCK_PATHS, DockIcon } from "./dock-icons";

/** Bottom-right zoom stepper. Hidden entirely in pencil mode (the parent decides). */
export function ZoomPill() {
  const editor = useEditor();
  const zoom = useValue("zoom", () => editor.getZoomLevel(), [editor]);

  return (
    <div className="board-island board-island--pill board-flank pointer-events-auto absolute right-5.5 flex items-center gap-0.5 p-1">
      <button
        type="button"
        aria-label="Zoom out"
        onClick={() => editor.zoomOut()}
        className="board-tile grid h-11 w-11 place-items-center rounded-control text-ink-3 hover:bg-hover hover:text-ink"
      >
        <DockIcon d={DOCK_PATHS.zoomOut} size={18} strokeWidth={2} />
      </button>
      <span className="min-w-[52px] text-center font-mono text-small text-ink-3">
        {Math.round(zoom * 100)}%
      </span>
      <button
        type="button"
        aria-label="Zoom in"
        onClick={() => editor.zoomIn()}
        className="board-tile grid h-11 w-11 place-items-center rounded-control text-ink-3 hover:bg-hover hover:text-ink"
      >
        <DockIcon d={DOCK_PATHS.zoomIn} size={18} strokeWidth={2} />
      </button>
    </div>
  );
}
