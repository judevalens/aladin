import { useEffect, useRef, useState } from "react";
import { FitAddon } from "@xterm/addon-fit";
import { WebglAddon } from "@xterm/addon-webgl";
import { Terminal } from "@xterm/xterm";
import { useAppStore } from "@/app/state/store";
import { terminalRepo } from "@/repos/terminal/terminal-repo";
import { terminalTheme } from "@/modules/terminal/terminal-theme";

// Concrete monospace stack — the canvas/WebGL renderer can't resolve CSS `var()`,
// and it must match a font that's actually loaded or glyph metrics come out wrong.
// Matches the app's font-mono (JetBrains Mono, via @fontsource).
const FONT_FAMILY = '"JetBrains Mono", ui-monospace, "SFMono-Regular", Menlo, monospace';
const FONT_SIZE = 13;

/**
 * Binds an xterm instance to a Rust PTY for the given session id. The container is
 * kept mounted for the session's lifetime (so scrollback survives tab switches and
 * dock toggles); unmount closes the pty. `onExit` fires when the shell exits.
 */
export function useTerminalSession(id: string, onExit: (id: string) => void) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const termRef = useRef<Terminal | null>(null);
  const fitRef = useRef<FitAddon | null>(null);
  // Revealed only after xterm's first paint, so the create-time warm-up (font load, the
  // initial 80x24 → fitted reflow, WebGL atlas build) happens off-screen instead of
  // flashing when a terminal first appears.
  const [ready, setReady] = useState(false);
  const theme = useAppStore((state) => state.theme);
  // Keep the latest onExit without re-running the setup effect.
  const onExitRef = useRef(onExit);
  onExitRef.current = onExit;

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    let disposed = false;
    let cleanup = () => {};

    const start = () => {
      if (disposed || !container) return;

      const term = new Terminal({
        fontFamily: FONT_FAMILY,
        fontSize: FONT_SIZE,
        cursorBlink: true,
        theme: terminalTheme(),
        allowProposedApi: true,
      });
      const fit = new FitAddon();
      term.loadAddon(fit);
      term.open(container);
      // GPU-accelerated rendering. Falls back to the DOM renderer if WebGL is
      // unavailable or its context is lost (e.g. GPU reset).
      try {
        const webgl = new WebglAddon();
        webgl.onContextLoss(() => webgl.dispose());
        term.loadAddon(webgl);
      } catch {
        // no WebGL — xterm keeps its default renderer
      }
      fit.fit();
      termRef.current = term;
      fitRef.current = fit;

      // Reveal after the first render (with a timer fallback), so no intermediate frame
      // is shown. onRender fires once the initial screen/cursor paints.
      let revealed = false;
      const reveal = () => {
        if (!revealed && !disposed) {
          revealed = true;
          setReady(true);
        }
      };
      const renderSub = term.onRender(reveal);
      const revealTimer = setTimeout(reveal, 400);

      void terminalRepo
        .open(
          id,
          { cols: term.cols, rows: term.rows },
          (bytes) => {
            if (!disposed) term.write(bytes);
          },
          () => {
            if (!disposed) onExitRef.current(id);
          },
        )
        .catch((error: unknown) => {
          term.write(`\r\n\x1b[31mfailed to start terminal: ${String(error)}\x1b[0m\r\n`);
        });

      const dataSub = term.onData((data) => {
        void terminalRepo.write(id, data);
      });

      const resizeObserver = new ResizeObserver(() => {
        if (disposed) return;
        try {
          fit.fit();
          void terminalRepo.resize(id, term.cols, term.rows);
        } catch {
          // container may be hidden (display:none) — ignore until it's shown again.
        }
      });
      resizeObserver.observe(container);

      cleanup = () => {
        clearTimeout(revealTimer);
        renderSub.dispose();
        resizeObserver.disconnect();
        dataSub.dispose();
        void terminalRepo.close(id);
        term.dispose();
        termRef.current = null;
        fitRef.current = null;
      };
    };

    // Gate on the mono web font being loaded, so xterm measures real glyph widths
    // (otherwise the WebGL atlas bakes fallback metrics → tiny, spaced-out text).
    const fontReady =
      typeof document !== "undefined" && document.fonts?.load
        ? document.fonts.load(`${FONT_SIZE}px "JetBrains Mono"`).then(
            () => undefined,
            () => undefined,
          )
        : Promise.resolve();
    void fontReady.then(start);

    return () => {
      disposed = true;
      cleanup();
    };
  }, [id]);

  // Track the design-token theme when the user flips Dark/Soft.
  useEffect(() => {
    if (termRef.current) termRef.current.options.theme = terminalTheme();
  }, [theme]);

  // Re-fit when the caller signals the instance became visible.
  const refit = () => {
    try {
      fitRef.current?.fit();
      if (termRef.current) void terminalRepo.resize(id, termRef.current.cols, termRef.current.rows);
      termRef.current?.focus();
    } catch {
      // ignore fit failures on a hidden container
    }
  };

  return { containerRef, refit, ready };
}
