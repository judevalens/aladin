import { useEditor, useValue } from "tldraw";

import { useBoardHost } from "../domain/board-host";
import { describeShape } from "../domain/board-selection";
import { useBoardToasts } from "../domain/board-toasts";
import { DOCK_PATHS, DockIcon } from "./dock-icons";

/**
 * The floating bar at the top of the board while exactly one object is selected.
 * Buttons appear only when the host can honor them; Remove always works — it removes the
 * WINDOW, never the artifact (product rule 2).
 */
export function SelectionBar() {
  const editor = useEditor();
  const host = useBoardHost();
  const toasts = useBoardToasts();
  const shape = useValue("only-selected", () => editor.getOnlySelectedShape(), [editor]);
  const editing = useValue("editing-any", () => editor.getEditingShapeId() !== null, [editor]);

  if (!shape || editing) return null;
  const summary = describeShape(editor, shape);

  return (
    <div className="board-island board-edge-top pointer-events-auto absolute inset-x-5.5 flex items-center gap-3.5 rounded-board-card py-2 pl-4 pr-2">
      <div className="min-w-0 flex-1">
        <div className="truncate text-board-title text-ink">{summary.title}</div>
        <div className="mt-0.5 flex items-center gap-2">
          {summary.cited ? (
            <span className="shrink-0 rounded-full bg-for/10 px-2 py-0.5 font-mono text-board-meta uppercase tracking-wider text-for">
              cited
            </span>
          ) : null}
          <span className="truncate font-mono text-board-meta text-ink-4">{summary.meta}</span>
        </div>
      </div>
      <div className="flex shrink-0 gap-2">
        {host.onAskAbout ? (
          <button
            type="button"
            onClick={() =>
              host.onAskAbout?.({
                artifactId: summary.artifactId ?? undefined,
                title: summary.title,
              })
            }
            className="board-tile h-11 rounded-control border border-amber-line bg-amber-soft px-3.5 text-board-row font-semibold text-amber"
          >
            Ask about this
          </button>
        ) : null}
        {summary.openLabel && summary.artifactId && host.onOpenArtifact ? (
          <button
            type="button"
            onClick={() => host.onOpenArtifact?.(summary.artifactId as string)}
            className="board-tile h-11 rounded-control border border-line px-3.5 text-board-row text-ink-2 hover:text-ink"
          >
            {summary.openLabel}
          </button>
        ) : null}
        <button
          type="button"
          aria-label="Remove from board"
          title="Remove from board — the artifact stays in its folder"
          onClick={() => {
            // One tap removes the WINDOW; the toast's Undo is the only confirmation a
            // keyboard-less device gets, so it has to be there.
            const mark = editor.markHistoryStoppingPoint("remove-from-board");
            editor.deleteShapes([shape.id]);
            host.haptic?.("light");
            toasts.show({
              text: "Removed from the board — the artifact stays in its folder",
              action: { label: "Undo", onPress: () => editor.bailToMark(mark) },
            });
          }}
          className="board-tile grid h-11 w-11 place-items-center rounded-control text-ink-3 hover:bg-hover hover:text-against"
        >
          <DockIcon d={DOCK_PATHS.trash} size={17} strokeWidth={1.9} />
        </button>
      </div>
    </div>
  );
}
