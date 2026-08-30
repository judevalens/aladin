import type { BoardInkColor } from "./board-theme";

/**
 * The dock's tool model, mapped onto tldraw's tools.
 *
 * The rail exposes selection, pan, creation, and drawing; Pencil fans out into four
 * sub-tools that are separate tldraw tools. tldraw's `getCurrentToolId()` stays the single
 * source of truth — the dock derives its lit state from it and only remembers which pencil
 * sub-tool to return to.
 */

export type BoardTool = "select" | "pencil" | "arrow" | "hand" | "text" | "frame" | "note";
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

/**
 * Every tldraw tool represented by the rail. The chrome returns unsupported tools
 * to selection so a stock shortcut cannot silently enter an unrepresented mode.
 */
export const BOARD_TOOL_IDS: ReadonlySet<string> = new Set([
  "select",
  "arrow",
  "hand", "text", "frame", "note",
  ...Object.values(SUB_TOOL_IDS),
]);

export function isBoardToolId(toolId: string): boolean {
  return BOARD_TOOL_IDS.has(toolId);
}

/** The tldraw tool id the dock should activate. */
export function tldrawToolId(tool: BoardTool, subTool: PencilSubTool): string {
  return tool === "pencil" ? SUB_TOOL_IDS[subTool] : tool;
}

/** The dock state implied by whatever tool tldraw currently runs (Escape, shortcuts…). */
export function boardToolFromTldraw(toolId: string): {
  tool: BoardTool;
  subTool: PencilSubTool | null;
} {
  if (toolId === "arrow") return { tool: "arrow", subTool: null };
  if (toolId === "hand" || toolId === "text" || toolId === "frame" || toolId === "note") return { tool: toolId, subTool: null };
  const subTool = (Object.keys(SUB_TOOL_IDS) as PencilSubTool[]).find(
    (key) => SUB_TOOL_IDS[key] === toolId,
  );
  if (subTool) return { tool: "pencil", subTool };
  return { tool: "select", subTool: null };
}

export const BOARD_INK_SWATCHES: { id: BoardInkColor; label: string; cssVar: string }[] = [
  { id: "learn", label: "Thinking ink", cssVar: "var(--board-learn)" },
  { id: "amber", label: "Amber", cssVar: "var(--amber)" },
  { id: "against", label: "Against", cssVar: "var(--against)" },
];
