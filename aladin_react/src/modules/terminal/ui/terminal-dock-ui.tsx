import "@xterm/xterm/css/xterm.css";
import { Plus, X } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { useAppStore } from "@/app/state/store";
import { cn } from "@/lib/utils";

const MIN_HEIGHT = 160;
const DEFAULT_HEIGHT = 288;

import { TerminalInstanceUI } from "@/modules/terminal/ui/terminal-instance-ui";

function newSessionId(): string {
  return typeof crypto !== "undefined" && crypto.randomUUID
    ? crypto.randomUUID()
    : `term-${Date.now()}-${Math.floor(Math.random() * 1e6)}`;
}

/**
 * IDE-style terminal dock. Mounted once in the workspace shell so pty sessions
 * survive route changes. Collapsed to `hidden` (not unmounted) while closed so the
 * xterm instances — and their scrollback — persist.
 */
export function TerminalDockUI() {
  const open = useAppStore((state) => state.terminalOpen);
  const sessions = useAppStore((state) => state.terminalSessions);
  const activeId = useAppStore((state) => state.activeTerminalId);
  const addSession = useAppStore((state) => state.addTerminalSession);
  const removeSession = useAppStore((state) => state.removeTerminalSession);
  const setActive = useAppStore((state) => state.setActiveTerminal);
  const setOpen = useAppStore((state) => state.setTerminalOpen);

  const [height, setHeight] = useState(DEFAULT_HEIGHT);
  const counterRef = useRef(0);

  const createSession = useCallback(() => {
    counterRef.current += 1;
    addSession({ id: newSessionId(), title: `Terminal ${counterRef.current}` });
  }, [addSession]);

  // Opening the dock with no sessions spawns the first shell.
  useEffect(() => {
    if (open && sessions.length === 0) createSession();
  }, [open, sessions.length, createSession]);

  const onResizeStart = useCallback((event: React.PointerEvent) => {
    event.preventDefault();
    const startY = event.clientY;
    const startHeight = height;
    const max = Math.min(window.innerHeight * 0.85, 720);
    const onMove = (move: PointerEvent) => {
      const next = startHeight + (startY - move.clientY);
      setHeight(Math.max(MIN_HEIGHT, Math.min(max, next)));
    };
    const onUp = () => {
      window.removeEventListener("pointermove", onMove);
      window.removeEventListener("pointerup", onUp);
    };
    window.addEventListener("pointermove", onMove);
    window.addEventListener("pointerup", onUp);
  }, [height]);

  if (sessions.length === 0) return null;

  // Closed = clip to zero height (overflow-hidden) rather than display:none, so the inner
  // dock stays laid out at full height and the xterm canvases stay alive/sized. Reopening
  // then reveals an already-painted terminal instead of flashing a blank/re-fitting frame.
  return (
    <div className="shrink-0 overflow-hidden" style={{ height: open ? height : 0 }}>
      <div className="flex flex-col border-t border-line bg-panel" style={{ height }}>
        {/* Resize handle */}
      <div
        role="separator"
        aria-orientation="horizontal"
        onPointerDown={onResizeStart}
        className="h-1 w-full cursor-row-resize bg-transparent hover:bg-amber-line"
      />
      {/* Tab strip */}
      <div className="flex h-9 items-center gap-1 border-b border-line px-2">
        <div className="scrollbar-hidden flex min-w-0 flex-1 items-center gap-1 overflow-x-auto">
          {sessions.map((session) => {
            const active = session.id === activeId;
            return (
              <div
                key={session.id}
                className={cn(
                  "group flex h-7 shrink-0 items-center gap-1.5 rounded-chip border px-2.5 text-[12px] transition-colors",
                  active
                    ? "border-line bg-bg text-ink"
                    : "border-transparent text-ink-3 hover:bg-raise hover:text-ink",
                )}
              >
                <button type="button" className="max-w-[160px] truncate" onClick={() => setActive(session.id)}>
                  {session.title}
                </button>
                <button
                  type="button"
                  aria-label={`Close ${session.title}`}
                  className="grid size-4 place-items-center rounded text-ink-4 hover:bg-[rgb(var(--hover))] hover:text-ink"
                  onClick={() => removeSession(session.id)}
                >
                  <X className="size-3" strokeWidth={2} />
                </button>
              </div>
            );
          })}
        </div>
        <button
          type="button"
          aria-label="New terminal"
          title="New terminal"
          onClick={createSession}
          className="grid size-6 shrink-0 place-items-center rounded text-ink-3 hover:bg-[rgb(var(--hover))] hover:text-ink"
        >
          <Plus className="size-4" strokeWidth={1.75} />
        </button>
        <button
          type="button"
          aria-label="Hide terminal"
          title="Hide terminal"
          onClick={() => setOpen(false)}
          className="grid size-6 shrink-0 place-items-center rounded text-ink-3 hover:bg-[rgb(var(--hover))] hover:text-ink"
        >
          <X className="size-4" strokeWidth={1.75} />
        </button>
      </div>
      {/* Instances — all kept mounted AND laid out; only the active one is shown.
          Inactive tabs use visibility (not display:none) so their WebGL canvas and
          measured size survive the switch — otherwise re-showing a display:none terminal
          flashes a blank/mis-sized frame while xterm re-fits and repaints. */}
      <div className="relative min-h-0 flex-1 bg-bg">
        {sessions.map((session) => (
          <div
            key={session.id}
            className={cn(
              "absolute inset-0 px-3 py-1.5",
              session.id === activeId ? "visible z-10" : "invisible",
            )}
          >
            <TerminalInstanceUI id={session.id} active={session.id === activeId && open} onExit={removeSession} />
          </div>
        ))}
        </div>
      </div>
    </div>
  );
}
