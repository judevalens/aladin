import { describe, expect, it } from "vitest";

import {
  BOARD_TOOL_IDS,
  boardToolFromTldraw,
  isBoardToolId,
  tldrawToolId,
} from "@/modules/board/domain/board-tools";

describe("board tools ↔ tldraw tool ids", () => {
  it("round-trips every dock tool through tldraw's id", () => {
    expect(boardToolFromTldraw(tldrawToolId("select", "pen"))).toEqual({ tool: "select", subTool: null });
    expect(boardToolFromTldraw(tldrawToolId("arrow", "pen"))).toEqual({ tool: "arrow", subTool: null });
    for (const sub of ["pen", "highlighter", "eraser", "lasso"] as const) {
      expect(boardToolFromTldraw(tldrawToolId("pencil", sub))).toEqual({ tool: "pencil", subTool: sub });
    }
  });

  it("models exactly the six tools the dock can show", () => {
    expect([...BOARD_TOOL_IDS].sort()).toEqual(
      ["arrow", "draw", "eraser", "highlight", "lasso", "select"].sort(),
    );
  });

  it("rejects tldraw's shortcut-only tools so the chrome can snap back to select", () => {
    for (const id of ["frame", "note", "geo", "hand", "laser", "text", "zoom"]) {
      expect(isBoardToolId(id)).toBe(false);
    }
    expect(isBoardToolId("draw")).toBe(true);
  });
});
