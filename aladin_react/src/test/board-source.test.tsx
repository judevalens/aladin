import type { ReactNode } from "react";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Editor } from "tldraw";
import { BoardHostContext, type BoardHost } from "@/modules/board/domain/board-host";
import { boardSourceUrl, useOpenBoardSource } from "@/modules/board/domain/board-source";
import { BoardToastContext, createToastStore } from "@/modules/board/domain/board-toasts";
import { LinkShapeUtil } from "@/modules/board/shapes/link-shape";
import { LINK_DEFAULTS, type LinkShape } from "@/modules/board/shapes/shape-types";

vi.mock("tldraw", async (importOriginal) => ({
  ...await importOriginal<typeof import("tldraw")>(),
  HTMLContainer: ({ children }: { children: ReactNode }) => <div>{children}</div>,
}));
afterEach(() => { cleanup(); vi.restoreAllMocks(); });
const source = "https://www.youtube.com/watch?v=jNQXAC9IVRw&t=24#source";

function OpenAction({ url = source }: { url?: string }) {
  const open = useOpenBoardSource();
  return <button onClick={() => open(url)}>Open source</button>;
}

describe("board source actions", () => {
  it("opens native sources once without trying to create a webview popup", async () => {
    const open = vi.fn().mockResolvedValue(undefined);
    const popup = vi.spyOn(window, "open").mockReturnValue(null);
    render(<BoardHostContext.Provider value={{ onOpenExternalUrl: open }}><OpenAction /></BoardHostContext.Provider>);
    fireEvent.click(screen.getByRole("button", { name: "Open source" }));
    expect(open).toHaveBeenCalledExactlyOnceWith(source);
    expect(popup).not.toHaveBeenCalled();
  });

  it("opens the browser fallback synchronously and does not replace the board", () => {
    const popup = vi.spyOn(window, "open").mockReturnValue(null);
    render(<OpenAction />);
    fireEvent.click(screen.getByRole("button", { name: "Open source" }));
    expect(popup).toHaveBeenCalledExactlyOnceWith(source, "_blank", "noopener,noreferrer");
  });

  it("reports rejected host opens instead of failing silently", async () => {
    const toasts = createToastStore();
    const host: BoardHost = { onOpenExternalUrl: vi.fn().mockRejectedValue(new Error("old native shell")) };
    render(<BoardToastContext.Provider value={toasts}><BoardHostContext.Provider value={host}><OpenAction /></BoardHostContext.Provider></BoardToastContext.Provider>);
    fireEvent.click(screen.getByRole("button", { name: "Open source" }));
    await waitFor(() => expect(toasts.get()?.text).toContain("Couldn't open this source"));
    toasts.dismiss();
  });

  it("uses the same native path for the link-card icon and marks the pointer handled", () => {
    const markEventAsHandled = vi.fn();
    const editor = { markEventAsHandled } as unknown as Editor;
    const shape = { id: "shape:link", type: "aladin-link", meta: {}, props: { ...LINK_DEFAULTS, url: source, title: "Video", status: "ready" } } as LinkShape;
    const util = new LinkShapeUtil(editor);
    function Card() { return util.component(shape); }
    const open = vi.fn();
    render(<BoardHostContext.Provider value={{ onOpenExternalUrl: open }}><Card /></BoardHostContext.Provider>);
    const button = screen.getByRole("button", { name: "Open link" });
    fireEvent.pointerDown(button);
    fireEvent.click(button);
    expect(markEventAsHandled).toHaveBeenCalledTimes(1);
    expect(open).toHaveBeenCalledExactlyOnceWith(source);
  });

  it.each(["javascript:alert(1)", "file:///etc/passwd", "data:text/html,test", "https://user:pass@example.com", "not a url"])("refuses unsafe or malformed URL %s", (url) => {
    expect(boardSourceUrl(url)).toBeNull();
  });
});
