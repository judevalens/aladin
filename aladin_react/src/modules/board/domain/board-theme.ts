import { DEFAULT_THEME } from "tldraw";
import type { TLDefaultColor, TLTheme } from "tldraw";

/**
 * The board's tldraw theme, built over the Aladin tokens.
 *
 * Two rules shape this file:
 *
 * 1. It spreads DEFAULT_THEME so **all 13 stock color names survive**. tldraw's
 *    `registerColorsFromThemes` *removes* any registered color absent from every provided
 *    theme — and a board saved before this theme existed contains `color: "black"` shapes
 *    that would then fail validation and vanish. Keeping the defaults is load-bearing, not
 *    politeness; `board-theme.test.ts` guards it.
 * 2. It is a pure function of a token reader, so it can run under vitest with the palette
 *    below and under the app with `getComputedStyle` — the live `data-theme` stays the
 *    source of truth without this module touching the DOM at import time.
 */

declare module "@tldraw/tlschema" {
  interface TLThemeDefaultColors {
    /** Thinking ink — the handoff's --learn, Pencil's default color. */
    learn: TLDefaultColor;
    /** The accent, same hue as the shell's amber. */
    amber: TLDefaultColor;
    /** Counters/negations — the shell's --against. */
    against: TLDefaultColor;
    /** Object-to-object link arrows: amber at .45, per the handoff. */
    link: TLDefaultColor;
  }
}

/** The three ink colors the dock offers (a strict subset of what the theme registers). */
export const BOARD_INK_COLORS = ["learn", "amber", "against"] as const;
export type BoardInkColor = (typeof BOARD_INK_COLORS)[number];

/** Fallbacks = the dark theme of record from the handoff; the live app overrides via vars. */
const DARK_FALLBACKS: Record<string, string> = {
  "--bg": "#0d0d10",
  "--ink": "#eceaef",
  "--amber": "#c9925a",
  "--against": "#d8796b",
  "--board-learn": "#8fb9e8",
};

export type TokenReader = (name: string) => string;

/** Reads a CSS custom property off <html>, falling back to the dark palette. */
export function domTokenReader(): TokenReader {
  const style =
    typeof window === "undefined" ? null : window.getComputedStyle(document.documentElement);
  return (name) => {
    const raw = style?.getPropertyValue(name).trim();
    return raw && raw.startsWith("#") ? raw : (DARK_FALLBACKS[name] ?? "#000000");
  };
}

function hexToRgb(hex: string): [number, number, number] {
  const h = hex.replace("#", "");
  const v =
    h.length === 3
      ? h
          .split("")
          .map((c) => c + c)
          .join("")
      : h;
  return [parseInt(v.slice(0, 2), 16), parseInt(v.slice(2, 4), 16), parseInt(v.slice(4, 6), 16)];
}

function toHex([r, g, b]: [number, number, number]): string {
  const c = (n: number) => Math.round(Math.max(0, Math.min(255, n))).toString(16).padStart(2, "0");
  return `#${c(r)}${c(g)}${c(b)}`;
}

/** `amount` of `over` blended onto `base` (0 = base, 1 = over). */
export function blend(base: string, over: string, amount: number): string {
  const [br, bg, bb] = hexToRgb(base);
  const [or_, og, ob] = hexToRgb(over);
  return toHex([br + (or_ - br) * amount, bg + (og - bg) * amount, bb + (ob - bb) * amount]);
}

function rgba(hex: string, alpha: number): string {
  const [r, g, b] = hexToRgb(hex);
  return `rgba(${r},${g},${b},${alpha})`;
}

/**
 * All 14 TLDefaultColor variants derived from one ink color over one background.
 * The board only exercises solid/semi/highlight in practice; the rest are derived
 * consistently so frames/notes would not render garbage if they ever appear.
 */
export function makeInkColor(solid: string, bg: string, ink: string): TLDefaultColor {
  return {
    solid,
    semi: blend(bg, solid, 0.16),
    pattern: blend(solid, bg, 0.25),
    fill: solid,
    linedFill: blend(bg, solid, 0.12),
    frameHeadingStroke: blend(solid, bg, 0.4),
    frameHeadingFill: blend(bg, solid, 0.08),
    frameStroke: blend(solid, bg, 0.4),
    frameFill: blend(bg, solid, 0.04),
    frameText: ink,
    noteFill: blend(bg, solid, 0.2),
    noteText: ink,
    highlightSrgb: solid,
    highlightP3: solid,
  };
}

/** Builds the board theme. Pass a reader in tests; defaults to live CSS vars. */
export function buildBoardTheme(read: TokenReader = domTokenReader()): TLTheme {
  const bg = read("--bg");
  const ink = read("--ink");
  const amber = read("--amber");
  const against = read("--against");
  const learn = read("--board-learn");

  const darkCustoms = {
    learn: makeInkColor(learn, bg, ink),
    amber: makeInkColor(amber, bg, ink),
    against: makeInkColor(against, bg, ink),
    link: { ...makeInkColor(amber, bg, ink), solid: rgba(amber, 0.45) },
  };
  // The board is dark-of-record; the light branch exists so the type is total and a future
  // light board is not garbage. Same hues over white.
  const lightCustoms = {
    learn: makeInkColor("#0a6cdc", "#ffffff", "#1d1d1f"),
    amber: makeInkColor(amber, "#ffffff", "#1d1d1f"),
    against: makeInkColor(against, "#ffffff", "#1d1d1f"),
    link: { ...makeInkColor(amber, "#ffffff", "#1d1d1f"), solid: rgba(amber, 0.45) },
  };

  return {
    ...DEFAULT_THEME,
    id: "default",
    fonts: {
      draw: { fontFamily: "'Caveat', 'Segoe Print', 'Bradley Hand', cursive" },
      sans: {
        fontFamily: "'Space Grotesk Variable', 'Space Grotesk', system-ui, sans-serif",
      },
      serif: { fontFamily: "Georgia, 'Times New Roman', serif" },
      mono: { fontFamily: "'JetBrains Mono', ui-monospace, monospace" },
    },
    colors: {
      dark: {
        ...DEFAULT_THEME.colors.dark,
        background: bg,
        text: ink,
        selectionStroke: amber,
        brushFill: rgba(amber, 0.08),
        brushStroke: rgba(amber, 0.35),
        ...darkCustoms,
      },
      light: {
        ...DEFAULT_THEME.colors.light,
        ...lightCustoms,
      },
    },
  };
}
