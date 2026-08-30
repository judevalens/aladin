import { render } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { BoardPane } from "@/modules/board/ui/board-pane";
import { DesktopBoardPane } from "@/modules/board/ui/desktop-board-pane";

vi.mock("@/app/composition/app-composition", () => ({
  useAppComposition: () => ({ runtime }),
}));
vi.mock("@/modules/board/ui/board-pane", () => ({
  BoardPane: vi.fn(() => <div data-testid="board-canvas" />),
}));

const runtime = {
  config: { boardSyncWsUrl: "" },
  apiClient: {},
  desktopSession: { getToken: () => "test-session" },
};

describe("DesktopBoardPane", () => {
  it.each(["", "ws://board.example.test"])(
    "omits the duplicate board header with sync URL %j",
    (boardSyncWsUrl) => {
      runtime.config.boardSyncWsUrl = boardSyncWsUrl;
      render(<DesktopBoardPane boardId="research-board" title="Research" />);

      expect(vi.mocked(BoardPane).mock.calls.at(-1)?.[0]).toMatchObject({
        boardId: "research-board",
        chrome: "plane",
        sync: boardSyncWsUrl ? { url: boardSyncWsUrl } : null,
      });
    },
  );
});
