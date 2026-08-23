import { createShapeId } from "tldraw";
import type { Editor, TLShapeId, VecLike } from "tldraw";

import {
  CARD_DEFAULTS,
  DOC_WINDOW_DEFAULTS,
  EXCERPT_DEFAULTS,
  TASK_DEFAULTS,
} from "../shapes/shape-types";

/**
 * Object creation — every insert lands selected, in the select tool, so the selection bar
 * appears and a drag immediately moves the new object (the handoff's insert behavior).
 */

/** Cascade near the viewport center, stepped by how many objects the board already has. */
export function cascadePoint(editor: Editor): VecLike {
  const center = editor.getViewportPageBounds().center;
  const k = editor.getCurrentPageShapes().length % 4;
  return { x: center.x - 150 + k * 26, y: center.y - 80 + k * 26 };
}

function insert(
  editor: Editor,
  type: "aladin-doc" | "aladin-excerpt" | "aladin-task" | "aladin-card",
  at: VecLike | undefined,
  props: Record<string, unknown>,
): TLShapeId {
  const id = createShapeId();
  const point = at ?? cascadePoint(editor);
  editor.createShape({ id, type, x: point.x, y: point.y, props });
  editor.setCurrentTool("select");
  editor.select(id);
  return id;
}

export function addExcerpt(
  editor: Editor,
  opts: {
    text: string;
    sourceArtifactId?: string | null;
    sourceTitle?: string;
    page?: number | null;
    at?: VecLike;
  },
): TLShapeId {
  return insert(editor, "aladin-excerpt", opts.at, {
    ...EXCERPT_DEFAULTS,
    text: opts.text,
    sourceArtifactId: opts.sourceArtifactId ?? null,
    sourceTitle: opts.sourceTitle ?? EXCERPT_DEFAULTS.sourceTitle,
    page: opts.page ?? null,
  });
}

export function addTask(editor: Editor, at?: VecLike): TLShapeId {
  const id = insert(editor, "aladin-task", at, { ...TASK_DEFAULTS });
  editor.setEditingShape(id);
  return id;
}

export function addCard(editor: Editor, at?: VecLike): TLShapeId {
  const id = insert(editor, "aladin-card", at, { ...CARD_DEFAULTS });
  editor.setEditingShape(id);
  return id;
}

export function addDocWindow(
  editor: Editor,
  opts: {
    artifactId: string;
    artifactKind: string;
    title: string;
    page?: number;
    pageCount?: number;
    at?: VecLike;
  },
): TLShapeId {
  return insert(editor, "aladin-doc", opts.at, {
    ...DOC_WINDOW_DEFAULTS,
    artifactId: opts.artifactId,
    artifactKind: opts.artifactKind,
    title: opts.title,
    page: opts.page ?? 1,
    pageCount: opts.pageCount ?? 1,
  });
}

/** artifactId → shapeId for every live window already on this board ("on this board"). */
export function boardArtifactIds(editor: Editor): Map<string, TLShapeId> {
  const map = new Map<string, TLShapeId>();
  for (const shape of editor.getCurrentPageShapes()) {
    if (shape.type === "aladin-doc") {
      const artifactId = (shape.props as { artifactId: string }).artifactId;
      if (artifactId) map.set(artifactId, shape.id);
    }
  }
  return map;
}
