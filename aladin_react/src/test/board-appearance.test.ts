import { describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { DEFAULT_THEME, type Editor } from "tldraw";
import { BOARD_APPEARANCE_KEY, buildBoardStudioTheme, loadBoardAppearance, saveBoardAppearance, useSavedBoardAppearance, useBoardThemeSync } from "@/modules/board/domain/board-appearance";

function contrast(a: string, b: string) {
  const luminance = (hex: string) => {
    const rgb = [1, 3, 5].map((offset) => {
      const channel = Number.parseInt(hex.slice(offset, offset + 2), 16) / 255;
      return channel <= .04045 ? channel / 12.92 : ((channel + .055) / 1.055) ** 2.4;
    });
    return rgb[0] * .2126 + rgb[1] * .7152 + rgb[2] * .0722;
  };
  const values = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (values[0] + .05) / (values[1] + .05);
}

describe("board studio appearance", () => {
  it("keeps all mounted board panes in the same appearance", () => {
    const first = renderHook(useSavedBoardAppearance);
    const second = renderHook(useSavedBoardAppearance);
    const original = first.result.current.appearance;
    act(() => first.result.current.toggle());
    expect(first.result.current.appearance).not.toBe(original);
    expect(second.result.current.appearance).toBe(first.result.current.appearance);
    act(() => first.result.current.toggle());
  });

  it("defaults to paper and tolerates unavailable preference storage", () => {
    expect(loadBoardAppearance(null)).toBe("light");
    const unavailable = { getItem: () => { throw new Error("blocked"); }, setItem: () => { throw new Error("blocked"); } };
    expect(loadBoardAppearance(unavailable)).toBe("light");
    expect(() => saveBoardAppearance(unavailable, "dark")).not.toThrow();
  });

  it("remembers dark mode and migrates the discarded Aladin-mode preference", () => {
    const storage = { getItem: vi.fn(() => "dark"), setItem: vi.fn() };
    expect(loadBoardAppearance(storage)).toBe("dark");
    saveBoardAppearance(storage, "light");
    expect(storage.setItem).toHaveBeenCalledWith(BOARD_APPEARANCE_KEY, "light");
    storage.getItem.mockReturnValue("aladin");
    expect(loadBoardAppearance(storage)).toBe("dark");
  });

  it("preserves stock and legacy color names in both modes", () => {
    const theme = buildBoardStudioTheme();
    for (const mode of ["light", "dark"] as const) {
      for (const key of Object.keys(DEFAULT_THEME.colors[mode])) expect(theme.colors[mode]).toHaveProperty(key);
      for (const key of ["learn", "amber", "against", "link"]) expect(theme.colors[mode]).toHaveProperty(key);
    }
  });

  it("leaves the approved light paper and note fills unchanged", () => {
    const { light } = buildBoardStudioTheme().colors;
    expect(light.background).toBe("#f6f5f1");
    expect(light.text).toBe("#303632");
    expect(light.selectionStroke).toBe("#727eb7");
    expect(light.yellow.noteFill).toBe("#f5edc8");
    expect(light["light-green"].noteFill).toBe("#e4eadb");
    expect(light["light-violet"].noteFill).toBe("#ece8f2");
  });

  it("keeps dark labels readable on canvas, cards, controls and tinted notes", () => {
    const { dark } = buildBoardStudioTheme().colors;
    expect(dark.background).toBe("#232624");
    for (const surface of [dark.background, dark.white.noteFill, "#363b36"]) {
      expect(contrast(dark.text, surface)).toBeGreaterThanOrEqual(4.5);
      expect(contrast(dark.grey.solid, surface)).toBeGreaterThanOrEqual(4.5);
    }
    for (const name of ["yellow", "light-green", "light-violet"] as const) {
      expect(contrast(dark[name].noteText, dark[name].noteFill)).toBeGreaterThanOrEqual(4.5);
    }
    expect(contrast(dark.selectionStroke, dark.background)).toBeGreaterThanOrEqual(3);
  });

  it("does not follow shell theme changes, but switches the same editor in place", async () => {
    const root = document.documentElement;
    const originalStyle = root.getAttribute("style");
    const originalTheme = root.getAttribute("data-theme");
    const before = buildBoardStudioTheme();
    const editor = { updateTheme: vi.fn(), setColorMode: vi.fn() };
    const view = renderHook(({ appearance }) => useBoardThemeSync(editor as unknown as Editor, appearance), { initialProps: { appearance: "dark" as "light" | "dark" } });
    try {
      const updates = editor.updateTheme.mock.calls.length;
      await act(async () => {
        root.dataset.theme = "apple-light";
        root.style.setProperty("--bg", "#f2f2f7");
        root.style.setProperty("--ink", "#1d1d1f");
        root.style.setProperty("--amber", "#007aff");
      });
      expect(buildBoardStudioTheme()).toEqual(before);
      expect(editor.updateTheme).toHaveBeenCalledTimes(updates);
      expect(editor.setColorMode).toHaveBeenLastCalledWith("dark");
      view.rerender({ appearance: "light" });
      expect(editor.setColorMode).toHaveBeenLastCalledWith("light");
      expect(editor.updateTheme.mock.calls.at(-1)?.[0]).toEqual(before);
    } finally {
      view.unmount();
      if (originalStyle === null) root.removeAttribute("style"); else root.setAttribute("style", originalStyle);
      if (originalTheme === null) root.removeAttribute("data-theme"); else root.setAttribute("data-theme", originalTheme);
    }
  });
});
