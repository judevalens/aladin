import { useEditor, useValue } from "tldraw";

import { PAPER_GAP, PAPER_PAGE, paperPageCount, useBoardPaper } from "../domain/board-paper";

/**
 * The paper itself — page rectangles with rule lines and numbers, rendered in canvas
 * space (`components.OnTheCanvas`) so they scroll and zoom with the ink. Renders nothing
 * on a plane board; on paper it always shows one blank page past the ink, so the paper
 * grows as you write (the camera bounds follow in BoardCanvas).
 */
export function PaperPages() {
  const paper = useBoardPaper();
  const editor = useEditor();
  const pageCount = useValue(
    "paper-pages",
    () => {
      if (!paper.paged) return 0;
      const bounds = editor.getCurrentPageBounds();
      return paperPageCount(bounds ? bounds.maxY : 0);
    },
    [editor, paper.paged],
  );
  if (!paper.paged || pageCount === 0) return null;

  return (
    <>
      {Array.from({ length: pageCount }, (_, index) => (
        <div
          key={index}
          className="pointer-events-none absolute rounded-tap border border-line-2 bg-card"
          style={{
            left: 0,
            top: index * (PAPER_PAGE.h + PAPER_GAP),
            width: PAPER_PAGE.w,
            height: PAPER_PAGE.h,
            // Rule lines, faint, in the writing zone only — margins stay clean.
            backgroundImage:
              "repeating-linear-gradient(to bottom, transparent, transparent 39px, rgb(var(--line-2)) 39px, rgb(var(--line-2)) 40px)",
            backgroundClip: "content-box",
            paddingTop: 64,
            paddingBottom: 48,
          }}
        >
          <span className="absolute bottom-3 right-5 font-mono text-meta text-ink-4">
            {index + 1}
          </span>
        </div>
      ))}
    </>
  );
}
