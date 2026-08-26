import { useEffect, useState } from "react";
import { useEditor, useValue } from "tldraw";

import { useBoardContent, useBoardFolder } from "../domain/board-content";
import { useBoardHost } from "../domain/board-host";
import { describeShape } from "../domain/board-selection";
import { useBoardToasts } from "../domain/board-toasts";
import { DOCK_PATHS, DockIcon } from "./dock-icons";

/**
 * The floating bar at the top of the board while something is selected — compact, centred,
 * actions only (the caption it once carried duplicated what the selected object already
 * shows; owner cut it 2026-08-24).
 *
 * One object: the host's actions (Ask / Open), a "…" overflow with the arrange verbs a
 * long-press would otherwise carry (long-press means insert here), and Remove. Several: a
 * small count, Group/Ungroup, Remove. Buttons appear only when the host can honor them;
 * Remove always works — it removes the WINDOW(S), never the artifact (rule 2), and says so
 * with an Undo.
 */
export function SelectionBar() {
  const editor = useEditor();
  const host = useBoardHost();
  const toasts = useBoardToasts();
  const contentSource = useBoardContent();
  const folderId = useBoardFolder();
  const shapes = useValue("selected", () => editor.getSelectedShapes(), [editor]);
  const editing = useValue("editing-any", () => editor.getEditingShapeId() !== null, [editor]);
  const [moreOpen, setMoreOpen] = useState(false);

  // The overflow closes with the selection and on any tap on the plane.
  useEffect(() => {
    if (!moreOpen) return;
    const close = (info: { name: string }) => {
      if (info.name === "pointer_down") setMoreOpen(false);
    };
    editor.on("event", close);
    return () => {
      editor.off("event", close);
    };
  }, [editor, moreOpen]);
  useEffect(() => {
    setMoreOpen(false);
  }, [shapes.length]);

  if (shapes.length === 0 || editing) return null;
  const ids = shapes.map((s) => s.id);

  const remove = () => {
    const mark = editor.markHistoryStoppingPoint("remove-from-board");
    editor.deleteShapes(ids);
    host.haptic?.("light");
    toasts.show({
      text:
        ids.length === 1
          ? "Removed from the board — the artifact stays in its folder"
          : `Removed ${ids.length} objects from the board — artifacts stay in their folders`,
      action: { label: "Undo", onPress: () => editor.bailToMark(mark) },
    });
  };

  if (shapes.length > 1) {
    const groups = shapes.filter((s) => s.type === "group").length;
    return (
      <Bar>
        <span className="px-2 font-mono text-board-meta text-ink-3">{shapes.length} objects</span>
        {groups > 0 ? (
          <TextAction label="Ungroup" onClick={() => editor.ungroupShapes(ids)} />
        ) : (
          <TextAction label="Group" onClick={() => editor.groupShapes(ids)} />
        )}
        <RemoveButton onClick={remove} />
      </Bar>
    );
  }

  const shape = shapes[0];
  const summary = describeShape(editor, shape);
  const locked = editor.getShape(shape.id)?.isLocked ?? false;

  return (
    <Bar>
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
        {shape.type === "aladin-doc" &&
        summary.artifactId &&
        contentSource?.createWorksheet &&
        host.onOpenArtifact ? (
          <TextAction
            label="Work this"
            onClick={() => {
              const cite = {
                artifactId: summary.artifactId as string,
                page: summary.page ?? 1,
                title: summary.title,
              };
              void contentSource
                .createWorksheet?.({
                  folderId,
                  title: `Worksheet — ${summary.title} · p. ${cite.page}`,
                  cite,
                })
                .then((id) => {
                  host.haptic?.("light");
                  host.onOpenArtifact?.(id);
                })
                .catch(() => {
                  toasts.show({ text: "Couldn't create the worksheet — try again" });
                });
            }}
          />
        ) : null}
        {summary.openLabel && summary.artifactId && host.onOpenArtifact ? (
          <TextAction
            label={summary.openLabel}
            onClick={() =>
              host.onOpenArtifact?.(
                summary.artifactId as string,
                summary.page != null ? { page: summary.page } : undefined,
              )
            }
          />
        ) : null}
        <button
          type="button"
          aria-label="More"
          aria-expanded={moreOpen}
          onClick={() => setMoreOpen((open) => !open)}
          className={`board-tile grid h-11 w-11 place-items-center rounded-control ${
            moreOpen ? "bg-sel text-ink" : "text-ink-3 hover:bg-hover hover:text-ink"
          }`}
        >
          <DockIcon d={DOCK_PATHS.more} size={19} strokeWidth={2.2} />
        </button>
      <RemoveButton onClick={remove} />
      {moreOpen ? (
        <div
          role="menu"
          className="board-island board-island--popover absolute right-0 top-[calc(100%+10px)] flex w-60 flex-col overflow-hidden py-1"
        >
          <MenuRow
            label="Duplicate"
            onPick={() => {
              setMoreOpen(false);
              editor.duplicateShapes(ids, { x: 24, y: 24 });
            }}
          />
          <MenuRow
            label="Bring to front"
            onPick={() => {
              setMoreOpen(false);
              editor.bringToFront(ids);
            }}
          />
          <MenuRow
            label="Send to back"
            onPick={() => {
              setMoreOpen(false);
              editor.sendToBack(ids);
            }}
          />
          <MenuRow
            label={locked ? "Unlock" : "Lock in place"}
            onPick={() => {
              setMoreOpen(false);
              editor.toggleLock(ids);
            }}
          />
        </div>
      ) : null}
    </Bar>
  );
}

function Bar({ children }: { children: React.ReactNode }) {
  return (
    <div className="board-island board-edge-top pointer-events-auto absolute left-1/2 flex max-w-[calc(100vw-24px)] -translate-x-1/2 items-center gap-2 rounded-board-card p-1.5">
      {children}
    </div>
  );
}

function TextAction({ label, onClick }: { label: string; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="board-tile h-11 rounded-control border border-line px-3.5 text-board-row text-ink-2 hover:text-ink"
    >
      {label}
    </button>
  );
}

function RemoveButton({ onClick }: { onClick: () => void }) {
  return (
    <button
      type="button"
      aria-label="Remove from board"
      title="Remove from board — the artifact stays in its folder"
      onClick={onClick}
      className="board-tile grid h-11 w-11 place-items-center rounded-control text-ink-3 hover:bg-hover hover:text-against"
    >
      <DockIcon d={DOCK_PATHS.trash} size={17} strokeWidth={1.9} />
    </button>
  );
}

function MenuRow({ label, onPick }: { label: string; onPick: () => void }) {
  return (
    <button
      type="button"
      role="menuitem"
      onClick={onPick}
      className="flex h-11 w-full items-center px-4 text-left text-board-row text-ink hover:bg-hover active:bg-sel"
    >
      {label}
    </button>
  );
}
