import type { StateCreator } from "zustand";

export type ThemeName = "dark" | "soft";

export const THEME_STORAGE_KEY = "aladin.theme";

export function readStoredTheme(): ThemeName {
  try {
    const stored = globalThis.localStorage?.getItem(THEME_STORAGE_KEY);
    return stored === "soft" || stored === "dark" ? stored : "dark";
  } catch {
    return "dark";
  }
}

export interface ThemeSlice {
  theme: ThemeName;
  setTheme: (theme: ThemeName) => void;
  toggleTheme: () => void;
}

export const createThemeSlice: StateCreator<ThemeSlice, [], [], ThemeSlice> = (set, get) => ({
  theme: readStoredTheme(),
  setTheme: (theme) => {
    try {
      globalThis.localStorage?.setItem(THEME_STORAGE_KEY, theme);
    } catch {
      // ignore (private mode / unavailable storage)
    }
    set({ theme });
  },
  toggleTheme: () => get().setTheme(get().theme === "dark" ? "soft" : "dark"),
});
