import { createContext, useContext } from "react";

/**
 * The pane's store status, read by the chrome's status pill. `local` boards (the spike)
 * have nothing to report; a synced board surfaces the connection — including the terminal
 * sync errors, which tldraw never retries on its own (`retry` remounts and redials).
 */
export interface BoardStatus {
  mode: "synced" | "local";
  state: "loading" | "online" | "offline" | "error";
  /** Human-readable cause, present when state is "error". */
  reason?: string;
  retry: () => void;
}

export const BoardStatusContext = createContext<BoardStatus>({
  mode: "local",
  state: "online",
  retry: () => {},
});

export function useBoardStatus(): BoardStatus {
  return useContext(BoardStatusContext);
}
