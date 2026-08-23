import { useEffect, useSyncExternalStore } from "react";
import { BaseBoxShapeUtil, HTMLContainer } from "tldraw";
import type { TLIndicatorPath } from "tldraw";

import { useBoardContent, type DocPageContent } from "../domain/board-content";
import { DOCK_PATHS, DockIcon } from "../ui/dock-icons";
import { DOC_WINDOW_DEFAULTS, docWindowProps, type DocWindowShape } from "./shape-types";
import { roundedIndicator, tappable } from "./shape-shared";

const KIND_ICONS: Record<string, string> = {
  file: DOCK_PATHS.file,
  note: DOCK_PATHS.note,
  link: DOCK_PATHS.link,
};

/**
 * A live window onto a workspace artifact. The page is THIS window's own — two windows
 * on one PDF are two reading positions, one document (product rule 1). The body resolves
 * read-live through BoardContentContext; the shape stores no content.
 */
export class DocWindowShapeUtil extends BaseBoxShapeUtil<DocWindowShape> {
  static override type = "aladin-doc" as const;
  static override props = docWindowProps;

  override getDefaultProps(): DocWindowShape["props"] {
    return { ...DOC_WINDOW_DEFAULTS };
  }

  override getIndicatorPath(shape: DocWindowShape): TLIndicatorPath {
    return roundedIndicator(shape.props.w, shape.props.h);
  }

  override component(shape: DocWindowShape) {
    const source = useBoardContent();
    const { artifactId, page } = shape.props;
    const content = useSyncExternalStore<DocPageContent | null>(
      (onChange) => (source ? source.subscribe(artifactId, page, onChange) : () => {}),
      () => (source ? source.get(artifactId, page) : null),
    );
    const pageCount =
      content?.state === "ready" ? content.pageCount : shape.props.pageCount;

    // Persist the resolved page count into the window's props (once per change), so the
    // selection bar and a cold reload agree with the live footer.
    useEffect(() => {
      if (content?.state === "ready" && content.pageCount !== shape.props.pageCount) {
        this.editor.updateShape({
          id: shape.id,
          type: shape.type,
          props: { pageCount: content.pageCount },
        });
      }
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [content?.state === "ready" ? content.pageCount : null, shape.props.pageCount]);

    const setPage = (next: number) => {
      const clamped = Math.max(1, Math.min(pageCount, next));
      if (clamped === page) return;
      this.editor.updateShape({
        id: shape.id,
        type: shape.type,
        props: { page: clamped, pageCount },
      });
    };

    return (
      <HTMLContainer>
        <div className="board-object flex flex-col overflow-hidden">
          <div className="flex items-center gap-2.5 border-b border-line-2 px-4 py-3">
            <span className="shrink-0 text-ink-3">
              <DockIcon d={KIND_ICONS[shape.props.artifactKind] ?? KIND_ICONS.file} size={17} strokeWidth={1.75} />
            </span>
            <span className="min-w-0 truncate text-board-row text-ink-2">{shape.props.title}</span>
            <span className="ml-auto flex shrink-0 items-center gap-1.5 rounded-chip bg-amber-soft px-2 py-0.5 font-mono text-board-meta text-amber">
              <span className="h-1.5 w-1.5 rounded-full bg-amber" />
              live
            </span>
          </div>
          <div className="min-h-0 flex-1 overflow-hidden px-4 py-3.5">
            {content?.state === "ready" ? (
              <>
                <div className="font-mono text-board-meta uppercase tracking-wider text-ink-4">
                  {content.sourceLine}
                </div>
                <p className="mt-2 font-serif text-board-row leading-[1.65] text-ink-2">
                  {content.excerpt}
                </p>
              </>
            ) : (
              <div className="font-mono text-small text-ink-4">
                {content?.state === "missing"
                  ? "the artifact is gone — the window stays"
                  : "live window — resolving…"}
              </div>
            )}
          </div>
          <div className="flex items-center gap-0.5 px-1.5 pb-1 font-mono text-board-meta text-ink-4">
            <button
              type="button"
              aria-label="Previous page"
              {...tappable(this.editor, () => setPage(page - 1))}
              className="board-tile grid h-11 w-11 place-items-center rounded-control text-ink-3 hover:bg-hover disabled:text-ink-4"
              disabled={page <= 1}
            >
              <DockIcon d={DOCK_PATHS.chevronLeft} size={15} strokeWidth={2} />
            </button>
            <span>
              page {page} / {pageCount}
            </span>
            <button
              type="button"
              aria-label="Next page"
              {...tappable(this.editor, () => setPage(page + 1))}
              className="board-tile grid h-11 w-11 place-items-center rounded-control text-ink-3 hover:bg-hover disabled:text-ink-4"
              disabled={page >= pageCount}
            >
              <DockIcon d={DOCK_PATHS.chevronRight} size={15} strokeWidth={2} />
            </button>
            <span className="ml-auto pr-2.5">this window's own page</span>
          </div>
        </div>
      </HTMLContainer>
    );
  }
}
