import { useEffect, useRef, useState } from "react";
import type { PDFDocumentProxy, RenderTask } from "pdfjs-dist";

import type { BoardContentSource } from "../domain/board-content";
import { DOCK_PATHS, DockIcon } from "./dock-icons";

/** A page image, not an embedded reader: dragging/zooming remains the board's job. */
export function BoardPdfThumbnail({ source, artifactId, page, title, onPageCount }: {
  source: BoardContentSource | null;
  artifactId: string;
  page: number;
  title: string;
  onPageCount(count: number): void;
}) {
  const container = useRef<HTMLDivElement>(null);
  const canvas = useRef<HTMLCanvasElement>(null);
  const [visible, setVisible] = useState(false);
  const [doc, setDoc] = useState<PDFDocumentProxy | null>(null);
  const [failed, setFailed] = useState(false);
  const [renderedPage, setRenderedPage] = useState<number | null>(null);
  const countCallback = useRef(onPageCount);
  countCallback.current = onPageCount;

  useEffect(() => {
    if (!container.current) return;
    const observer = new IntersectionObserver(([entry]) => setVisible(entry.isIntersecting), { rootMargin: "200px" });
    observer.observe(container.current);
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    if (!visible) return;
    let cancelled = false;
    setFailed(false);
    const lease = source?.acquirePdf?.(artifactId);
    if (!lease) { setFailed(true); return; }
    void lease.document.then((document) => {
      if (cancelled) return;
      setDoc(document);
      countCallback.current(document.numPages);
    }).catch(() => { if (!cancelled) setFailed(true); });
    return () => {
      cancelled = true;
      setDoc(null);
      lease.release();
    };
  }, [source, artifactId, visible]);

  useEffect(() => {
    if (!visible || !doc) return;
    let cancelled = false;
    let task: RenderTask | undefined;
    const element = canvas.current;
    setRenderedPage(null);
    setFailed(false);
    void (async () => {
      try {
        const pdfPage = await doc.getPage(Math.max(1, Math.min(doc.numPages, page)));
        if (cancelled || !element) return;
        const base = pdfPage.getViewport({ scale: 1 });
        // Crisp at ordinary board zoom, bounded even for poster-sized PDFs.
        const viewport = pdfPage.getViewport({ scale: Math.min(800 / base.width, 1100 / base.height) });
        element.width = Math.ceil(viewport.width);
        element.height = Math.ceil(viewport.height);
        task = pdfPage.render({ canvas: element, viewport });
        await task.promise;
        if (!cancelled) setRenderedPage(page);
      } catch {
        if (!cancelled) setFailed(true);
      }
    })();
    return () => {
      cancelled = true;
      task?.cancel();
      // A duplicate window may still be rendering the same page, so don't call
      // page.cleanup() here. The shared document is destroyed after its last lease.
      if (element) { element.width = 0; element.height = 0; }
    };
  }, [doc, page, visible]);

  const ready = visible && !!doc && renderedPage === page && !failed;
  return (
    <div ref={container} className="board-pdf-preview" aria-busy={!ready && !failed}>
      <canvas ref={canvas} role="img" aria-label={`${title || "PDF"}, page ${page}`} hidden={!ready} />
      {!ready && <div className="board-pdf-placeholder" role="status">
        <DockIcon d={DOCK_PATHS.file} size={25} strokeWidth={1.2} />
        <span>{failed ? "Preview unavailable" : "Loading PDF preview…"}</span>
        {failed && <small>Your PDF is still linked. Open the source to view it.</small>}
      </div>}
    </div>
  );
}
