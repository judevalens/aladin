import { useCallback, useEffect, useRef } from "react";
import type { Editor, TLShape } from "tldraw";

/** Rounded-rect indicator path — the amber selection outline tldraw draws around a shape.
 *  The radius is the same `--radius-board-card` the objects draw with. */
export const BOARD_CARD_RADIUS = 10;

/** Cosmetic metadata is supported by the existing record schema on both clients/server. */
export function boardObjectClass(shape: TLShape) {
  const tint = ["butter", "sage", "lilac"].includes(String(shape.meta.boardTint)) ? shape.meta.boardTint : "neutral";
  return "board-object rs-tint--" + tint;
}

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
  onNeedHeight,
  className,
}: {
  editor: Editor;
  value: string;
  onChange: (next: string) => void;
  /**
   * Called with the object's content height in PAGE units after the text re-flowed — the
   * shape grows its `h` to at least that, so typed text never scrolls out of sight inside
   * a box with no scrollbar. Idempotent by construction (a height, not a delta): the
   * HTMLContainer is scaled by transform, so `scrollHeight` is already in page units.
   */
  onNeedHeight?: (heightPagePx: number) => void;
  className?: string;
}) {
  const ref = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    el.focus();
    el.setSelectionRange(el.value.length, el.value.length);
  }, []);

  // Autosize to the content, then tell the object how tall its content now is. Runs on
  // the VALUE and on a real resize — not on every render — so a stale layout can never
  // compound; and never before the box has a width (at mount the shape may not be laid out
  // yet, and a zero-width textarea wraps every character onto its own line).
  const needHeightRef = useRef(onNeedHeight);
  needHeightRef.current = onNeedHeight;
  const fit = useCallback(() => {
    const el = ref.current;
    if (!el || el.offsetWidth < 40) return;
    el.style.height = "0px";
    el.style.height = `${el.scrollHeight}px`;
    const card = el.closest<HTMLElement>(".board-object");
    if (card) needHeightRef.current?.(card.scrollHeight);
  }, []);
  useEffect(fit, [value, fit]);
  useEffect(() => {
    const el = ref.current;
    if (!el?.parentElement) return;
    const observer = new ResizeObserver(fit);
    observer.observe(el.parentElement);
    return () => observer.disconnect();
  }, [fit]);

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
