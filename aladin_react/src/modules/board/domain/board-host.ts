import { createContext, useContext } from "react";

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
  /** Open the artifact — at a page when the object carries a cite (the wormhole). */
  onOpenArtifact?: (artifactId: string, at?: { page?: number }) => void;
  onAskAbout?: (ctx: { artifactId?: string; title: string; text?: string }) => void;
  /** Tool change, object insert, snap, flip — a tap that changed something. */
  haptic?: (kind: BoardHaptic) => void;
}

export const BoardHostContext = createContext<BoardHost>({});

export function useBoardHost(): BoardHost {
  return useContext(BoardHostContext);
}
