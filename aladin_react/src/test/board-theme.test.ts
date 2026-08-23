import { describe, expect, it } from "vitest";
import { DEFAULT_THEME } from "tldraw";

import {
  BOARD_INK_COLORS,
  blend,
  buildBoardTheme,
  makeInkColor,
} from "@/modules/board/domain/board-theme";

const reader = (name: string) =>
  ({
    "--bg": "#0d0d10",
    "--ink": "#eceaef",
    "--amber": "#c9925a",
    "--against": "#d8796b",
    "--board-learn": "#8fb9e8",
  })[name] ?? "#000000";

describe("buildBoardTheme", () => {
  it("keeps every stock color name in both modes", () => {
    // Load-bearing: tldraw's registerColorsFromThemes REMOVES any registered color that is
    // absent from all provided themes. Dropping a default here would make every board
    // saved with the stock palette fail validation on load.
    const theme = buildBoardTheme(reader);
    for (const mode of ["dark", "light"] as const) {
      for (const name of Object.keys(DEFAULT_THEME.colors[mode])) {
        expect(theme.colors[mode], `${mode}.${name}`).toHaveProperty(name);
      }
    }
  });

  it("registers the board ink colors and the link color in both modes", () => {
    const theme = buildBoardTheme(reader);
    for (const mode of ["dark", "light"] as const) {
      for (const name of [...BOARD_INK_COLORS, "link"]) {
        expect(theme.colors[mode], `${mode}.${name}`).toHaveProperty(name);
      }
    }
  });

  it("maps the Aladin tokens onto the dark palette", () => {
    const theme = buildBoardTheme(reader);
    expect(theme.colors.dark.background).toBe("#0d0d10");
    expect(theme.colors.dark.text).toBe("#eceaef");
    expect(theme.colors.dark.selectionStroke).toBe("#c9925a");
    expect(theme.colors.dark.learn.solid).toBe("#8fb9e8");
    expect(theme.colors.dark.amber.solid).toBe("#c9925a");
    expect(theme.colors.dark.against.solid).toBe("#d8796b");
    // The link arrows are amber at .45, per the handoff.
    expect(theme.colors.dark.link.solid).toBe("rgba(201,146,90,0.45)");
  });

  it("assigns the handoff font stacks", () => {
    const theme = buildBoardTheme(reader);
    expect(theme.fonts.draw.fontFamily).toContain("Caveat");
    expect(theme.fonts.sans.fontFamily).toContain("Space Grotesk");
    expect(theme.fonts.serif.fontFamily).toContain("Georgia");
    expect(theme.fonts.mono.fontFamily).toContain("JetBrains Mono");
  });
});

describe("color derivation", () => {
  it("blend interpolates in RGB", () => {
    expect(blend("#000000", "#ffffff", 0.5)).toBe("#808080");
    expect(blend("#0d0d10", "#0d0d10", 1)).toBe("#0d0d10");
  });

  it("makeInkColor fills every TLDefaultColor variant", () => {
    const color = makeInkColor("#8fb9e8", "#0d0d10", "#eceaef");
    const sample = DEFAULT_THEME.colors.dark.blue;
    for (const key of Object.keys(sample)) {
      expect(color, key).toHaveProperty(key);
      expect((color as unknown as Record<string, string>)[key]).toBeTruthy();
    }
  });
});
