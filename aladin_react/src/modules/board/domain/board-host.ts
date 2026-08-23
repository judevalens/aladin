import { createContext, useContext } from "react";

/**
 * What the host mounting the board can do for it. An absent callback hides the affordance
 * — that is the whole capability negotiation:
 *
 * - iPad embed: `onOpenArtifact` posts over the native bridge; no `onAskAbout` (no copilot
 *   on the companion yet).
 * - Desktop: both wired into the workspace/copilot slices by the desktop wrapper.
 * - Spike: neither — the buttons simply do not render.
 */
export interface BoardHost {
  onOpenArtifact?: (artifactId: string) => void;
  onAskAbout?: (ctx: { artifactId?: string; title: string; text?: string }) => void;
}

export const BoardHostContext = createContext<BoardHost>({});

export function useBoardHost(): BoardHost {
  return useContext(BoardHostContext);
}
