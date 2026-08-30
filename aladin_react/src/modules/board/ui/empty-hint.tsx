import { useState } from "react";
import { useEditor, useValue } from "tldraw";

import { useBoardStatus } from "../domain/board-status";

/**
 * A brand-new board: one ghost line at the centre, and nothing else — Muse-blank otherwise.
 * It leaves with the first object and can be dismissed; it never comes back for the session.
 */
export function EmptyHint() {
  const editor = useEditor();
  const status = useBoardStatus();
  const empty = useValue("empty", () => editor.getCurrentPageShapeIds().size === 0, [editor]);
  const [dismissed, setDismissed] = useState(false);
  if (!empty || dismissed || (status.mode === "synced" && status.state !== "online")) return null;
  return (
    <button
      type="button"
      onClick={() => setDismissed(true)}
      aria-label="Dismiss hint"
      className="pointer-events-auto absolute left-1/2 top-1/2 w-[min(340px,70%)] -translate-x-1/2 -translate-y-1/2 px-4 py-3 text-center text-small text-ink-4"
    >
      Add a thought, drop a link, or bring something from your library.
    </button>
  );
}
