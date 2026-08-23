import { createContext, useContext } from "react";

import type { BoardSaveState } from "./board-persistence";

/** Load: whether the board's content has arrived. Saving is armed only once it has. */
export type BoardLoadState = "loading" | "ready" | "failed";

/**
 * The pane's persistence status, read by the chrome's status pill. Provided by BoardPane;
 * the chrome never talks to the saver directly.
 */
export interface BoardStatus {
  load: BoardLoadState;
  save: BoardSaveState;
  /** The last error's message, empty when there is none. */
  message: string;
  retryLoad: () => void;
}

export const BoardStatusContext = createContext<BoardStatus>({
  load: "ready",
  save: "saved",
  message: "",
  retryLoad: () => {},
});

export function useBoardStatus(): BoardStatus {
  return useContext(BoardStatusContext);
}
