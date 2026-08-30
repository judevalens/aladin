import { createContext, useContext, useEffect, useSyncExternalStore } from "react";
import type { Editor } from "tldraw";
import { buildBoardTheme, makeInkColor } from "./board-theme";
import { browserPrefsStorage, type PrefsStorage } from "./board-prefs";

export type BoardAppearance = "light" | "dark";
export const BOARD_APPEARANCE_KEY = "aladin.board.appearance";
export function loadBoardAppearance(storage: PrefsStorage | null): BoardAppearance {
  try {
    const saved = storage?.getItem(BOARD_APPEARANCE_KEY);
    // Preserve the alternate-mode choice from the short-lived shell-theme experiment.
    return saved === "dark" || saved === "aladin" ? "dark" : "light";
  } catch { return "light"; }
}
export function saveBoardAppearance(storage: PrefsStorage | null, appearance: BoardAppearance) {
  try { storage?.setItem(BOARD_APPEARANCE_KEY, appearance); } catch { /* Optional preference. */ }
}

// All mounted boards share a board preference, independent of Aladin's shell theme.
let currentAppearance = loadBoardAppearance(browserPrefsStorage());
const listeners = new Set<() => void>();
function subscribeAppearance(listener: () => void) {
  listeners.add(listener);
  return () => { listeners.delete(listener); };
}
export function useSavedBoardAppearance() {
  const appearance = useSyncExternalStore(subscribeAppearance, () => currentAppearance, () => "light" as const);
  return {
    appearance,
    toggle: () => {
      currentAppearance = currentAppearance === "light" ? "dark" : "light";
      saveBoardAppearance(browserPrefsStorage(), currentAppearance);
      listeners.forEach((listener) => listener());
    },
  };
}

export const BoardAppearanceContext = createContext({ appearance: "light" as BoardAppearance, toggle: () => {} });
export const useBoardAppearance = () => useContext(BoardAppearanceContext);

/** Update the existing editor; never change the prop that recreates it and loses undo. */
export function useBoardThemeSync(editor: Editor | null, appearance: BoardAppearance) {
  useEffect(() => {
    if (!editor) return;
    editor.updateTheme(buildBoardStudioTheme());
    editor.setColorMode(appearance);
  }, [editor, appearance]);
}

// Freeze the legacy palette inputs so even saved custom ink stays independent of the shell.
// The light branch retains the existing approved paper palette.
const BOARD_TOKENS: Record<string, string> = {
  "--bg": "#232624", "--ink": "#e8ebe3", "--amber": "#c9925a",
  "--against": "#d8796b", "--board-learn": "#8fb9e8",
};

/** Both appearances retain every stock and legacy color name: no saved-shape migration. */
export function buildBoardStudioTheme() {
  const legacy = buildBoardTheme((name) => BOARD_TOKENS[name] ?? "#000000");
  const light = legacy.colors.light;
  const dark = legacy.colors.dark;
  const ink = "#e8ebe3";
  return {
    ...legacy,
    fonts: { ...legacy.fonts, sans: { fontFamily: "'Geist Variable', system-ui, sans-serif" } },
    colors: {
      light: {
        ...light, background: "#f6f5f1", text: "#303632", selectionStroke: "#727eb7",
        black: { ...light.black, solid: "#3a423e", frameFill: "#eeeee880", frameStroke: "#dbddd2", frameText: "#757d70" },
        yellow: { ...light.yellow, noteFill: "#f5edc8", noteText: "#333b35" },
        "light-green": { ...light["light-green"], noteFill: "#e4eadb", noteText: "#333b35" },
        "light-violet": { ...light["light-violet"], noteFill: "#ece8f2", noteText: "#333b35" },
        learn: { ...light.learn, solid: "#648ac0" },
        link: { ...light.link, solid: "#8a9188" },
      },
      dark: {
        ...dark, background: "#232624", text: ink, selectionStroke: "#a8b8db",
        brushFill: "#a8b8db14", brushStroke: "#a8b8db80",
        black: { ...dark.black, solid: ink, frameFill: "#292e2880", frameStroke: "#4c5549", frameHeadingFill: "#232624", frameHeadingStroke: "#4c5549", frameText: "#b4bcae", noteFill: "#303530", noteText: ink },
        grey: { ...dark.grey, solid: "#b4bcae" },
        white: { ...dark.white, noteFill: "#303530", noteText: ink },
        yellow: { ...dark.yellow, noteFill: "#514a32", noteText: ink },
        "light-green": { ...dark["light-green"], noteFill: "#3b4b38", noteText: ink },
        "light-violet": { ...dark["light-violet"], noteFill: "#494151", noteText: ink },
        blue: { ...dark.blue, solid: "#91b2da" },
        green: { ...dark.green, solid: "#a3c288" },
        violet: { ...dark.violet, solid: "#baa3d3" },
        orange: { ...dark.orange, solid: "#d8b982" },
        learn: makeInkColor("#91b2da", "#232624", ink),
        amber: makeInkColor("#d8b982", "#232624", ink),
        against: makeInkColor("#dc988a", "#232624", ink),
        link: makeInkColor("#adb9a3", "#232624", ink),
      },
    },
  };
}
