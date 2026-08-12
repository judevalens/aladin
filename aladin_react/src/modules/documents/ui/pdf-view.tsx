import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import * as pdfjs from "pdfjs-dist";
import { TextLayer } from "pdfjs-dist";
import type { PDFDocumentProxy, PDFPageProxy, RenderTask } from "pdfjs-dist";
import workerUrl from "pdfjs-dist/build/pdf.worker.min.mjs?url";

import { cn } from "@/lib/utils";

import "./pdf-text-layer.css";

pdfjs.GlobalWorkerOptions.workerSrc = workerUrl;

/**
 * The page renderer.
 *
 * **Why not an `<iframe>`.** An iframe pointed at a PDF does not show a page — it shows the
 * *browser's own PDF viewer*: a grey toolbar, foreign fonts, its own scrollbar and zoom UI,
 * none of it themeable. That is somebody else's interface sitting inside the app, and it is
 * the reason the first version read as unfinished. Rendering to canvas costs a dependency
 * and buys every pixel back.
 *
 * It is deliberately **presentational** — a URL and a page number in, a visible page out,
 * no app context. That is what lets it be verified in isolation, and it is what will let
 * §13f's region boxes be overlaid on it later: the boxes are stored in PDF points, which is
 * exactly the space `viewport.convertToViewportRectangle` speaks.
 *
 * Only visible pages hold a canvas. A 280-page scan is ~35MB and rasterising all of it at
 * once would be hundreds of megabytes of bitmap, so slots keep their geometry and give the
 * pixels back when they leave.
 */
export interface PdfViewProps {
  url: string;
  /** Scroll here when it changes. Not a controlled value — the user may scroll away. */
  targetPage?: number | null;
  /** Fires as the reader scrolls, so chrome outside can follow along. */
  onVisiblePageChange?: (page: number) => void;
  zoom?: number;
  className?: string;
}

/** How far outside the viewport a page still earns its pixels. */
const RENDER_MARGIN = "200% 0px";
/** Fit-width target. Wider than this and lines get too long to track comfortably. */
const MAX_PAGE_WIDTH = 980;
/**
 * The stage's side padding, in px. Declared once because it is used twice — as the class
 * below and as the width the page must leave for it — and the two drifting apart is what
 * produces an off-centre page with a gutter wider on one side than the other.
 */
const STAGE_PADDING = 40;

export function PdfView({ url, targetPage, onVisiblePageChange, zoom = 1, className }: PdfViewProps) {
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const docRef = useRef<PDFDocumentProxy | null>(null);
  const [pageCount, setPageCount] = useState(0);
  const [aspect, setAspect] = useState(1.294); // height / width; corrected on load
  const [width, setWidth] = useState(0);
  const [visible, setVisible] = useState<Set<number>>(new Set());
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    const task = pdfjs.getDocument({ url });
    task.promise
      .then(async (doc) => {
        // The loading task owns the document; destroying it in cleanup tears both down.
        if (cancelled) return;
        docRef.current = doc;
        setPageCount(doc.numPages);
        const first = await doc.getPage(1);
        const viewport = first.getViewport({ scale: 1 });
        if (!cancelled) setAspect(viewport.height / viewport.width);
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(e instanceof Error ? e.message : "Couldn't open this PDF.");
      });
    return () => {
      cancelled = true;
      void task.destroy();
      docRef.current = null;
    };
  }, [url]);

  // Fit-to-width, measured rather than assumed, so the page tracks the pane as it resizes.
  useEffect(() => {
    const node = scrollRef.current;
    if (!node) return;
    // Ignore zero. A hidden pane — collapsed panel, background tab — measures 0, and
    // acting on that would cancel every in-flight render and blank the pages we already
    // have. Keeping the last good width means coming back from hidden costs nothing.
    const measure = () => setWidth((current) => node.clientWidth || current);
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(node);
    return () => observer.disconnect();
  }, []);

  const pageWidth = useMemo(() => {
    if (!width) return 0;
    return Math.max(240, Math.min(width - STAGE_PADDING * 2, MAX_PAGE_WIDTH) * zoom);
  }, [width, zoom]);

  const observePage = useCallback(
    (page: number, node: HTMLElement | null) => {
      if (!node) return;
      const observer = new IntersectionObserver(
        (entries) => {
          for (const entry of entries) {
            setVisible((current) => {
              const has = current.has(page);
              if (entry.isIntersecting === has) return current;
              const next = new Set(current);
              if (entry.isIntersecting) next.add(page);
              else next.delete(page);
              return next;
            });
          }
        },
        { root: scrollRef.current, rootMargin: RENDER_MARGIN },
      );
      observer.observe(node);
      return () => observer.disconnect();
    },
    [],
  );

  // The page you're actually looking at: whatever crosses the upper third of the pane.
  useEffect(() => {
    const node = scrollRef.current;
    if (!node || !onVisiblePageChange) return;
    let frame = 0;
    const onScroll = () => {
      if (frame) return;
      frame = requestAnimationFrame(() => {
        frame = 0;
        const probe = node.getBoundingClientRect().top + node.clientHeight / 3;
        const slots = node.querySelectorAll<HTMLElement>("[data-pdf-page]");
        for (const slot of slots) {
          const box = slot.getBoundingClientRect();
          if (box.bottom >= probe) {
            onVisiblePageChange(Number(slot.dataset.pdfPage));
            return;
          }
        }
      });
    };
    node.addEventListener("scroll", onScroll, { passive: true });
    return () => {
      node.removeEventListener("scroll", onScroll);
      if (frame) cancelAnimationFrame(frame);
    };
  }, [onVisiblePageChange, pageCount]);

  useEffect(() => {
    if (!targetPage || !scrollRef.current) return;
    scrollRef.current
      .querySelector(`[data-pdf-page="${targetPage}"]`)
      ?.scrollIntoView({ behavior: "smooth", block: "start" });
  }, [targetPage, pageCount, pageWidth]);

  if (error) {
    return (
      <div className={cn("flex items-center justify-center bg-rail p-8", className)}>
        <p className="max-w-sm text-center text-[13px] leading-relaxed text-ink-4">{error}</p>
      </div>
    );
  }

  return (
    <div ref={scrollRef} className={cn("overflow-auto bg-rail", className)}>
      <div
        className="flex flex-col items-center gap-6"
        style={{ paddingInline: STAGE_PADDING, paddingBlock: STAGE_PADDING - 8 }}
      >
        {Array.from({ length: pageCount }, (_, index) => index + 1).map((page) => (
          <PageSlot
            key={page}
            page={page}
            width={pageWidth}
            aspect={aspect}
            active={visible.has(page)}
            doc={docRef.current}
            observe={observePage}
          />
        ))}
      </div>
    </div>
  );
}

