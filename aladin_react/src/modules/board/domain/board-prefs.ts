import type { BoardInkColor } from "./board-theme";
import type { BoardWeightIndex, PencilSubTool } from "./board-tools";

/**
 * The pencil's remembered setup — sub-tool, ink, weight, and whether a finger may draw.
 * One record for the whole app, not per board: you pick up the pen you put down
 * (GoodNotes, Concepts). Lives in localStorage; survives a reload, a pane switch and the
 * iPad's web process restarting.
 */
export interface BoardToolPrefs {
  subTool: PencilSubTool;
  inkColor: BoardInkColor;
  weight: BoardWeightIndex;
  /**
   * Off (default): once a Pencil has touched the glass, a finger never draws in the pencil
   * tools — it pans. On: a finger draws too (for people without a Pencil, Freeform's
   * "Draw with Finger").
   */
  drawWithFinger: boolean;
}

export const BOARD_PREFS_KEY = "aladin.board.tools";

export const DEFAULT_BOARD_PREFS: BoardToolPrefs = {
  subTool: "pen",
  inkColor: "learn",
  weight: 1,
  drawWithFinger: false,
};

const SUB_TOOLS = new Set<PencilSubTool>(["pen", "highlighter", "eraser", "lasso"]);
const INKS = new Set<BoardInkColor>(["learn", "amber", "against"]);

/** A storage the size of what we use — `localStorage` or a test map. */
export interface PrefsStorage {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
}

/** Reads the prefs, field by field, falling back per field to the default on anything odd. */
export function loadBoardPrefs(storage: PrefsStorage | null | undefined): BoardToolPrefs {
  if (!storage) return DEFAULT_BOARD_PREFS;
  try {
    const raw = storage.getItem(BOARD_PREFS_KEY);
    if (!raw) return DEFAULT_BOARD_PREFS;
    const parsed = JSON.parse(raw) as Partial<Record<keyof BoardToolPrefs, unknown>>;
    return {
      subTool: SUB_TOOLS.has(parsed.subTool as PencilSubTool)
        ? (parsed.subTool as PencilSubTool)
        : DEFAULT_BOARD_PREFS.subTool,
      inkColor: INKS.has(parsed.inkColor as BoardInkColor)
        ? (parsed.inkColor as BoardInkColor)
        : DEFAULT_BOARD_PREFS.inkColor,
      weight:
        parsed.weight === 0 || parsed.weight === 1 || parsed.weight === 2
          ? parsed.weight
          : DEFAULT_BOARD_PREFS.weight,
      drawWithFinger:
        typeof parsed.drawWithFinger === "boolean"
          ? parsed.drawWithFinger
          : DEFAULT_BOARD_PREFS.drawWithFinger,
    };
  } catch {
    return DEFAULT_BOARD_PREFS;
  }
}

export function saveBoardPrefs(storage: PrefsStorage | null | undefined, prefs: BoardToolPrefs) {
  try {
    storage?.setItem(BOARD_PREFS_KEY, JSON.stringify(prefs));
  } catch {
    // Private mode / quota — the prefs are a convenience, never a failure.
  }
}

/** The browser's localStorage, or null where there is none (SSR, a locked-down web view). */
export function browserPrefsStorage(): PrefsStorage | null {
  try {
    return typeof window !== "undefined" ? window.localStorage : null;
  } catch {
    return null;
  }
}
