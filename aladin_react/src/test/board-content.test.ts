import { describe, expect, it, vi } from "vitest";

import {
  createBoardContentSource,
  flattenBlocks,
  isPdfArtifact,
  sectionForPage,
} from "@/modules/board/domain/board-content";
import type { ApiClient } from "@/shared/api/client";
import type { UserArtifact } from "@/shared/api/models";

describe("flattenBlocks", () => {
  it("collects text across nested content and children", () => {
    const blocks = [
      { type: "paragraph", content: [{ type: "text", text: "Floor comes from the put;" }] },
      {
        type: "bullet",
        content: [{ type: "text", text: "width from both strikes" }],
        children: [{ content: [{ type: "text", text: "premium decides zero-cost." }] }],
      },
    ];
    expect(flattenBlocks(blocks)).toBe(
      "Floor comes from the put; width from both strikes premium decides zero-cost.",
    );
  });

  it("survives junk shapes", () => {
    expect(flattenBlocks(null)).toBe("");
    expect(flattenBlocks([{ weird: true }, 42, "loose"])).toBe("");
  });
});

describe("sectionForPage", () => {
  const meta = {
    pageCount: 361,
    sections: [
      { title: "§2 · legs", pageFrom: 41 },
      { title: "§4.2 · collars", pageFrom: 88 },
      { title: "§6 · greeks", pageFrom: 152 },
    ],
  };

  it("picks the last section at or before the page", () => {
    expect(sectionForPage(meta, 94)).toBe("§4.2 · collars");
    expect(sectionForPage(meta, 41)).toBe("§2 · legs");
    expect(sectionForPage(meta, 300)).toBe("§6 · greeks");
  });

  it("falls back to the bare page before any section", () => {
    expect(sectionForPage(meta, 3)).toBe("p. 3");
  });
});

function clientOf(handler: (path: string) => unknown): ApiClient {
  const fetch = vi.fn((path: string) => Promise.resolve(handler(path)));
  return {
    resolveUrl: (p) => p,
    fetch: fetch as ApiClient["fetch"],
    fetchBlob: () => Promise.reject(new Error("no blobs")),
  };
}

