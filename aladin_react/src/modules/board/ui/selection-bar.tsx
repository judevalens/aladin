import { useEditor, useValue } from "tldraw";

import { useBoardHost } from "../domain/board-host";
import { describeShape } from "../domain/board-selection";
import { DOCK_PATHS, DockIcon } from "./dock-icons";

/**
 * The floating bar at the top of the board while exactly one object is selected.
 * Buttons appear only when the host can honor them; Remove always works — it removes the
 * WINDOW, never the artifact (product rule 2).
 */
export function SelectionBar() {
  const editor = useEditor();
  const host = useBoardHost();
  const shape = useValue("only-selected", () => editor.getOnlySelectedShape(), [editor]);
  const editing = useValue("editing-any", () => editor.getEditingShapeId() !== null, [editor]);

  if (!shape || editing) return null;
  const summary = describeShape(editor, shape);

  return (
    <div className="board-glass-dock pointer-events-auto absolute left-[22px] right-[22px] top-4 flex items-center gap-3.5 rounded-[18px] py-2.5 pl-4 pr-3">
      <div className="min-w-0 flex-1">
        <div className="truncate text-[15.5px] leading-[1.35] text-ink">{summary.title}</div>
        <div className="mt-[3px] flex items-center gap-2">
          {summary.cited ? (
            <span className="shrink-0 rounded-full bg-for/10 px-[9px] py-[3px] font-mono text-[10px] uppercase tracking-[0.05em] text-for">
              cited
            </span>
          ) : null}
          <span className="truncate font-mono text-[10.5px] text-ink-4">{summary.meta}</span>
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
            className="h-[42px] rounded-xl border border-amber-line bg-amber-soft px-3.5 text-[14.5px] font-semibold text-amber"
          >
            Ask about this
          </button>
        ) : null}
        {summary.openLabel && summary.artifactId && host.onOpenArtifact ? (
          <button
            type="button"
            onClick={() => host.onOpenArtifact?.(summary.artifactId as string)}
            className="h-[42px] rounded-xl border border-line px-3.5 text-[14.5px] text-ink-2 hover:text-ink"
          >
            {summary.openLabel}
          </button>
        ) : null}
        <button
          type="button"
          aria-label="Remove from board"
          title="Remove from board — the artifact stays in its folder"
          onClick={() => editor.deleteShapes([shape.id])}
          className="grid h-[42px] w-[42px] place-items-center rounded-xl text-ink-3 hover:bg-hover hover:text-against"
        >
          <DockIcon d={DOCK_PATHS.trash} size={17} strokeWidth={1.9} />
        </button>
      </div>
    </div>
  );
}
