import type { Editor, TLShapeId } from "tldraw";

import type { BoardContentSource } from "./board-content";
import { unfurlFailedPatch, unfurlPatch } from "./board-links";
import type { LinkShape } from "../shapes/shape-types";

const requests = new WeakMap<Editor, Map<TLShapeId, symbol>>();

/**
 * Resolve a pending link object: unfurl through the host's API plane and patch the shape.
 * Whoever pasted does the one fetch; every other client just receives the synced props.
 * The patch is skipped when the shape is gone (deleted mid-flight) or the editor was
 * disposed (tab closed) — an update against either throws.
 */
export function resolveLinkInto(
  editor: Editor,
  content: BoardContentSource | null,
  id: TLShapeId,
  url: string,
): void {
  let pending = requests.get(editor);
  if (!pending) { pending = new Map(); requests.set(editor, pending); }
  const request = Symbol();
  pending.set(id, request);
  const patch = (props: Partial<LinkShape["props"]>) => {
    const current = editor.isDisposed ? null : editor.getShape<LinkShape>(id);
    if (!current || current.type !== "aladin-link" || current.props.url !== url || pending.get(id) !== request) return;
    // Refreshing should not undo a person's resize or let an older response win.
    editor.updateShape({ id, type: "aladin-link", props: { ...props, ...(props.h ? { h: Math.max(current.props.h, props.h) } : {}) } });
  };
  if (!content?.unfurl) {
    // No backend (the bare spike): the bare-URL rendering is the whole preview.
    patch(unfurlFailedPatch(url));
    pending.delete(id);
    return;
  }
  patch({ status: "pending" });
  content
    .unfurl(url)
    .then((result) => patch(unfurlPatch(result)))
    .catch(() => patch(unfurlFailedPatch(url)))
    .finally(() => { if (pending.get(id) === request) pending.delete(id); });
}
