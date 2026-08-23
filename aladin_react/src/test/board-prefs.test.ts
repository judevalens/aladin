import { describe, expect, it } from "vitest";

import {
  BOARD_PREFS_KEY,
  DEFAULT_BOARD_PREFS,
  loadBoardPrefs,
  saveBoardPrefs,
} from "@/modules/board/domain/board-prefs";

function memoryStorage(seed: Record<string, string> = {}) {
  const map = new Map(Object.entries(seed));
  return {
    getItem: (k: string) => map.get(k) ?? null,
    setItem: (k: string, v: string) => void map.set(k, v),
  };
}

describe("board prefs", () => {
  it("round-trips the pencil's setup", () => {
    const storage = memoryStorage();
    saveBoardPrefs(storage, { subTool: "lasso", inkColor: "against", weight: 2, drawWithFinger: true });
    expect(loadBoardPrefs(storage)).toEqual({
      subTool: "lasso",
      inkColor: "against",
      weight: 2,
      drawWithFinger: true,
    });
  });

  it("falls back field by field on anything odd, never throwing", () => {
    const storage = memoryStorage({
      [BOARD_PREFS_KEY]: JSON.stringify({ subTool: "crayon", inkColor: "amber", weight: 7 }),
    });
    expect(loadBoardPrefs(storage)).toEqual({ ...DEFAULT_BOARD_PREFS, inkColor: "amber" });
    expect(loadBoardPrefs(memoryStorage({ [BOARD_PREFS_KEY]: "{not json" }))).toEqual(DEFAULT_BOARD_PREFS);
    expect(loadBoardPrefs(null)).toEqual(DEFAULT_BOARD_PREFS);
  });
});
