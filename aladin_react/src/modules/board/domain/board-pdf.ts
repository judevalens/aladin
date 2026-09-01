import type { PDFDocumentLoadingTask, PDFDocumentProxy } from "pdfjs-dist";

import type { ApiClient } from "@/shared/api/client";

export interface BoardPdfLease {
  document: Promise<PDFDocumentProxy>;
  release(): void;
}

interface PdfEntry {
  document: Promise<PDFDocumentProxy>;
  users: number;
  controller: AbortController;
  task?: PDFDocumentLoadingTask;
  timer?: ReturnType<typeof setTimeout>;
}

/** One authenticated download / worker document for all windows onto the same PDF.
 * Only view state is persisted; binary resources live while thumbnails use them. */
export function createBoardPdfCache(client: ApiClient) {
  const entries = new Map<string, PdfEntry>();

  return (artifactId: string): BoardPdfLease => {
    let entry = entries.get(artifactId);
    if (!entry) {
      const controller = new AbortController();
      const next: PdfEntry = {
        users: 0, controller,
        document: Promise.resolve().then(async () => {
          // Lazy-load the same renderer used by the full PDF reader.
          const [pdfjs, worker, assets, blob] = await Promise.all([
            import("pdfjs-dist"),
            import("pdfjs-dist/build/pdf.worker.min.mjs?url"),
            import("./board-pdf-assets"),
            client.fetchBlob(`/api/artifacts/${encodeURIComponent(artifactId)}/resource`, { signal: controller.signal }),
          ]);
          const data = await blob.arrayBuffer();
          controller.signal.throwIfAborted();
          pdfjs.GlobalWorkerOptions.workerSrc = worker.default;
          next.task = pdfjs.getDocument({ data, useWorkerFetch: false, BinaryDataFactory: assets.BoardPdfBinaryDataFactory });
          return next.task.promise;
        }),
      };
      entry = next;
      entries.set(artifactId, entry);
    }
    const held = entry;
    clearTimeout(held.timer);
    held.users += 1;
    let released = false;
    return {
      document: held.document,
      release() {
        if (released) return;
        released = true;
        held.users -= 1;
        if (held.users) return;
        // Let React's effect replay / a replacement window reclaim the resource.
        held.timer = setTimeout(() => {
          entries.delete(artifactId);
          held.controller.abort();
          void held.task?.destroy().catch(() => {});
        }, 0);
      },
    };
  };
}
