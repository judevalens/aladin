import { useEffect } from "react";

/**
 * Radix bug workaround: when a Dialog is opened from a (modal) ContextMenu, the dialog can
 * capture the menu's `pointer-events: none` as its "original" body value and restore that
 * `none` on close — leaving the whole app unclickable even though nothing is open.
 *
 * Watch for a residual lock and strip it, but ONLY when no modal layer is actually mounted,
 * so an open menu/dialog keeps its legitimate lock. This reacts to the real DOM mutation
 * (no timers).
 *
 * Lives here rather than inside one dialog because every dialog reachable from the tree's
 * context menu needs it, and a subtle workaround copy-pasted twice is a workaround that gets
 * fixed once.
 */
export function useOrphanedPointerLockGuard() {
  useEffect(() => {
    const root = document.documentElement;
    const body = document.body;
    const stripOrphanedLock = () => {
      if (body.style.pointerEvents !== "none" && root.style.pointerEvents !== "none") return;
      if (document.querySelector("[role='dialog'],[role='alertdialog'],[role='menu']")) return;
      body.style.pointerEvents = "";
      root.style.pointerEvents = "";
    };
    const observer = new MutationObserver(stripOrphanedLock);
    observer.observe(body, { attributes: true, attributeFilter: ["style"], childList: true });
    observer.observe(root, { attributes: true, attributeFilter: ["style"] });
    return () => observer.disconnect();
  }, []);
}
