import { useLayoutEffect, useRef, useState } from "react";

import {
  BOARD_INK_SWATCHES,
  BOARD_WEIGHTS,
  type BoardWeightIndex,
} from "../domain/board-tools";
import type { BoardInkColor } from "../domain/board-theme";

/**
 * Pencil style — the three inks and the three weights, folded out of the dock into one
 * island above the style tile. Every swatch and dot is a 44pt target; the chosen swatch
 * wears the handoff's double ring.
 */
export function StylePopover({
  centerX,
  inkColor,
  weight,
  onPickColor,
  onPickWeight,
}: {
  /** The style tile's centre, in the dock's coordinate space. */
  centerX: number;
  inkColor: BoardInkColor;
  weight: BoardWeightIndex;
  onPickColor: (color: BoardInkColor) => void;
  onPickWeight: (weight: BoardWeightIndex) => void;
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
      role="group"
      aria-label="Ink colour and weight"
      className="board-island board-island--popover absolute bottom-[calc(100%+10px)] flex -translate-x-1/2 items-center gap-0.5 p-1.5"
      style={{ left }}
    >
      {BOARD_INK_SWATCHES.map((swatch) => (
        <button
          key={swatch.id}
          type="button"
          aria-label={swatch.label}
          aria-pressed={inkColor === swatch.id}
          title={swatch.label}
          onClick={() => onPickColor(swatch.id)}
          className="board-tile grid h-11 w-11 place-items-center rounded-control hover:bg-hover"
        >
          <span
            className="h-[19px] w-[19px] rounded-full transition-shadow"
            style={{
              background: swatch.cssVar,
              boxShadow:
                inkColor === swatch.id
                  ? "0 0 0 2.5px var(--bg), 0 0 0 4.5px var(--ink-2)"
                  : "0 0 0 1px rgb(var(--line))",
            }}
          />
        </button>
      ))}
      <span className="mx-1 h-7 w-px bg-line" />
      {BOARD_WEIGHTS.map((w, index) => (
        <button
          key={w.size}
          type="button"
          aria-label={`Stroke weight ${index + 1}`}
          aria-pressed={weight === index}
          onClick={() => onPickWeight(index as BoardWeightIndex)}
          className={`board-tile grid h-11 w-11 place-items-center rounded-control hover:bg-hover ${
            weight === index ? "bg-sel" : ""
          }`}
        >
          <span
            className={`rounded-full ${weight === index ? "bg-ink" : "bg-ink-3"}`}
            style={{ width: w.dotPx, height: w.dotPx }}
          />
        </button>
      ))}
    </div>
  );
}
