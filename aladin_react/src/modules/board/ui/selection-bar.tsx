import { useEffect, useState } from "react";
import { Copy, MoreHorizontal, Pencil, Trash2 } from "lucide-react";
import { useEditor, useValue, type TLDefaultColorStyle } from "tldraw";
import { useBoardContent, useBoardFolder } from "../domain/board-content";
import { useBoardHost } from "../domain/board-host";
import { describeShape } from "../domain/board-selection";
import { useBoardToasts } from "../domain/board-toasts";
import { BoardButton } from "./board-button";

const TINTS = [
  { id: "neutral", color: "white" },
  { id: "butter", color: "yellow" },
  { id: "sage", color: "light-green" },
  { id: "lilac", color: "light-violet" },
] as const;

/** Selection-local actions. Nothing remains pinned after deselection or during a drag. */
export function SelectionBar({ hidden = false }: { hidden?: boolean }) {
  const editor = useEditor();
  const host = useBoardHost();
  const toasts = useBoardToasts();
  const content = useBoardContent();
  const folderId = useBoardFolder();
  const shapes = useValue("selected", () => editor.getSelectedShapes(), [editor]);
  const editing = useValue("editing-any", () => editor.getEditingShapeId() !== null, [editor]);
  const idle = useValue("selection-idle", () => editor.isIn("select.idle"), [editor]);
  const viewport = useValue("selection-viewport", () => editor.getViewportScreenBounds(), [editor]);
  const anchor = useValue("selection-position", () => {
    const bounds = editor.getSelectionPageBounds();
    return bounds ? editor.pageToViewport({ x: bounds.midX, y: bounds.minY }) : null;
  }, [editor]);
  const [moreOpen, setMoreOpen] = useState(false);
  const selectionKey = shapes.map((shape) => shape.id).join(",");
  useEffect(() => { setMoreOpen(false); }, [selectionKey, hidden]);
  useEffect(() => {
    const dismiss = (event: { name: string }) => { if (event.name === "pointer_down") setMoreOpen(false); };
    editor.on("event", dismiss);
    return () => { editor.off("event", dismiss); };
  }, [editor]);
  if (hidden || !idle || editing || !anchor || !shapes.length) return null;

  const ids = shapes.map((shape) => shape.id);
  const shape = shapes.length === 1 ? shapes[0] : null;
  const summary = shape ? describeShape(editor, shape) : null;
  const editable = shape && editor.getShapeUtil(shape).canEdit(shape, { type: "unknown" });
  const canTint = shape && (shape.type === "note" || shape.type.startsWith("aladin-"));
  const perform = (label: string, action: () => void) => {
    editor.markHistoryStoppingPoint(label); action(); setMoreOpen(false);
  };
  const remove = () => {
    const mark = editor.markHistoryStoppingPoint("remove-from-board");
    editor.deleteShapes(ids);
    host.haptic?.("light");
    toasts.show({ text: "Removed from board", action: { label: "Undo", onPress: () => editor.bailToMark(mark) } });
  };
  return <div className="rs-selection rs-surface board-selection" role="toolbar" aria-label="Object actions" style={{
    left: Math.max(Math.min(190, viewport.w / 2), Math.min(viewport.w - Math.min(190, viewport.w / 2), anchor.x)),
    top: Math.max(60, Math.min(viewport.h - 110, anchor.y - 52)),
  }}>
    {editable && <button className="rs-selection-edit" onClick={() => { editor.setCurrentTool("select"); editor.setEditingShape(shape.id); }}><Pencil size={15} />{shape.type === "note" ? "Edit note" : "Edit"}</button>}
    {summary?.artifactId && host.onOpenArtifact && <button className="rs-selection-edit" onClick={() => host.onOpenArtifact?.(summary.artifactId!, summary.page != null ? { page: summary.page } : undefined)}>Open</button>}
    {shape?.type === "aladin-link" && /^https?:\/\//i.test(shape.props.url) && <a className="rs-selection-edit" href={shape.props.url} target="_blank" rel="noopener noreferrer">Open source</a>}
    {canTint && <>{TINTS.map((tint) => <button key={tint.id} type="button" className={"rs-tint-dot rs-tint--" + tint.id} aria-label={tint.id + " card"} aria-pressed={shape.type === "note" ? shape.props.color === tint.color : (shape.meta.boardTint ?? "neutral") === tint.id} onClick={() => perform("change card colour", () => {
      if (shape.type === "note") editor.updateShape({ id: shape.id, type: "note", props: { color: tint.color as TLDefaultColorStyle } });
      else editor.updateShape({ id: shape.id, type: shape.type, meta: { ...shape.meta, boardTint: tint.id } });
    })} />)}<span className="rs-divider" /></>}
    {shapes.length > 1 && <button className="rs-selection-edit" onClick={() => perform("group objects", () => editor.groupShapes(ids))}>Group {shapes.length}</button>}
    {shapes.some((item) => item.type === "group") && <button className="rs-selection-edit" onClick={() => perform("ungroup objects", () => editor.ungroupShapes(ids))}>Ungroup</button>}
    <BoardButton label="Duplicate selection" icon={Copy} onClick={() => perform("duplicate selection", () => editor.duplicateShapes(ids, { x: 24, y: 24 }))} />
    <BoardButton label="More" icon={MoreHorizontal} aria-expanded={moreOpen} onClick={() => setMoreOpen(!moreOpen)} />
    <BoardButton label="Remove from board" icon={Trash2} onClick={remove} />
    {moreOpen && <div className="board-selection-menu rs-surface" role="menu">
      {summary && host.onAskAbout && <button role="menuitem" onClick={() => { host.onAskAbout?.({ artifactId: summary.artifactId ?? undefined, title: summary.title }); setMoreOpen(false); }}>Ask about this</button>}
      {shape?.type === "aladin-doc" && summary?.artifactId && content?.createWorksheet && host.onOpenArtifact && <button role="menuitem" onClick={() => {
        setMoreOpen(false);
        void content.createWorksheet?.({ folderId, title: "Worksheet — " + summary.title, cite: { artifactId: summary.artifactId!, page: summary.page ?? 1, title: summary.title } })
          .then((id) => host.onOpenArtifact?.(id))
          .catch(() => toasts.show({ text: "Couldn't create the worksheet — try again" }));
      }}>Create worksheet</button>}
      <button role="menuitem" onClick={() => perform("bring to front", () => editor.bringToFront(ids))}>Bring to front</button>
      <button role="menuitem" onClick={() => perform("send to back", () => editor.sendToBack(ids))}>Send to back</button>
      <button role="menuitem" onClick={() => perform("toggle lock", () => editor.toggleLock(ids))}>{shape?.isLocked ? "Unlock" : "Lock in place"}</button>
    </div>}
  </div>;
}
