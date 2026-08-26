import { describe, expect, it } from "vitest";

import {
  PAPER_GAP,
  PAPER_PAGE,
  paperCameraOptions,
  paperPageCount,
  parsePaperConfig,
} from "@/modules/board/domain/board-paper";

describe("paper config", () => {
  it("reads paged + cite from board metadata, tolerantly", () => {
    expect(
      parsePaperConfig({
        board: { paper: "paged", cite: { artifactId: "a1", page: 96, title: "Options" } },
      }),
    ).toEqual({ paged: true, cite: { artifactId: "a1", page: 96, title: "Options" } });
    expect(parsePaperConfig({ board: { paper: "paged" } })).toEqual({ paged: true, cite: null });
    expect(parsePaperConfig(undefined).paged).toBe(false);
    expect(parsePaperConfig({ board: { paper: "plane" } }).paged).toBe(false);
    expect(parsePaperConfig({ board: { cite: { page: 3 } } }).cite).toBeNull();
    expect(parsePaperConfig({ board: { paper: "paged", cite: { artifactId: "a", page: -2 } } }).cite)
      .toEqual({ artifactId: "a", page: 1, title: "" });
  });
});

describe("paper geometry", () => {
  it("always offers a blank trailing page and grows with the ink", () => {
    expect(paperPageCount(0)).toBe(2);
    expect(paperPageCount(100)).toBe(2);
    expect(paperPageCount(PAPER_PAGE.h + PAPER_GAP + 10)).toBe(3);
    expect(paperPageCount(3 * (PAPER_PAGE.h + PAPER_GAP) - 5)).toBe(4);
  });

  it("camera bounds cover exactly the pages", () => {
    const constraints = paperCameraOptions(3).constraints!;
    expect(constraints.bounds.w).toBe(PAPER_PAGE.w);
    expect(constraints.bounds.h).toBe(3 * (PAPER_PAGE.h + PAPER_GAP) - PAPER_GAP);
    expect(constraints.initialZoom).toBe("fit-x-100");
  });
});
