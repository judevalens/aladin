import { describe, expect, it, vi } from "vitest";
import type { Editor, TLShapeId } from "tldraw";
import { resolveLinkInto } from "@/modules/board/domain/board-link-flow";
import type { BoardContentSource } from "@/modules/board/domain/board-content";
import type { UnfurlResult } from "@/modules/board/domain/board-links";
import type { LinkShape } from "@/modules/board/shapes/shape-types";

const url = "https://youtu.be/jNQXAC9IVRw?t=2";
const result: UnfurlResult = { url, domain: "youtu.be", title: "A video", description: "By channel", siteName: "YouTube", imageUrl: "https://i.ytimg.com/preview.jpg", faviconUrl: "" };
function fixture() {
  const shape = { id: "shape:link" as TLShapeId, type: "aladin-link", props: { url, h: 410, title: "Old title" } } as LinkShape;
  const updateShape = vi.fn();
  const editor = { isDisposed: false, getShape: vi.fn(() => shape), updateShape } as unknown as Editor;
  return { shape, updateShape, editor };
}

describe("refreshing board link previews", () => {
  it("refreshes a saved card without shrinking its user-sized bounds", async () => {
    const f = fixture();
    const source = { unfurl: vi.fn().mockResolvedValue(result) } as unknown as BoardContentSource;
    resolveLinkInto(f.editor, source, f.shape.id, url);
    expect(f.updateShape).toHaveBeenCalledWith(expect.objectContaining({ props: { status: "pending" } }));
    await vi.waitFor(() => expect(f.updateShape).toHaveBeenLastCalledWith(expect.objectContaining({ props: expect.objectContaining({ title: "A video", h: 410, status: "ready" }) })));
  });

  it("ignores stale responses after another refresh or a URL edit", async () => {
    const f = fixture();
    let finishFirst!: (value: UnfurlResult) => void;
    let finishSecond!: (value: UnfurlResult) => void;
    const source = { unfurl: vi.fn()
      .mockImplementationOnce(() => new Promise((resolve) => { finishFirst = resolve; }))
      .mockImplementationOnce(() => new Promise((resolve) => { finishSecond = resolve; })) } as unknown as BoardContentSource;
    resolveLinkInto(f.editor, source, f.shape.id, url);
    resolveLinkInto(f.editor, source, f.shape.id, url);
    finishFirst({ ...result, title: "Stale" });
    await Promise.resolve();
    expect(f.updateShape).toHaveBeenCalledTimes(2);
    f.shape.props.url = "https://example.com/changed";
    finishSecond(result);
    await Promise.resolve();
    expect(f.updateShape).toHaveBeenCalledTimes(2);
  });

  it("keeps the last metadata if a refresh fails", async () => {
    const f = fixture();
    resolveLinkInto(f.editor, { unfurl: vi.fn().mockRejectedValue(new Error("blocked")) } as unknown as BoardContentSource, f.shape.id, url);
    await vi.waitFor(() => expect(f.updateShape).toHaveBeenLastCalledWith(expect.objectContaining({ props: { status: "failed", domain: "youtu.be" } })));
  });
});
