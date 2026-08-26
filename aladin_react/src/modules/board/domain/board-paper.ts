import { createContext, useContext } from "react";
import type { TLCameraOptions } from "tldraw";

import { BOARD_ZOOM_STEPS } from "./board-camera";

/**
 * Paper — the board engine wearing a page costume. A worksheet is a BOARD artifact whose
 * `metadata.board.paper === "paged"`: same shapes, same rooms, same chrome family, but the
 * camera is clamped to a fixed-width column of pages and the pencil is the default tool.
 * Deliberately a property, not an artifact kind: a third kind would grow a case in every
 * downstream system for what is, mechanically, a camera setting (STUDY_PRD §4).
 */

/** One page of paper, in page units. Roughly A4 at a comfortable ink scale. */
export const PAPER_PAGE = { w: 820, h: 1160 } as const;
export const PAPER_GAP = 28;

export interface PaperCite {
  artifactId: string;
  page: number;
  title: string;
}

export interface PaperConfig {
  paged: boolean;
  /** The exercise this worksheet was spawned from — its header chip wormholes back. */
  cite: PaperCite | null;
}

export const PLAIN_PAPER: PaperConfig = { paged: false, cite: null };

/** Tolerant read of `artifact.metadata.board` — junk shapes degrade to a plain board. */
export function parsePaperConfig(metadata: unknown): PaperConfig {
  if (!metadata || typeof metadata !== "object") return PLAIN_PAPER;
  const board = (metadata as { board?: unknown }).board;
  if (!board || typeof board !== "object") return PLAIN_PAPER;
  const paged = (board as { paper?: unknown }).paper === "paged";
  const rawCite = (board as { cite?: unknown }).cite;
  let cite: PaperCite | null = null;
  if (rawCite && typeof rawCite === "object") {
    const c = rawCite as { artifactId?: unknown; page?: unknown; title?: unknown };
    if (typeof c.artifactId === "string" && c.artifactId) {
      cite = {
        artifactId: c.artifactId,
        page: typeof c.page === "number" && c.page >= 1 ? c.page : 1,
        title: typeof c.title === "string" ? c.title : "",
      };
    }
  }
  return { paged, cite };
}

/**
 * How many pages the paper shows: enough to hold the ink plus ONE blank trailing page, so
 * there is always somewhere to keep writing and the paper grows as you do. Never fewer
 * than two (a fresh worksheet shows a page and the promise of the next).
 */
export function paperPageCount(maxShapeBottom: number): number {
  const used = Math.max(0, maxShapeBottom);
  return Math.max(2, Math.ceil(used / (PAPER_PAGE.h + PAPER_GAP)) + 1);
}

/** The paged camera: a fixed-width column, fit-to-width (max 100%), vertical scroll. */
export function paperCameraOptions(pageCount: number): Partial<TLCameraOptions> {
  return {
    zoomSteps: BOARD_ZOOM_STEPS,
    wheelBehavior: "pan",
    constraints: {
      bounds: {
        x: 0,
        y: 0,
        w: PAPER_PAGE.w,
        h: pageCount * (PAPER_PAGE.h + PAPER_GAP) - PAPER_GAP,
      },
      padding: { x: 32, y: 32 },
      origin: { x: 0.5, y: 0 },
      initialZoom: "fit-x-100",
      baseZoom: "fit-x-100",
      behavior: { x: "contain", y: "contain" },
    },
  };
}

export const BoardPaperContext = createContext<PaperConfig>(PLAIN_PAPER);

export function useBoardPaper(): PaperConfig {
  return useContext(BoardPaperContext);
}
