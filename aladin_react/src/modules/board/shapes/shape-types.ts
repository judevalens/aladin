import { T } from "tldraw";
import type { RecordProps, TLBaseShape } from "tldraw";

/**
 * The board's object types, per the handoff:
 *
 * - `aladin-doc`     a live window onto a workspace artifact. Body text is NOT stored —
 *                    it resolves read-live from the artifact by (artifactId, page); only
 *                    the window's own view state (page) persists. `frozen` is
 *                    schema-ready but has no UI yet (deferred).
 * - `aladin-excerpt` a frozen quote — text IS stored, with its citation.
 * - `aladin-task`    checkbox + handwriting line.
 * - `aladin-card`    a flashcard; tap flips, no scheduler (product rule 5).
 * - `aladin-link`    an external URL with its unfurled preview (title/description/domain/
 *                    image, resolved by POST /api/unfurl). Self-contained like an excerpt —
 *                    no artifact row behind it; the MCP board summary reads these props.
 *
 * Every custom shape type must ALSO be declared into TLGlobalShapePropsMap below — without
 * the augmentation, ShapeUtil rejects the type with a misleading "not assignable to
 * TLBookmarkShape" error.
 */

export interface DocWindowProps {
  w: number;
  h: number;
  artifactId: string;
  /** The artifact's kind string ("file" | "note" | "link" …) — picks the header icon. */
  artifactKind: string;
  /** Cached label; refreshed whenever the artifact resolves. */
  title: string;
  /** THIS window's page — per-window view state, never per-artifact. */
  page: number;
  pageCount: number;
  frozen: boolean;
}

export interface ExcerptProps {
  w: number;
  h: number;
  text: string;
  sourceArtifactId: string | null;
  sourceTitle: string;
  page: number | null;
}

export interface TaskProps {
  w: number;
  h: number;
  text: string;
  meta: string;
  checked: boolean;
}

export interface LinkProps {
  w: number;
  h: number;
  url: string;
  /** Unfurled metadata; empty string = not resolved (or absent). */
  title: string;
  description: string;
  domain: string;
  image: string;
  favicon: string;
  /** pending = unfurl in flight · ready = resolved · failed = showing the bare URL. */
  status: "pending" | "ready" | "failed";
}

export interface CardProps {
  w: number;
  h: number;
  front: string;
  back: string;
  cite: string;
  flipped: boolean;
}

export type DocWindowShape = TLBaseShape<"aladin-doc", DocWindowProps>;
export type ExcerptShape = TLBaseShape<"aladin-excerpt", ExcerptProps>;
export type TaskShape = TLBaseShape<"aladin-task", TaskProps>;
export type CardShape = TLBaseShape<"aladin-card", CardProps>;
export type LinkShape = TLBaseShape<"aladin-link", LinkProps>;

declare module "@tldraw/tlschema" {
  interface TLGlobalShapePropsMap {
    "aladin-doc": DocWindowProps;
    "aladin-excerpt": ExcerptProps;
    "aladin-task": TaskProps;
    "aladin-card": CardProps;
    "aladin-link": LinkProps;
  }
}

export const docWindowProps: RecordProps<DocWindowShape> = {
  w: T.nonZeroNumber,
  h: T.nonZeroNumber,
  artifactId: T.string,
  artifactKind: T.string,
  title: T.string,
  page: T.number,
  pageCount: T.number,
  frozen: T.boolean,
};

export const excerptProps: RecordProps<ExcerptShape> = {
  w: T.nonZeroNumber,
  h: T.nonZeroNumber,
  text: T.string,
  sourceArtifactId: T.string.nullable(),
  sourceTitle: T.string,
  page: T.number.nullable(),
};

export const taskProps: RecordProps<TaskShape> = {
  w: T.nonZeroNumber,
  h: T.nonZeroNumber,
  text: T.string,
  meta: T.string,
  checked: T.boolean,
};

export const cardProps: RecordProps<CardShape> = {
  w: T.nonZeroNumber,
  h: T.nonZeroNumber,
  front: T.string,
  back: T.string,
  cite: T.string,
  flipped: T.boolean,
};

export const linkProps: RecordProps<LinkShape> = {
  w: T.nonZeroNumber,
  h: T.nonZeroNumber,
  url: T.string,
  title: T.string,
  description: T.string,
  domain: T.string,
  image: T.string,
  favicon: T.string,
  status: T.literalEnum("pending", "ready", "failed"),
};

/** Default rects, from the handoff's reference frames. */
export const DOC_WINDOW_DEFAULTS: DocWindowProps = {
  w: 364,
  h: 304,
  artifactId: "",
  artifactKind: "file",
  title: "",
  page: 1,
  pageCount: 1,
  frozen: false,
};

export const EXCERPT_DEFAULTS: ExcerptProps = {
  w: 304,
  h: 128,
  text: "",
  sourceArtifactId: null,
  sourceTitle: "unsourced",
  page: null,
};

export const TASK_DEFAULTS: TaskProps = {
  w: 364,
  h: 112,
  text: "New task",
  meta: "open",
  checked: false,
};

export const CARD_DEFAULTS: CardProps = {
  w: 304,
  h: 206,
  front: "Front of the card",
  back: "Back of the card",
  cite: "unsourced",
  flipped: false,
};

export const LINK_DEFAULTS: LinkProps = {
  w: 304,
  h: 128,
  url: "",
  title: "",
  description: "",
  domain: "",
  image: "",
  favicon: "",
  status: "pending",
};
