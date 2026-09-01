import { useEffect, type ReactNode } from "react";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Editor } from "tldraw";

import { BoardContentContext, type BoardContentSource, type DocPageContent } from "@/modules/board/domain/board-content";
import { DocWindowShapeUtil } from "@/modules/board/shapes/doc-window-shape";
import { DOC_WINDOW_DEFAULTS, type DocWindowShape } from "@/modules/board/shapes/shape-types";

vi.mock("tldraw", async (importOriginal) => ({
  ...await importOriginal<typeof import("tldraw")>(),
  HTMLContainer: ({ children }: { children: ReactNode }) => <div>{children}</div>,
}));
vi.mock("@/modules/board/ui/board-pdf-thumbnail", () => ({
  BoardPdfThumbnail: ({ page, onPageCount }: { page: number; onPageCount(count: number): void }) => {
    useEffect(() => onPageCount(8), []);
    return <div role="img" aria-label={`PDF page ${page}`} />;
  },
}));
afterEach(cleanup);

function fixture(content: DocPageContent, overrides: Partial<DocWindowShape["props"]> = {}) {
  const updateShape = vi.fn();
  const editor = { updateShape, markEventAsHandled: vi.fn() } as unknown as Editor;
  const util = new DocWindowShapeUtil(editor);
  const shape = { id: "shape:pdf", type: "aladin-doc", meta: {}, props: { ...DOC_WINDOW_DEFAULTS, artifactId: "paper", title: "Research paper", ...overrides } } as DocWindowShape;
  const source = { get: () => content, subscribe: () => () => {} } as unknown as BoardContentSource;
  function View() { return util.component(shape); }
  render(<BoardContentContext.Provider value={source}><View /></BoardContentContext.Provider>);
  return { updateShape };
}

describe("PDF board card", () => {
  it("shows PDF and a page image instead of extracted text, retaining page controls", async () => {
    const { updateShape } = fixture({ state: "ready", format: "pdf", sourceLine: "Section", excerpt: "Extracted text must not appear", pageCount: 1 });
    expect(screen.getByRole("article", { name: "PDF: Research paper" })).toBeVisible();
    expect(screen.queryByText("Document")).not.toBeInTheDocument();
    expect(screen.queryByText("Extracted text must not appear")).not.toBeInTheDocument();
    expect(screen.getByRole("img", { name: "PDF page 1" })).toBeVisible();
    expect(screen.getByRole("button", { name: "Previous page" })).toBeDisabled();
    await waitFor(() => expect(screen.getByText("Page 1 / 8")).toBeVisible());
    fireEvent.click(screen.getByRole("button", { name: "Next page" }));
    expect(updateShape).toHaveBeenLastCalledWith({ id: "shape:pdf", type: "aladin-doc", props: { page: 2, pageCount: 8 } });
  });

  it("clamps a stale saved page against the real PDF count", async () => {
    const { updateShape } = fixture({ state: "ready", format: "pdf", sourceLine: "PDF", excerpt: "", pageCount: 1 }, { page: 20, pageCount: 30 });
    await waitFor(() => expect(updateShape).toHaveBeenCalledWith({ id: "shape:pdf", type: "aladin-doc", props: { page: 8, pageCount: 8 } }));
  });

  it("leaves workspace notes as text cards", () => {
    fixture({ state: "ready", sourceLine: "Workspace note", excerpt: "My research notes", pageCount: 1 }, { artifactKind: "note" });
    expect(screen.getByText("My research notes")).toBeVisible();
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
  });
});
