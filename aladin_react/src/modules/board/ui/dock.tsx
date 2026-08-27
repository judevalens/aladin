import { useLayoutEffect, useRef, useState } from "react";

import {
  BOARD_INK_SWATCHES,
  BOARD_WEIGHTS,
  type BoardTool,
  type BoardWeightIndex,
  type PencilSubTool,
} from "../domain/board-tools";
import type { BoardInkColor } from "../domain/board-theme";
import { DOCK_PATHS, DockIcon } from "./dock-icons";
import { StylePopover } from "./style-popover";

/** The dock's zoom presets — the whole menu, deliberately. Pinch covers everything else. */
export const DOCK_ZOOM_PRESETS = [50, 75, 100, 150] as const;

/**
 * The bottom tool dock — pure render plus its own geometry; all tool state lives in
 * BoardChrome.
 *
 * Two things make it feel like a fixture rather than a widget:
 * - **The island is CENTRED on its full width** (owner's call 2026-08-26: the old
 *   head-anchored layout read as off-centre once a tray was open). Switching tools
 *   re-centres, so the tray grows out of both sides symmetrically.
 * - **Sub-options fold.** Pencil's colour and weight are one *style* tile showing the
 *   current ink at its weight; tapping it — or re-tapping the active sub-tool — opens a
 *   popover (the Freeform / GoodNotes convention). Keeps every target ≥ 44pt and the dock
 *   inside a portrait iPad. Zoom folds the same way: one `%` tile → a four-preset popover
 *   (pinch is the real zoom control; the flank pill it replaces was chrome overkill).
 */
