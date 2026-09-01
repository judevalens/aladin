import { createContext, useContext } from "react";

/** One excerpt captured from a reader, waiting for a board to place it. */
export interface CapturedExcerpt {
  text: string;
  sourceArtifactId: string;
  sourceTitle: string;
  page: number;
}

/** The three haptic weights the board asks for. Never fired per stroke. */
export type BoardHaptic = "light" | "medium" | "select";

/**
 * What the host mounting the board can do for it. An absent callback hides the affordance
 * — that is the whole capability negotiation:
 *
 * - iPad embed: `onOpenArtifact` and `haptic` post over the native bridge; no `onAskAbout`
 *   until the companion's copilot exists.
 * - Desktop: open/ask wired into the workspace/copilot slices by the desktop wrapper; no
 *   haptics (a trackpad has none the page can reach).
 * - Spike: none — the buttons simply do not render.
 */
export interface BoardHost {
  /** Open a web source outside the board (native shells must handle new windows). */
  onOpenExternalUrl?: (url: string) => void | Promise<void>;
  /** Open the artifact — at a page when the object carries a cite (the wormhole). */
  onOpenArtifact?: (artifactId: string, at?: { page?: number }) => void;
  onAskAbout?: (ctx: { artifactId?: string; title: string; text?: string }) => void;
  /** Tool change, object insert, snap, flip — a tap that changed something. */
  haptic?: (kind: BoardHaptic) => void;
  /**
   * The capture inbox (desktop): the ACTIVE board drains excerpts pulled while reading.
   * `take` returns-and-clears; `subscribe` fires on any inbox change.
   */
  captures?: {
    take: () => CapturedExcerpt[];
    subscribe: (onChange: () => void) => () => void;
  };
}

export const BoardHostContext = createContext<BoardHost>({});

export function useBoardHost(): BoardHost {
  return useContext(BoardHostContext);
}
