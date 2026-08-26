/**
 * The capture gesture's pure half: given the live selection and the reader's scroll
 * container, decide whether it is an excerptable span — non-empty, inside ONE rendered
 * PDF page — and return its text, page number and rect (viewport coords, for the chip).
 *
 * Pure over a narrow interface so it tests without a real browser Selection.
 */
export interface SelectionLike {
  isCollapsed: boolean;
  rangeCount: number;
  toString(): string;
  getRangeAt(index: number): {
    commonAncestorContainer: Node;
    getBoundingClientRect(): { left: number; top: number; right: number; bottom: number };
  };
}

export interface SelectionExcerpt {
  text: string;
  page: number;
  rect: { left: number; top: number; right: number; bottom: number };
}

export function selectionExcerpt(
  selection: SelectionLike | null,
  root: HTMLElement,
): SelectionExcerpt | null {
  if (!selection || selection.isCollapsed || selection.rangeCount === 0) return null;
  const text = selection.toString().trim();
  if (!text) return null;
  const range = selection.getRangeAt(0);
  const container = range.commonAncestorContainer;
  const element =
    container.nodeType === Node.ELEMENT_NODE
      ? (container as HTMLElement)
      : container.parentElement;
  if (!element || !root.contains(element)) return null;
  // One page only: a selection spanning two pages has no single cite. The common
  // ancestor of a cross-page range is the page LIST, which carries no data-pdf-page.
  const slot = element.closest<HTMLElement>("[data-pdf-page]");
  if (!slot) return null;
  const page = Number(slot.dataset.pdfPage);
  if (!Number.isFinite(page) || page < 1) return null;
  return { text, page, rect: range.getBoundingClientRect() };
}
