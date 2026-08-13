import { useCallback, useMemo, useRef, useState } from "react";

import { useAppStore } from "@/app/state/store";
import { orderByMru, type WorkTab } from "@/modules/workspace/domain";
import { useWorkPane, type WorkPaneTab } from "@/modules/workspace/hooks/use-workspace-state";

export interface TabSwitcherRow extends WorkPaneTab {
  /** True for the first row of each research group — the group header renders above it. */
  startsGroup: boolean;
  /** True when this is the currently active tab (index 0 of the MRU list). */
  active: boolean;
}

export interface TabSwitcherState {
  rows: TabSwitcherRow[];
  /** Index into `rows`. -1 only when a filter matches nothing. */
  highlight: number;
  filter: string;
  onFilter: (value: string) => void;
  /** Move the highlight by `delta`, wrapping at both ends. */
  onMove: (delta: number) => void;
  onHighlight: (index: number) => void;
  /** Activate the highlighted row (or `key`) and close. */
  onCommit: (key?: string) => void;
  onCancel: () => void;
  /** Close the highlighted tab, keeping the overlay open. */
  onCloseHighlighted: () => void;
}

/**
 * The Ctrl+Tab overlay's state.
 *
 * Two orderings coexist and neither touches the other: the strip stays STRUCTURAL (§12
 * grouping, so a research folder's tabs are contiguous) and the switcher is RECENCY. This
 * hook reads the very same `tabs` array the strip renders — not a second derivation — so the
 * grouping invariant cannot drift between the two surfaces.
 *
 * The MRU order is frozen when the overlay opens. Committing calls activateTab, which
 * promotes that key to the head; if the list re-sorted live, cycling would move the rows you
 * are cycling through under your own fingers.
 */
export function useTabSwitcher(): TabSwitcherState {
  const { tabs, onActivateTab, onCloseTab } = useWorkPane();
  const setOpen = useAppStore((state) => state.setTabSwitcherOpen);
  const activeTabKey = useAppStore((state) => state.workspace.activeTabKey);
  const liveMru = useAppStore((state) => state.workspace.tabMru);

  // Frozen on mount — the component only mounts while the overlay is open.
  const frozenMru = useRef(liveMru).current;
  const [filter, setFilter] = useState("");
  // Opens on index 1, the previously-active tab. That single choice is what makes a repeated
  // Ctrl+Tab a toggle between the last two tabs rather than a no-op on the current one.
  const [highlight, setHighlight] = useState(1);

  const rows = useMemo<TabSwitcherRow[]>(() => {
    const ordered = orderByMru(tabs, frozenMru);
    const needle = filter.trim().toLowerCase();
    const matched = needle
      ? ordered.filter(
          (t) =>
            t.label.toLowerCase().includes(needle) ||
            (t.groupTitle ?? "").toLowerCase().includes(needle),
        )
      : ordered;

    // Group headers render for the first surviving row of each research folder, so a filter
    // that removes a whole group removes its header with it.
    const seen = new Set<string>();
    return matched.map((t) => {
      const contextId = t.tab.kind === "research" ? t.tab.contextId : null;
      const startsGroup = contextId !== null && !seen.has(contextId);
      if (contextId !== null) seen.add(contextId);
      return { ...t, startsGroup, active: t.key === activeTabKey };
    });
  }, [tabs, frozenMru, filter, activeTabKey]);

  const clamped = rows.length === 0 ? -1 : Math.min(highlight, rows.length - 1);

  const onCancel = useCallback(() => setOpen(false), [setOpen]);

  const onCommit = useCallback(
    (key?: string) => {
      const target = key ?? rows[clamped]?.key;
      if (target) onActivateTab(target);
      setOpen(false);
    },
    [rows, clamped, onActivateTab, setOpen],
  );

  const onMove = useCallback(
    (delta: number) => {
      setHighlight((current) => {
        if (rows.length === 0) return 0;
        const from = Math.min(current, rows.length - 1);
        return (from + delta + rows.length) % rows.length;
      });
    },
    [rows.length],
  );

  const onFilter = useCallback((value: string) => {
    setFilter(value);
    // A new filter means a new list; index 0 is the most recent match.
    setHighlight(0);
  }, []);

  const onCloseHighlighted = useCallback(() => {
    const row = rows[clamped];
    if (!row) return;
    onCloseTab(row.key);
    // Closing the last remaining tab leaves nothing to switch to.
    if (rows.length <= 1) {
      setOpen(false);
      return;
    }
    // Hold position so the highlight lands on the next row down; if it was last, the clamp
    // above pulls it back to the new last.
    setHighlight(clamped);
  }, [rows, clamped, onCloseTab, setOpen]);

  return {
    rows,
    highlight: clamped,
    filter,
    onFilter,
    onMove,
    onHighlight: setHighlight,
    onCommit,
    onCancel,
    onCloseHighlighted,
  };
}

/** True when the switcher has something to switch between (§5.5: 0 or 1 tabs → no flash). */
export function canSwitchTabs(openTabs: WorkTab[]): boolean {
  return openTabs.length > 1;
}
