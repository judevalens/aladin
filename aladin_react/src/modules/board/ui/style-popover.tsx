import {
  BOARD_INK_SWATCHES,
  BOARD_WEIGHTS,
  type BoardWeightIndex,
} from "../domain/board-tools";
import type { BoardInkColor } from "../domain/board-theme";

/**
 * Pencil style — the three inks, the three weights, and whether a finger may draw, folded
 * out of the dock into one island above the style tile. Every swatch and dot is a 44pt
 * target; the chosen swatch wears the handoff's double ring.
 */
export function StylePopover({
  inkColor,
  weight,
  drawWithFinger,
  onPickColor,
  onPickWeight,
  onToggleDrawWithFinger,
}: {
  inkColor: BoardInkColor;
  weight: BoardWeightIndex;
  drawWithFinger: boolean;
  onPickColor: (color: BoardInkColor) => void;
  onPickWeight: (weight: BoardWeightIndex) => void;
  onToggleDrawWithFinger: () => void;
}) {
  return (
    <div
      role="group"
      aria-label="Ink colour and weight"
      className="board-island board-island--popover absolute bottom-[calc(100%+10px)] right-0 flex items-center gap-0.5 p-1.5"
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
      <span className="mx-1 h-7 w-px bg-line" />
      {/* Freeform's "Draw with Finger": off, a finger pans while the Pencil draws. */}
      <button
        type="button"
        aria-pressed={drawWithFinger}
        onClick={onToggleDrawWithFinger}
        className={`board-tile h-11 rounded-control px-3 text-board-label ${
          drawWithFinger ? "bg-sel text-ink" : "text-ink-3 hover:bg-hover"
        }`}
      >
        Finger draws
      </button>
    </div>
  );
}
