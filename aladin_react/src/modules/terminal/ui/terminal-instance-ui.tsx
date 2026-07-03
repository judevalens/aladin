import { useEffect } from "react";
import { useTerminalSession } from "@/modules/terminal/hooks/use-terminal-session";
import { cn } from "@/lib/utils";

// One xterm instance ↔ one pty. Kept mounted while the session exists (even when
// its tab is inactive), so scrollback survives tab switches. Inactive instances are
// hidden with CSS rather than unmounted.
export function TerminalInstanceUI({
  id,
  active,
  onExit,
}: {
  id: string;
  active: boolean;
  onExit: (id: string) => void;
}) {
  const { containerRef, refit, ready } = useTerminalSession(id, onExit);

  useEffect(() => {
    if (active) {
      // Wait a frame so the container has non-zero size before fitting.
      const raf = requestAnimationFrame(() => refit());
      return () => cancelAnimationFrame(raf);
    }
  }, [active, refit]);

  // Always laid out (h-full w-full, no display toggle) so xterm keeps a real size and its
  // canvas stays painted; the parent controls show/hide via visibility to avoid a
  // switch-time flash. Held at opacity-0 until the first paint so create-time warm-up
  // isn't visible.
  return (
    <div
      ref={containerRef}
      aria-hidden={!active}
      className={cn("h-full w-full transition-opacity duration-150", ready ? "opacity-100" : "opacity-0")}
    />
  );
}