/**
 * One page. Holds its geometry always and its pixels only while near the viewport, so the
 * scrollbar stays honest without paying for 280 bitmaps.
 */
function PageSlot({
  page,
  width,
  aspect,
  active,
  doc,
  observe,
}: {
  page: number;
  width: number;
  aspect: number;
  active: boolean;
  doc: PDFDocumentProxy | null;
  observe: (page: number, node: HTMLElement | null) => (() => void) | undefined;
}) {
  const slotRef = useRef<HTMLDivElement | null>(null);
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const textRef = useRef<HTMLDivElement | null>(null);
  const [rendered, setRendered] = useState(false);

  useEffect(() => observe(page, slotRef.current), [observe, page]);

  useEffect(() => {
    if (!active || !doc || !width) return;
    let cancelled = false;
    let task: RenderTask | null = null;
    let proxy: PDFPageProxy | null = null;

    void (async () => {
      try {
        proxy = await doc.getPage(page);
        if (cancelled) return;
        const base = proxy.getViewport({ scale: 1 });
        const canvas = canvasRef.current;
        if (!canvas) return;

        // Two viewports, deliberately. The canvas is rasterised at device resolution —
        // a canvas sized in CSS pixels is visibly soft on a retina display, and "soft"
        // reads as "low quality" long before anyone diagnoses why. The text layer is
        // positioned in CSS pixels, so it gets the unscaled one.
        const ratio = Math.min(window.devicePixelRatio || 1, 2);
        const cssViewport = proxy.getViewport({ scale: width / base.width });
        const deviceViewport = proxy.getViewport({ scale: (width / base.width) * ratio });
        canvas.width = Math.floor(deviceViewport.width);
        canvas.height = Math.floor(deviceViewport.height);

        task = proxy.render({ canvas, viewport: deviceViewport });
        await task.promise;
        if (cancelled) return;
        setRendered(true);

        // The selectable layer. Without it the page is a picture, and quoting a paper you
        // are reading is the whole point of keeping it in here.
        const container = textRef.current;
        if (container) {
          container.replaceChildren();
          // pdf.js positions each run against this, not against the element's box.
          container.style.setProperty("--total-scale-factor", String(cssViewport.scale));
          const textLayer = new TextLayer({
            textContentSource: await proxy.getTextContent(),
            container,
            viewport: cssViewport,
          });
          await textLayer.render();
        }
      } catch (e) {
        // A page that won't render leaves its slot empty rather than taking the view down —
        // but it says so, because a silently blank page is indistinguishable from a bug.
        if (!(e instanceof Error && e.name === "RenderingCancelledException")) {
          console.error(`[pdf] page ${page} failed to render`, e);
        }
      }
    })();

    return () => {
      cancelled = true;
      task?.cancel();
      proxy?.cleanup();
      setRendered(false);
      textRef.current?.replaceChildren();
    };
  }, [active, doc, page, width]);

  const height = width * aspect;

  return (
    <div ref={slotRef} data-pdf-page={page} className="scroll-mt-6">
      {/* No white plate behind the canvas: 280 blank white rectangles on load is a wall of
          glare in a dark workspace. The page's own pixels arrive with the render; until then
          the slot stays a dark placeholder holding its geometry. */}
      <div
        className="relative overflow-hidden rounded-[3px] bg-card shadow-[0_1px_2px_rgb(0_0_0/0.35),0_10px_28px_-12px_rgb(0_0_0/0.55)]"
        style={{ width: width || undefined, height: height || undefined }}
      >
        <canvas ref={canvasRef} className={cn("block h-full w-full transition-opacity duration-150", rendered ? "opacity-100" : "opacity-0")} />
        {/* pdf.js's own class name — the stylesheet's selectors are scoped to it. */}
        <div ref={textRef} className="textLayer" />
        {!rendered ? <div className="absolute inset-0 animate-pulse bg-card" /> : null}
      </div>
      <div className="pt-1.5 text-center font-mono text-[10px] tabular-nums text-ink-4">{page}</div>
    </div>
  );
}
