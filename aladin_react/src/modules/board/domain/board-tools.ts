import type { BoardInkColor } from "./board-theme";

/**
 * The dock's tool model, mapped onto tldraw's tools.
 *
 * The dock shows three main tools (Select / Pencil / Arrow); Pencil fans out into four
 * sub-tools that ARE separate tldraw tools. tldraw's `getCurrentToolId()` stays the single
 * source of truth — the dock derives its lit state from it and only remembers which pencil
 * sub-tool to return to.
 */

export type BoardTool = "select" | "pencil" | "arrow";
export type PencilSubTool = "pen" | "highlighter" | "eraser" | "lasso";

/** Stroke weight index → tldraw size style + the dock's dot diameter. */
export const BOARD_WEIGHTS = [
  { size: "s", dotPx: 5 },
  { size: "m", dotPx: 8 },
  { size: "l", dotPx: 11 },
] as const;
export type BoardWeightIndex = 0 | 1 | 2;

const SUB_TOOL_IDS: Record<PencilSubTool, string> = {
  pen: "draw",
  highlighter: "highlight",
  eraser: "eraser",
  lasso: "lasso",
};

/** The tldraw tool id the dock should activate. */
export function tldrawToolId(tool: BoardTool, subTool: PencilSubTool): string {
  if (tool === "select") return "select";
  if (tool === "arrow") return "arrow";
  return SUB_TOOL_IDS[subTool];
}

/** The dock state implied by whatever tool tldraw currently runs (Escape, shortcuts…). */
export function boardToolFromTldraw(toolId: string): {
  tool: BoardTool;
  subTool: PencilSubTool | null;
} {
  if (toolId === "arrow") return { tool: "arrow", subTool: null };
  const subTool = (Object.keys(SUB_TOOL_IDS) as PencilSubTool[]).find(
    (key) => SUB_TOOL_IDS[key] === toolId,
  );
  if (subTool) return { tool: "pencil", subTool };
  return { tool: "select", subTool: null };
}

/** Hint-pill copy per pencil sub-tool — verbatim from the handoff. */
export const PENCIL_HINTS: Record<PencilSubTool, string> = {
  pen: "Pencil holds the board at 100% — a stroke is always a stroke",
  highlighter: "Highlighter — marks ink and cards, sits under everything",
  eraser: "Eraser — scrub a stroke to remove it",
  lasso: "Lasso ink to move it — or keep it as a task, card or note",
};

export const BOARD_INK_SWATCHES: { id: BoardInkColor; label: string; cssVar: string }[] = [
  { id: "learn", label: "Thinking ink", cssVar: "var(--board-learn)" },
  { id: "amber", label: "Amber", cssVar: "var(--amber)" },
  { id: "against", label: "Against", cssVar: "var(--against)" },
];
