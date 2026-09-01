import { render } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { BoardPane } from "@/modules/board/ui/board-pane";
import { DesktopBoardPane } from "@/modules/board/ui/desktop-board-pane";
import { invoke } from "@tauri-apps/api/core";

vi.mock("@tauri-apps/api/core", () => ({ invoke: vi.fn().mockResolvedValue(undefined) }));

vi.mock("@/app/composition/app-composition", () => ({
  useAppComposition: () => ({ runtime }),
}));
vi.mock("@/modules/board/ui/board-pane", () => ({
  BoardPane: vi.fn(() => <div data-testid="board-canvas" />),
}));

const runtime = {
  config: { boardSyncWsUrl: "", isDesktopApp: false },
  apiClient: {},
  desktopSession: { getToken: () => "test-session" },
};

describe("DesktopBoardPane", () => {
  beforeEach(() => { runtime.config.isDesktopApp = false; });
  it("routes native source actions through the desktop command", async () => {
    runtime.config.isDesktopApp = true;
    render(<DesktopBoardPane boardId="research-board" />);
    const host = vi.mocked(BoardPane).mock.calls.at(-1)?.[0].host;
    await host?.onOpenExternalUrl?.("https://youtube.com/watch?v=jNQXAC9IVRw&t=2");
    expect(invoke).toHaveBeenCalledWith("open_external_url", { url: "https://youtube.com/watch?v=jNQXAC9IVRw&t=2" });
    runtime.config.isDesktopApp = false;
  });

  it.each(["", "ws://board.example.test"])(
    "omits the duplicate board header with sync URL %j",
    (boardSyncWsUrl) => {
      runtime.config.boardSyncWsUrl = boardSyncWsUrl;
      render(<DesktopBoardPane boardId="research-board" title="Research" />);

      expect(vi.mocked(BoardPane).mock.calls.at(-1)?.[0]).toMatchObject({
        boardId: "research-board",
        chrome: "plane",
        sync: boardSyncWsUrl ? { url: boardSyncWsUrl } : null,
        host: { onOpenExternalUrl: undefined },
      });
    },
  );
});
