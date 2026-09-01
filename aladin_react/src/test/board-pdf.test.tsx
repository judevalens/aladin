import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { PDFDocumentProxy } from "pdfjs-dist";

import { createBoardPdfCache } from "@/modules/board/domain/board-pdf";
import { BoardPdfBinaryDataFactory } from "@/modules/board/domain/board-pdf-assets";
import type { BoardContentSource } from "@/modules/board/domain/board-content";
import { BoardPdfThumbnail } from "@/modules/board/ui/board-pdf-thumbnail";
import type { ApiClient } from "@/shared/api/client";

const pdf = vi.hoisted(() => ({ getDocument: vi.fn(), GlobalWorkerOptions: { workerSrc: "" } }));
vi.mock("pdfjs-dist", () => pdf);

afterEach(() => { cleanup(); vi.useRealTimers(); vi.unstubAllGlobals(); });

describe("shared board PDF resources", () => {
  it("authenticates once for duplicate windows and releases after the last one", async () => {
    vi.useFakeTimers();
    const document = { numPages: 8 };
    const destroy = vi.fn().mockResolvedValue(undefined);
    pdf.getDocument.mockReturnValue({ promise: Promise.resolve(document), destroy });
    const fetchBlob = vi.fn().mockResolvedValue({ arrayBuffer: async () => new ArrayBuffer(4) });
    const acquire = createBoardPdfCache({ fetchBlob } as unknown as ApiClient);
    const first = acquire("paper/id");
    const second = acquire("paper/id");
    expect(first.document).toBe(second.document);
    expect(await first.document).toBe(document);
    expect(pdf.getDocument).toHaveBeenCalledWith({ data: expect.any(ArrayBuffer), useWorkerFetch: false, BinaryDataFactory: BoardPdfBinaryDataFactory });
    expect(fetchBlob).toHaveBeenCalledExactlyOnceWith("/api/artifacts/paper%2Fid/resource", { signal: expect.any(AbortSignal) });
    first.release();
    await vi.runAllTimersAsync();
    expect(destroy).not.toHaveBeenCalled();
    second.release();
    second.release();
    await vi.runAllTimersAsync();
    expect(destroy).toHaveBeenCalledTimes(1);
    expect(fetchBlob.mock.calls[0][1].signal.aborted).toBe(true);
  });

  it("reuses an effect replay, then permits retry after a failed resource is released", async () => {
    vi.useFakeTimers();
    const fetchBlob = vi.fn().mockRejectedValue(new Error("offline"));
    const acquire = createBoardPdfCache({ fetchBlob } as unknown as ApiClient);
    const first = acquire("paper");
    first.release();
    const replay = acquire("paper");
    expect(replay.document).toBe(first.document);
    await expect(replay.document).rejects.toThrow("offline");
    replay.release();
    await vi.runAllTimersAsync();
    const retry = acquire("paper");
    expect(retry.document).not.toBe(first.document);
    await expect(retry.document).rejects.toThrow("offline");
    retry.release();
    await vi.runAllTimersAsync();
    expect(fetchBlob).toHaveBeenCalledTimes(2);
  });
});

describe("bundled PDF assets", () => {
  it("resolves scanned-image decoders, fonts, and character maps without a CDN", async () => {
    const fetch = vi.fn().mockResolvedValue({ ok: true, arrayBuffer: async () => new ArrayBuffer(4) });
    vi.stubGlobal("fetch", fetch);
    const factory = new BoardPdfBinaryDataFactory();
    for (const filename of ["jbig2.wasm", "openjpeg.wasm", "qcms_bg.wasm"]) {
      expect(await factory.fetch({ kind: "wasmUrl", filename })).toBeInstanceOf(Uint8Array);
    }
    await factory.fetch({ kind: "standardFontDataUrl", filename: "FoxitSerif.pfb" });
    await factory.fetch({ kind: "cMapUrl", filename: "UniJIS-UCS2-H.bcmap" });
    expect(fetch).toHaveBeenCalledTimes(5);
    await expect(factory.fetch({ kind: "wasmUrl", filename: "../../other-file" })).rejects.toThrow("Unsupported PDF resource");
  });
});

describe("board PDF thumbnail", () => {
  let visibility: IntersectionObserverCallback;
  beforeEach(() => {
    vi.stubGlobal("IntersectionObserver", class {
      constructor(callback: IntersectionObserverCallback) { visibility = callback; }
      observe() {}
      disconnect() {}
    });
  });
  const show = () => act(() => visibility([{ isIntersecting: true } as IntersectionObserverEntry], {} as IntersectionObserver));

  function fixture() {
    const cancel = vi.fn();
    const renderPage = vi.fn(() => ({ promise: Promise.resolve(), cancel }));
    const getPage = vi.fn().mockResolvedValue({
      getViewport: ({ scale }: { scale: number }) => ({ width: 612 * scale, height: 792 * scale }),
      render: renderPage,
    });
    const document = { numPages: 5, getPage } as unknown as PDFDocumentProxy;
    const release = vi.fn();
    const acquirePdf = vi.fn(() => ({ document: Promise.resolve(document), release }));
    return { source: { acquirePdf } as unknown as BoardContentSource, acquirePdf, release, cancel, getPage, renderPage };
  }

  it("loads only near the viewport, renders the selected page, and keeps navigation on one resource", async () => {
    const f = fixture();
    const onPageCount = vi.fn();
    const props = { source: f.source, artifactId: "paper", page: 2, title: "Research", onPageCount };
    const view = render(<BoardPdfThumbnail {...props} />);
    expect(f.acquirePdf).not.toHaveBeenCalled();
    show();
    await waitFor(() => expect(screen.getByRole("img", { name: "Research, page 2" })).toBeVisible());
    expect(onPageCount).toHaveBeenCalledWith(5);
    expect(f.getPage).toHaveBeenCalledWith(2);
    view.rerender(<BoardPdfThumbnail {...props} page={3} />);
    await waitFor(() => expect(f.getPage).toHaveBeenCalledWith(3));
    expect(f.acquirePdf).toHaveBeenCalledTimes(1);
    expect(f.cancel).toHaveBeenCalled();
    await waitFor(() => expect(screen.getByRole("img", { name: "Research, page 3" })).toBeVisible());
    view.unmount();
    expect(f.release).toHaveBeenCalledTimes(1);
  });

  it("explains a failed preview without substituting extracted text", async () => {
    const release = vi.fn();
    const source = { acquirePdf: () => ({ document: Promise.reject(new Error("bad pdf")), release }) } as unknown as BoardContentSource;
    const view = render(<BoardPdfThumbnail source={source} artifactId="paper" page={1} title="Paper" onPageCount={vi.fn()} />);
    show();
    expect(await screen.findByText("Preview unavailable")).toBeVisible();
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
    expect(screen.getByText("Your PDF is still linked. Open the source to view it.")).toBeVisible();
    view.unmount();
    expect(release).toHaveBeenCalled();
  });
});
