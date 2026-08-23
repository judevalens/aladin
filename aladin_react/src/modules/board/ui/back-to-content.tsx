import { useEditor, useValue } from "tldraw";

import { DOCK_PATHS, DockIcon } from "./dock-icons";

/**
 * tldraw's HelperButtons idiom: when the camera has wandered off every object and the board
 * has objects, one pill brings it back. The cheap answer to "I flung the board away and
 * cannot find it" — no minimap needed for that.
 */
export function BackToContent() {
  const editor = useEditor();
  const lost = useValue(
    "lost",
    () => {
      const bounds = editor.getCurrentPageBounds();
      if (!bounds) return false;
      return !editor.getViewportPageBounds().collides(bounds);
    },
    [editor],
  );
  if (!lost) return null;
  return (
    <button
      type="button"
      onClick={() => editor.zoomToFit({ animation: { duration: 260 } })}
      className="board-island board-island--pill board-edge-top pointer-events-auto absolute left-1/2 flex h-11 -translate-x-1/2 items-center gap-2 pl-3 pr-4 text-board-row text-ink-2 hover:text-ink"
    >
      <DockIcon d={DOCK_PATHS.fit} size={17} strokeWidth={1.9} />
      Back to the board
    </button>
  );
}
