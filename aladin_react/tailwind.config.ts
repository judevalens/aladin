import type { Config } from "tailwindcss";
import tailwindAnimate from "tailwindcss-animate";

export default {
  darkMode: "class",
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        border: "hsl(var(--border))",
        input: "hsl(var(--input))",
        ring: "hsl(var(--ring))",
        background: "hsl(var(--background))",
        foreground: "hsl(var(--foreground))",
        primary: {
          DEFAULT: "hsl(var(--primary))",
          foreground: "hsl(var(--primary-foreground))",
        },
        secondary: {
          DEFAULT: "hsl(var(--secondary))",
          foreground: "hsl(var(--secondary-foreground))",
        },
        muted: {
          DEFAULT: "hsl(var(--muted))",
          foreground: "hsl(var(--muted-foreground))",
        },
        accent: {
          DEFAULT: "hsl(var(--accent))",
          foreground: "hsl(var(--accent-foreground))",
        },
        destructive: {
          DEFAULT: "hsl(var(--destructive))",
          foreground: "hsl(var(--destructive-foreground))",
        },
        card: {
          DEFAULT: "hsl(var(--card))",
          foreground: "hsl(var(--card-foreground))",
        },
        popover: {
          DEFAULT: "hsl(var(--popover))",
          foreground: "hsl(var(--popover-foreground))",
        },
        aladin: {
          canvas: "var(--aladin-canvas)",
          panel: "var(--aladin-panel)",
          panelMuted: "var(--aladin-panel-muted)",
          rowHover: "var(--aladin-row-hover)",
          rowSelected: "var(--aladin-row-selected)",
          rowSelectedStrong: "var(--aladin-row-selected-strong)",
          controlHover: "var(--aladin-control-hover)",
          controlPressed: "var(--aladin-control-pressed)",
          divider: "var(--aladin-divider)",
          border: "var(--aladin-border)",
          ink: "var(--aladin-ink)",
          inkSecondary: "var(--aladin-ink-secondary)",
          inkMuted: "var(--aladin-ink-muted)",
          inkDisabled: "var(--aladin-ink-disabled)",
          inkSurface: "var(--aladin-ink-surface)",
          inkSurfaceHover: "var(--aladin-ink-surface-hover)",
          onInkSurface: "var(--aladin-on-ink-surface)",
          activeMarker: "var(--aladin-active-marker)",
          commandSurface: "var(--aladin-command-surface)",
          codeText: "var(--aladin-code-text)",
        },
      },
      borderRadius: {
        lg: "var(--radius)",
        md: "calc(var(--radius) - 2px)",
        sm: "calc(var(--radius) - 4px)",
        sharp: "4px",
        control: "6px",
      },
      fontFamily: {
        sans: ["Inter", "ui-sans-serif", "system-ui", "sans-serif"],
        mono: ["IBM Plex Mono", "ui-monospace", "SFMono-Regular", "monospace"],
      },
      boxShadow: {
        panel: "0 16px 40px rgba(24, 24, 20, 0.08)",
      },
      spacing: {
        sidebar: "232px",
        "workspace-max": "980px",
        "workspace-chrome-max": "1120px",
      },
    },
  },
  plugins: [tailwindAnimate],
} satisfies Config;
