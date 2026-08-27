import { Box, createShapeId } from "tldraw";
import type { Editor, TLShapeId, VecLike } from "tldraw";

import {
  CARD_DEFAULTS,
  DOC_WINDOW_DEFAULTS,
  EXCERPT_DEFAULTS,
  LINK_DEFAULTS,
  TASK_DEFAULTS,
} from "../shapes/shape-types";
import { linkDomain } from "./board-links";

/**
 * Object creation — every insert lands selected, in the select tool, so the selection bar
 * appears and a drag immediately moves the new object (the handoff's insert behavior).
 */

/** Gap kept between a placed object and its neighbours, in page units. */
const FREE_GAP = 16;
/** Step of the search spiral, in page units. */
const FREE_STEP = 40;
const FREE_MAX_RINGS = 24;

/**
 * The first rect of `w × h` near `start` that overlaps none of `occupied` — searched on a
 * square spiral (right, down, left, up…) so the object lands as close to where you were
 * looking as the board allows. Pure, so the rule is testable without an editor. Falls back
 * to `start` when the board is packed that far out.
 */
export function findFreeRect(
  occupied: readonly Box[],
  start: VecLike,
  w: number,
  h: number,
): VecLike {
  const clear = (x: number, y: number) => {
    const candidate = new Box(x - FREE_GAP, y - FREE_GAP, w + 2 * FREE_GAP, h + 2 * FREE_GAP);
    return !occupied.some((box) => candidate.collides(box));
  };
  if (clear(start.x, start.y)) return { x: start.x, y: start.y };
  for (let ring = 1; ring <= FREE_MAX_RINGS; ring++) {
    const r = ring * FREE_STEP;
    // Walk the ring's perimeter: right edge top→bottom, bottom edge, left edge, top edge.
    for (let k = -ring; k <= ring; k++) {
      const d = k * FREE_STEP;
      const points = [
        { x: start.x + r, y: start.y + d },
        { x: start.x - r, y: start.y + d },
        { x: start.x + d, y: start.y + r },
        { x: start.x + d, y: start.y - r },
      ];
      for (const p of points) if (clear(p.x, p.y)) return p;
    }
  }
  return { x: start.x, y: start.y };
}

/** The handoff's "cascade near free space": free room, searched out from the viewport centre. */
export function freePoint(editor: Editor, w: number, h: number): VecLike {
  const center = editor.getViewportPageBounds().center;
  const occupied = editor
    .getCurrentPageShapes()
    .map((shape) => editor.getShapePageBounds(shape))
    .filter((box): box is Box => box !== undefined);
  return findFreeRect(occupied, { x: center.x - w / 2, y: center.y - h / 2 }, w, h);
}

function insert(
  editor: Editor,
  type: "aladin-doc" | "aladin-excerpt" | "aladin-task" | "aladin-card" | "aladin-link",
  at: VecLike | undefined,
  props: Record<string, unknown>,
): TLShapeId {
  const id = createShapeId();
  const size = props as { w?: number; h?: number };
  const point = at ?? freePoint(editor, size.w ?? 300, size.h ?? 120);
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

/** A link object lands `pending` showing its domain; the caller starts the unfurl. */
export function addLink(editor: Editor, opts: { url: string; at?: VecLike }): TLShapeId {
  return insert(editor, "aladin-link", opts.at, {
    ...LINK_DEFAULTS,
    url: opts.url,
    domain: linkDomain(opts.url),
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
