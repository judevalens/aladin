import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";

import { Eyebrow } from "@/components/ui/eyebrow";
import { Icon } from "@/components/ui/icon";
import { useTabSwitcher, type TabSwitcherRow } from "@/modules/workspace/hooks/use-tab-switcher";
import {
  ARTIFACT_ICONS,
  RESEARCH_ICON,
  RESEARCH_SLOT_ICONS,
} from "@/modules/workspace/ui/kind-icons";
import { cn } from "@/lib/utils";

/**
 * The Ctrl+Tab overlay: every open tab in most-recently-used order.
 *
 * The tab strip is a status display, not a navigation surface — its hidden scrollbar is
 * deliberate — so reaching a tab needed somewhere else to live. This is that somewhere. It
 * never reorders the strip (see use-tab-switcher for why the two orderings coexist).
 *
 * Mounted only while open, so the whole thing costs nothing when it isn't on screen.
 */
export function TabSwitcher() {
  const { rows, highlight, filter, onFilter, onMove, onHighlight, onCommit, onCancel, onCloseHighlighted } =
    useTabSwitcher();
  // "Browse mode": the user tapped Ctrl+Tab and let go before choosing, so there is no
  // Ctrl-release to commit on and the overlay waits for arrows / typing / Enter / Esc.
  const [holding, setHolding] = useState(true);
  const highlightRef = useRef<HTMLButtonElement | null>(null);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Tab") {
        event.preventDefault();
        onMove(event.shiftKey ? -1 : 1);
        return;
      }
      if (event.key === "Escape") {
        event.preventDefault();
        onCancel();
        return;
      }
      if (event.key === "Enter") {
        event.preventDefault();
        onCommit();
        return;
      }
      if (event.key === "ArrowDown" || event.key === "ArrowUp") {
        event.preventDefault();
        setHolding(false);
        onMove(event.key === "ArrowDown" ? 1 : -1);
        return;
      }
      if (event.key === "Backspace" || event.key === "Delete") {
        event.preventDefault();
        onCloseHighlighted();
        return;
      }
      // Typing switches to filter mode. Printable keys only, and never while a modifier that
      // means something else is down.
      if (event.key.length === 1 && !event.metaKey && !event.altKey) {
        if (event.ctrlKey) return;
        event.preventDefault();
        setHolding(false);
        onFilter(filter + event.key);
      }
    };
    const onKeyUp = (event: KeyboardEvent) => {
      if (event.key === "Control" && holding) onCommit();
    };
    document.addEventListener("keydown", onKeyDown);
    document.addEventListener("keyup", onKeyUp);
    return () => {
      document.removeEventListener("keydown", onKeyDown);
      document.removeEventListener("keyup", onKeyUp);
    };
  }, [filter, holding, onCancel, onCloseHighlighted, onCommit, onFilter, onMove]);

  // Keep the highlight in view without scrollIntoView, which scrolls ancestors too.
  useEffect(() => {
    const el = highlightRef.current;
    if (!el) return;
    const list = el.parentElement?.parentElement;
    if (!list) return;
    const top = el.offsetTop;
    const bottom = top + el.offsetHeight;
    if (top < list.scrollTop) list.scrollTop = top;
    else if (bottom > list.scrollTop + list.clientHeight) {
      list.scrollTop = bottom - list.clientHeight;
    }
  }, [highlight]);

  return createPortal(
    <div
      className="fixed inset-0 z-50 bg-scrim backdrop-blur-[2px]"
      onMouseDown={onCancel}
      data-testid="tab-switcher-scrim"
    >
      <div
        className="animate-pop fixed left-1/2 top-[18vh] flex max-h-[min(60vh,520px)] w-[480px] -translate-x-1/2 flex-col overflow-hidden rounded-modal border border-line bg-explorer shadow-modal"
        onMouseDown={(event) => event.stopPropagation()}
        role="dialog"
        aria-label="Switch tab"
      >
        {filter ? (
          <input
            className="h-9 shrink-0 border-b border-line bg-transparent px-3 font-mono text-small text-ink outline-none placeholder:text-ink-4"
            value={filter}
            placeholder="filter tabs…"
            onChange={(event) => onFilter(event.target.value)}
            autoFocus
          />
        ) : null}

        <div className="min-h-0 flex-1 overflow-y-auto p-1.5">
          {rows.length === 0 ? (
            <div className="px-2 py-6 text-center text-small text-ink-4">No open tab matches.</div>
          ) : (
            rows.map((row, index) => (
              <div key={row.key}>
                {row.startsGroup ? (
                  <Eyebrow className="px-2 pt-2 pb-1 text-ink-4">{row.groupTitle}</Eyebrow>
                ) : null}
                <Row
                  ref={index === highlight ? highlightRef : undefined}
                  row={row}
                  highlighted={index === highlight}
                  onMouseEnter={() => onHighlight(index)}
                  onClick={() => onCommit(row.key)}
                />
              </div>
            ))
          )}
        </div>

        <div className="flex h-7 shrink-0 items-center gap-3 border-t border-line px-3 font-mono text-meta text-ink-4">
          <span>⏎ open</span>
          <span>⌫ close</span>
          <span>esc cancel</span>
        </div>
      </div>
    </div>,
    document.body,
  );
}

function Row({
  row,
  highlighted,
  onMouseEnter,
  onClick,
  ref,
}: {
  row: TabSwitcherRow;
  highlighted: boolean;
  onMouseEnter: () => void;
  onClick: () => void;
  ref?: React.Ref<HTMLButtonElement>;
}) {
  // Narrow through the tab union rather than a boolean, so `view` is reachable.
  const research = row.tab.kind === "research" ? row.tab : null;
  // A research row sits under its folder's header, so the row itself shows the view only —
  // and the slot glyph, which is what distinguishes Runs from Manifest at a glance.
  const glyph = research
    ? RESEARCH_SLOT_ICONS[research.view]
    : row.artifact
      ? ARTIFACT_ICONS[row.artifact.kind]
      : RESEARCH_ICON;

  return (
    <button
      ref={ref}
      type="button"
      onMouseEnter={onMouseEnter}
      onClick={onClick}
      className={cn(
        "relative flex h-8 w-full items-center gap-2 rounded-control px-3 text-left transition-colors",
        highlighted ? "bg-[rgb(var(--sel))] text-ink" : "text-ink-2 hover:bg-[rgb(var(--hover))]",
      )}
    >
      {/* The same left amber bar the tree uses for its active row — one visual language for
          "you are here", rather than a second one invented for this surface. */}
      {row.active ? (
        <span className="absolute left-0 top-[5px] bottom-[5px] w-0.5 rounded-tap bg-amber" />
      ) : null}
      <Icon as={glyph} size="inline" className={cn("shrink-0", research && "text-amber")} />
      <span className="min-w-0 flex-1 truncate">{research ? row.viewLabel : row.label}</span>
      {!research && row.parentFolderTitle ? (
        <span className="shrink-0 font-mono text-meta text-ink-4">{row.parentFolderTitle}</span>
      ) : null}
    </button>
  );
}