describe("createBoardContentSource", () => {
  it("resolves PDFs without requesting extracted text or waiting for ingestion", async () => {
    const client = clientOf(() => ({ type: "file", title: "Paper", metadata: { mimeType: "application/pdf" } }));
    const source = createBoardContentSource(client);
    const a = vi.fn();
    const b = vi.fn();
    source.subscribe("pdf/1", 1, a, "file");
    source.subscribe("pdf/1", 2, b, "file");
    await vi.waitFor(() => expect(b).toHaveBeenCalled());
    expect(source.get("pdf/1", 1)).toMatchObject({ state: "ready", format: "pdf", sourceLine: "PDF", excerpt: "" });
    expect(a).toHaveBeenCalled();
    expect(client.fetch).toHaveBeenCalledExactlyOnceWith("/api/artifacts/pdf%2F1");
  });

  it("keeps non-PDF files as files", async () => {
    const client = clientOf(() => ({ type: "file", title: "Data", summary: "Price history", metadata: { mimeType: "text/csv" } }));
    const source = createBoardContentSource(client);
    const changed = vi.fn();
    source.subscribe("csv-1", 1, changed, "file");
    await vi.waitFor(() => expect(changed).toHaveBeenCalled());
    expect(source.get("csv-1", 1)).toMatchObject({ state: "ready", format: "file", sourceLine: "File", excerpt: "Price history" });
  });

  it("resolves a document page through meta + pages, and caches within the TTL", async () => {
    const calls: string[] = [];
    const client = clientOf((path) => {
      calls.push(path);
      if (path.endsWith("/document")) {
        return { pageCount: 361, sections: [{ title: "§4.2 · collars", pageFrom: 88 }] };
      }
      return { pages: [{ page: 94, text: "bounded on both sides" }] };
    });
    const source = createBoardContentSource(client);

    const changed = vi.fn();
    source.subscribe("a1", 94, changed);
    await vi.waitFor(() => expect(changed).toHaveBeenCalled());

    const value = source.get("a1", 94);
    expect(value).toEqual({
      state: "ready",
      sourceLine: "§4.2 · collars",
      excerpt: "bounded on both sides",
      pageCount: 361,
    });

    // A second subscriber within the TTL costs zero requests.
    const before = calls.length;
    source.subscribe("a1", 94, vi.fn());
    expect(calls.length).toBe(before);
  });

  it("shares one meta fetch across pages of the same artifact", async () => {
    const calls: string[] = [];
    const client = clientOf((path) => {
      calls.push(path);
      if (path.endsWith("/document")) return { pageCount: 10, sections: [] };
      return { pages: [{ page: 1, text: "x" }] };
    });
    const source = createBoardContentSource(client);

    const a = vi.fn();
    const b = vi.fn();
    source.subscribe("a1", 1, a);
    source.subscribe("a1", 2, b);
    await vi.waitFor(() => {
      expect(a).toHaveBeenCalled();
      expect(b).toHaveBeenCalled();
    });

    expect(calls.filter((p) => p.endsWith("/document")).length).toBe(1);
    expect(calls.filter((p) => p.includes("/document/pages")).length).toBe(2);
  });

  it("lists only window-insertable folder artifacts", async () => {
    const client = clientOf(() => [
      { id: "f1", type: "file", title: "Book", content: "", metadata: { mimeType: "application/pdf" }, createdAt: "", updatedAt: "" },
      { id: "p1", type: "page", title: "Note", content: "", metadata: {}, createdAt: "", updatedAt: "" },
      { id: "b1", type: "board", title: "Board", content: "", metadata: {}, createdAt: "", updatedAt: "" },
      { id: "l1", type: "link", title: "Link", content: "", metadata: {}, createdAt: "", updatedAt: "" },
    ]);
    const source = createBoardContentSource(client);
    const rows = await source.listFolderArtifacts("folder-1");
    expect(rows.map((r) => r.id)).toEqual(["f1", "p1", "l1"]);
    expect(rows[1]).toMatchObject({ kind: "note", meta: "note" });
    expect(rows[0]).toMatchObject({ kind: "file", meta: "PDF" });
  });

  it("resolves an instrument from real artifact metadata, not the notes/PDF endpoints", async () => {
    const client = clientOf(() => ({ type: "app", title: "Spread explorer", summary: "A workspace instrument", content: "do not render source code" }));
    const source = createBoardContentSource(client);
    const changed = vi.fn();
    source.subscribe("app-1", 1, changed, "app");
    await vi.waitFor(() => expect(changed).toHaveBeenCalled());
    expect(source.get("app-1", 1)).toMatchObject({ state: "ready", excerpt: "A workspace instrument", sourceLine: "Aladin instrument" });
    expect(client.fetch).toHaveBeenCalledWith("/api/artifacts/app-1");
    expect(client.fetch).toHaveBeenCalledTimes(1);
  });
});

describe("PDF identification", () => {
  const artifact = { type: "file", title: "My paper", metadata: {} } as UserArtifact;
  it("accepts MIME metadata and original filename fallbacks", () => {
    expect(isPdfArtifact({ ...artifact, metadata: { mimeType: "Application/PDF; charset=binary" } })).toBe(true);
    expect(isPdfArtifact({ ...artifact, metadata: { originalFilename: "paper.PDF" } })).toBe(true);
    expect(isPdfArtifact({ ...artifact, title: "paper.pdf" })).toBe(true);
  });
  it("does not mistake a note or another MIME type for a PDF", () => {
    expect(isPdfArtifact({ ...artifact, type: "note", title: "paper.pdf" })).toBe(false);
    expect(isPdfArtifact({ ...artifact, title: "paper.pdf", metadata: { mimeType: "image/png" } })).toBe(false);
  });
});
