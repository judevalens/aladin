import { useEffect, useState, useSyncExternalStore } from "react";
import { BaseBoxShapeUtil, HTMLContainer } from "tldraw";
import type { TLIndicatorPath } from "tldraw";

import { useBoardContent, type DocPageContent } from "../domain/board-content";
import { DOCK_PATHS, DockIcon } from "../ui/dock-icons";
import { BoardPdfThumbnail } from "../ui/board-pdf-thumbnail";
import { DOC_WINDOW_DEFAULTS, docWindowProps, type DocWindowShape } from "./shape-types";
import { boardObjectClass, roundedIndicator, tappable } from "./shape-shared";

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
    const [pdfInfo, setPdfInfo] = useState<{ artifactId: string; pageCount: number } | null>(null);
    const content = useSyncExternalStore<DocPageContent | null>(
      (onChange) => (source ? source.subscribe(artifactId, page, onChange, shape.props.artifactKind) : () => {}),
      () => (source ? source.get(artifactId, page) : null),
    );
    const isPdf = content?.state === "ready" && content.format === "pdf";
    const pageCount = isPdf
      ? (pdfInfo?.artifactId === artifactId ? pdfInfo.pageCount : shape.props.pageCount)
      : content?.state === "ready" ? content.pageCount : shape.props.pageCount;

    // Persist the resolved page count into the window's props (once per change), so the
    // selection bar and a cold reload agree with the live footer.
    useEffect(() => {
      const clamped = Math.max(1, Math.min(pageCount, page));
      if (content?.state === "ready" && (pageCount !== shape.props.pageCount || clamped !== page)) {
        this.editor.updateShape({
          id: shape.id,
          type: shape.type,
          props: { pageCount, page: clamped },
        });
      }
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [content?.state, pageCount, shape.props.pageCount, page]);

    const setPage = (next: number) => {
      const clamped = Math.max(1, Math.min(pageCount, next));
      if (clamped === page) return;
      this.editor.updateShape({
        id: shape.id,
        type: shape.type,
        props: { page: clamped, pageCount },
      });
    };

    const kind = shape.props.artifactKind;
    const kindLabel = isPdf ? "PDF" : kind === "file" ? "File" : kind === "app" ? "Aladin instrument" : kind === "link" ? "Saved link" : kind === "voice" ? "Voice note" : "Workspace note";
    return (
      <HTMLContainer>
        <article className={boardObjectClass(shape) + " rs-object board-source-object " + (isPdf ? "rs-object--paper board-source-object--pdf" : "rs-object--document")} aria-label={kindLabel + ": " + shape.props.title}>
          {isPdf && <BoardPdfThumbnail
            key={artifactId}
            source={source}
            artifactId={artifactId}
            page={page}
            title={shape.props.title}
            onPageCount={(count) => setPdfInfo((current) => current?.artifactId === artifactId && current.pageCount === count ? current : { artifactId, pageCount: count })}
          />}
          <div className="rs-object-content">
            <div className="rs-object-meta">
              <span className="rs-kind-icon"><DockIcon d={KIND_ICONS[kind] ?? KIND_ICONS.file} size={14} strokeWidth={1.65} /></span>
              <span>{kindLabel}</span>
            </div>
            <h2>{shape.props.title || kindLabel}</h2>
            {!isPdf && (content?.state === "ready" ? <>
              <div className="board-source-section">{content.sourceLine}</div>
              <p className="board-source-excerpt">{content.excerpt}</p>
            </> : <p>{content?.state === "missing" ? "This source is unavailable. Your board reference stays here." : "Loading source…"}</p>)}
            {(isPdf || pageCount > 1) && <div className="board-document-pages">
              <button type="button" aria-label="Previous page" {...tappable(this.editor, () => setPage(page - 1))} disabled={page <= 1}><DockIcon d={DOCK_PATHS.chevronLeft} size={14} /></button>
              <span>Page {page} / {pageCount}</span>
              <button type="button" aria-label="Next page" {...tappable(this.editor, () => setPage(page + 1))} disabled={page >= pageCount}><DockIcon d={DOCK_PATHS.chevronRight} size={14} /></button>
            </div>}
          </div>
        </article>
      </HTMLContainer>
    );
  }
}
