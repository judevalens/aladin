import type { StateCreator } from "zustand";

export type ThemeName =
  | "dark"
  | "soft"
  | "cool"
  | "contrast"
  | "linear"
  | "apple-dark"
  | "apple-light"
  | "light";

// The theme catalog — the single source for the picker UI and the cycle order. `family` drives
// nothing in CSS (data-theme does that) but documents intent; the light theme is the one that
// steps out of the dark family, which is why index.css scopes the `dark:` variant to the rest.
export interface ThemeInfo {
  name: ThemeName;
  label: string;
  hint: string;
  family: "dark" | "light";
}

export const THEMES: ThemeInfo[] = [
  { name: "dark", label: "Dark", hint: "Default · neutral near-black", family: "dark" },
  { name: "cool", label: "Cool", hint: "Cool slate · higher contrast", family: "dark" },
  { name: "contrast", label: "Contrast", hint: "Pure white on true black", family: "dark" },
  { name: "linear", label: "Linear", hint: "Cool graphite · indigo accent", family: "dark" },
  { name: "apple-dark", label: "Apple Dark", hint: "System grays · blue accent", family: "dark" },
  { name: "soft", label: "Soft", hint: "Lifted surfaces · gentler", family: "dark" },
  { name: "apple-light", label: "Apple Light", hint: "System light · blue accent", family: "light" },
  { name: "light", label: "Light", hint: "Paper · dark ink", family: "light" },
];

const THEME_NAMES = new Set<string>(THEMES.map((t) => t.name));

export const THEME_STORAGE_KEY = "aladin.theme";

export function readStoredTheme(): ThemeName {
  try {
    const stored = globalThis.localStorage?.getItem(THEME_STORAGE_KEY);
    return stored && THEME_NAMES.has(stored) ? (stored as ThemeName) : "dark";
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
  // Advance to the next theme in catalog order — the picker sets a theme directly, but this keeps
  // a one-shot toggle (e.g. a keybinding) meaningful across all four.
  toggleTheme: () => {
    const order = THEMES.map((t) => t.name);
    const next = order[(order.indexOf(get().theme) + 1) % order.length];
    get().setTheme(next);
  },
});
