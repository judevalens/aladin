import { useEffect, useRef } from "react";
import type { Editor } from "tldraw";

/** Rounded-rect indicator path — the amber selection outline tldraw draws around a shape.
 *  The radius is the same `--radius-board-card` the objects draw with. */
export const BOARD_CARD_RADIUS = 18;

export function roundedIndicator(w: number, h: number, radius = BOARD_CARD_RADIUS): Path2D {
  const path = new Path2D();
  path.roundRect(0, 0, w, h, radius);
  return path;
}

/**
 * The in-place text editor rendered while a shape is in tldraw's editing state.
 * Entering editing does NOT move focus into shape content (spike trap #5) — the
 * textarea must take it itself; once a real input is activeElement, tldraw stops
 * claiming keys. It inherits the face of the text it replaces (`.board-object textarea`).
 */
export function ShapeTextArea({
  editor,
  value,
  onChange,
  className,
}: {
  editor: Editor;
  value: string;
  onChange: (next: string) => void;
  className?: string;
}) {
  const ref = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    el.focus();
    el.setSelectionRange(el.value.length, el.value.length);
  }, []);

  return (
    <textarea
      ref={ref}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      onPointerDown={(e) => editor.markEventAsHandled(e)}
      onKeyDown={(e) => {
        // Escape hands control back to tldraw (which exits editing); everything else is ours.
        if (e.key !== "Escape") e.stopPropagation();
      }}
      className={className}
      rows={1}
    />
  );
}

/** Props for a small always-tappable control inside a shape (checkbox, pager arrows). */
export function tappable(editor: Editor, onTap: () => void) {
  return {
    onPointerDown: (e: React.PointerEvent) => editor.markEventAsHandled(e),
    onClick: (e: React.MouseEvent) => {
      e.stopPropagation();
      onTap();
    },
    style: { pointerEvents: "all" as const, cursor: "pointer" as const },
  };
}