export function Dock({
  tool,
  subTool,
  inkColor,
  weight,
  drawWithFinger,
  insertOpen,
  styleOpen,
  zoomPct,
  zoomOpen,
  canUndo,
  canRedo,
  onUndo,
  onRedo,
  onPickTool,
  onPickSubTool,
  onPickColor,
  onPickWeight,
  onToggleDrawWithFinger,
  onToggleInsert,
  onToggleStyle,
  onToggleZoom,
  onPickZoom,
  onAddInk,
  onAddTask,
  onAddCard,
  onQuizMe,
}: {
  tool: BoardTool;
  subTool: PencilSubTool;
  inkColor: BoardInkColor;
  weight: BoardWeightIndex;
  drawWithFinger: boolean;
  insertOpen: boolean;
  styleOpen: boolean;
  /** Current zoom in percent; null hides the tile (the pencil owns its camera). */
  zoomPct: number | null;
  zoomOpen: boolean;
  canUndo: boolean;
  canRedo: boolean;
  onUndo: () => void;
  onRedo: () => void;
  onPickTool: (tool: BoardTool) => void;
  onPickSubTool: (subTool: PencilSubTool) => void;
  onPickColor: (color: BoardInkColor) => void;
  onPickWeight: (weight: BoardWeightIndex) => void;
  onToggleDrawWithFinger: () => void;
  onToggleInsert: () => void;
  onToggleStyle: () => void;
  onToggleZoom: () => void;
  onPickZoom: (pct: number) => void;
  onAddInk: () => void;
  onAddTask: () => void;
  onAddCard: () => void;
  /** "Quiz" appears only when a copilot host exists (desktop today). */
  onQuizMe?: () => void;
}) {
  const islandRef = useRef<HTMLDivElement>(null);
  const styleRef = useRef<HTMLButtonElement>(null);
  const zoomRef = useRef<HTMLButtonElement>(null);
  const [left, setLeft] = useState<number | null>(null);
  const [styleLeft, setStyleLeft] = useState<number>(0);
  const [zoomLeft, setZoomLeft] = useState<number>(0);

  // Centre the island on its FULL width, clamped so it never leaves the viewport on a
  // narrow (portrait) iPad. Re-measured whenever the tray changes size.
  useLayoutEffect(() => {
    const island = islandRef.current;
    if (!island) return;
    const measure = () => {
      const vw = island.parentElement?.clientWidth ?? window.innerWidth;
      const width = island.offsetWidth;
      const ideal = (vw - width) / 2;
      setLeft(Math.max(8, Math.min(ideal, vw - width - 8)));
      const style = styleRef.current;
      setStyleLeft(style ? style.offsetLeft + style.offsetWidth / 2 : 0);
      const zoomEl = zoomRef.current;
      setZoomLeft(zoomEl ? zoomEl.offsetLeft + zoomEl.offsetWidth / 2 : 0);
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(island);
    if (island.parentElement) observer.observe(island.parentElement);
    return () => observer.disconnect();
  }, [tool]);

  const mainTools: { id: BoardTool; label: string; d: string }[] = [
    { id: "select", label: "Select", d: DOCK_PATHS.select },
    { id: "pencil", label: "Pencil", d: DOCK_PATHS.pencil },
    { id: "arrow", label: "Arrow", d: DOCK_PATHS.arrow },
  ];
  const subTools: { id: PencilSubTool; label: string; d: string }[] = [
    { id: "pen", label: "Pen", d: DOCK_PATHS.pen },
    { id: "highlighter", label: "Highlighter", d: DOCK_PATHS.highlighter },
    { id: "eraser", label: "Eraser", d: DOCK_PATHS.eraser },
    { id: "lasso", label: "Lasso", d: DOCK_PATHS.lasso },
  ];
  const swatch = BOARD_INK_SWATCHES.find((s) => s.id === inkColor) ?? BOARD_INK_SWATCHES[0];

  return (
    <div
      ref={islandRef}
      className="board-island board-edge-bottom pointer-events-auto absolute flex items-center gap-0.5 p-1.5"
      style={{ left: left ?? "50%", visibility: left === null ? "hidden" : undefined }}
    >
      <div className="flex items-center gap-0.5">
        <DockButton label="Undo" active={false} narrow disabled={!canUndo} onClick={onUndo}>
          <DockIcon d={DOCK_PATHS.undo} size={19} />
        </DockButton>
        <DockButton label="Redo" active={false} narrow disabled={!canRedo} onClick={onRedo}>
          <DockIcon d={DOCK_PATHS.redo} size={19} />
        </DockButton>
        <Divider />
        <DockButton label="Insert from workspace" active={insertOpen} onClick={onToggleInsert}>
          <DockIcon d={DOCK_PATHS.insert} size={22} strokeWidth={1.9} />
        </DockButton>
        <Divider />
        {mainTools.map((t) => (
          <DockButton
            key={t.id}
            label={t.label}
            active={tool === t.id}
            onClick={() => onPickTool(t.id)}
          >
            <DockIcon d={t.d} />
          </DockButton>
        ))}
      </div>

      {tool === "select" ? (
        <>
          <Divider />
          <TextButton label="Ink" onClick={onAddInk} />
          <TextButton label="Task" onClick={onAddTask} />
          <TextButton label="Card" onClick={onAddCard} />
          {onQuizMe ? (
            <>
              <Divider />
              <TextButton label="Quiz" onClick={onQuizMe} />
            </>
          ) : null}
        </>
      ) : null}

      {tool === "pencil" ? (
        <>
          <Divider />
          {subTools.map((t) => (
            <DockButton
              key={t.id}
              label={t.label}
              active={subTool === t.id}
              narrow
              onClick={() => (subTool === t.id ? onToggleStyle() : onPickSubTool(t.id))}
            >
              <DockIcon d={t.d} size={20} />
            </DockButton>
          ))}
          <button
            ref={styleRef}
            type="button"
            aria-label="Ink colour and weight"
            aria-expanded={styleOpen}
            onClick={onToggleStyle}
            className={`board-tile grid h-[52px] w-[52px] place-items-center rounded-board-tile ${
              styleOpen ? "bg-sel" : "hover:bg-hover"
            }`}
          >
            <span
              className="rounded-full"
              style={{
                width: BOARD_WEIGHTS[weight].dotPx + 8,
                height: BOARD_WEIGHTS[weight].dotPx + 8,
                background: swatch.cssVar,
                boxShadow: "0 0 0 1px rgb(var(--line))",
              }}
            />
          </button>
          {styleOpen ? (
            <StylePopover
              centerX={styleLeft}
              inkColor={inkColor}
              weight={weight}
              drawWithFinger={drawWithFinger}
              onPickColor={onPickColor}
              onPickWeight={onPickWeight}
              onToggleDrawWithFinger={onToggleDrawWithFinger}
            />
          ) : null}
        </>
      ) : null}

      {zoomPct !== null ? (
        <>
          <Divider />
          <button
            ref={zoomRef}
            type="button"
            aria-label="Zoom"
            aria-expanded={zoomOpen}
            onClick={onToggleZoom}
            className={`board-tile h-[52px] min-w-[64px] rounded-board-tile px-2 text-center font-mono text-small ${
              zoomOpen ? "bg-sel text-ink" : "text-ink-3 hover:bg-hover hover:text-ink"
            }`}
          >
            {zoomPct}%
          </button>
          {zoomOpen ? <ZoomPopover centerX={zoomLeft} zoomPct={zoomPct} onPick={onPickZoom} /> : null}
        </>
      ) : null}
    </div>
  );
}

/**
 * The zoom presets, folded out of the `%` tile the way the style popover folds out of the
 * style tile. Four rows and nothing else — pinch already covers continuous zoom.
 */
function ZoomPopover({
  centerX,
  zoomPct,
  onPick,
}: {
  /** The zoom tile's centre, in the dock's coordinate space. */
  centerX: number;
  zoomPct: number;
  onPick: (pct: number) => void;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const [left, setLeft] = useState(centerX);

  // Centre on the tile, but never past the viewport's edge — the tile sits at the dock's
  // far right, which on a portrait iPad is the screen's far right.
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const dockLeft = el.offsetParent?.getBoundingClientRect().left ?? 0;
    const half = el.offsetWidth / 2;
    const vw = window.innerWidth;
    const min = 8 + half - dockLeft;
    const max = vw - 8 - half - dockLeft;
    setLeft(Math.max(min, Math.min(centerX, max)));
  }, [centerX]);

  return (
    <div
      ref={ref}
      role="menu"
      aria-label="Zoom"
      className="board-island board-island--popover absolute bottom-[calc(100%+10px)] flex w-32 -translate-x-1/2 flex-col overflow-hidden py-1"
      style={{ left }}
    >
      {DOCK_ZOOM_PRESETS.map((pct) => {
        const active = pct === zoomPct;
        return (
          <button
            key={pct}
            type="button"
            role="menuitem"
            aria-pressed={active}
            onClick={() => onPick(pct)}
            className={`flex h-11 w-full items-center px-4 text-left font-mono text-board-row ${
              active ? "text-amber" : "text-ink hover:bg-hover active:bg-sel"
            }`}
          >
            {pct}%
          </button>
        );
      })}
    </div>
  );
}

function DockButton({
  label,
  active,
  narrow,
  disabled,
  onClick,
  children,
}: {
  label: string;
  active: boolean;
  narrow?: boolean;
  disabled?: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      aria-label={label}
      aria-pressed={active}
      title={label}
      disabled={disabled}
      onClick={onClick}
      className={`board-tile grid h-[52px] place-items-center rounded-board-tile ${
        narrow ? "w-11" : "w-[52px]"
      } ${
        active
          ? "bg-amber-soft text-amber"
          : "text-ink-3 hover:bg-hover disabled:text-ink-4 disabled:hover:bg-transparent"
      }`}
    >
      {children}
    </button>
  );
}

function TextButton({ label, onClick }: { label: string; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="board-tile h-[52px] rounded-board-tile px-3.5 text-board-row text-ink-2 hover:bg-hover hover:text-ink"
    >
      {label}
    </button>
  );
}

function Divider() {
  return <span className="mx-1 h-7 w-px shrink-0 bg-line" />;
}
