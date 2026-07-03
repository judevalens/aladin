import type { ITheme } from "@xterm/xterm";

// Build xterm's theme from the live Aladin design tokens (index.css) so the
// terminal tracks Dark/Soft without hardcoding hex in the component. xterm renders
// to canvas and needs concrete color strings, so we read the resolved token values.
function token(name: string, fallback: string): string {
  if (typeof window === "undefined") return fallback;
  const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  return value || fallback;
}

export function terminalTheme(): ITheme {
  const bg = token("--bg", "#202024");
  const ink = token("--ink", "#d7d6da");
  const amber = token("--amber", "#c69566");
  const raise = token("--raise", "#252529");
  return {
    background: bg,
    foreground: ink,
    cursor: amber,
    cursorAccent: bg,
    selectionBackground: raise,
  };
}
