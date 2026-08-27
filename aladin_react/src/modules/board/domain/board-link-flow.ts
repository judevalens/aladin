import type { Editor, TLShapeId } from "tldraw";

import type { BoardContentSource } from "./board-content";
import { unfurlFailedPatch, unfurlPatch } from "./board-links";
import type { LinkShape } from "../shapes/shape-types";

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
  const patch = (props: Partial<LinkShape["props"]>) => {
    if (editor.isDisposed || !editor.getShape(id)) return;
    editor.updateShape({ id, type: "aladin-link", props });
  };
  if (!content?.unfurl) {
    // No backend (the bare spike): the bare-URL rendering is the whole preview.
    patch(unfurlFailedPatch(url));
    return;
  }
  content
    .unfurl(url)
    .then((result) => patch(unfurlPatch(result)))
    .catch(() => patch(unfurlFailedPatch(url)));
}
