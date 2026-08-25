import { useMemo } from "react";

import { useAppComposition } from "@/app/composition/app-composition";
import { useAppStore } from "@/app/state/store";
import { useArtifact } from "@/modules/artifacts/hooks/use-artifact";
import { BoardPane } from "./board-pane";
import type { BoardHost } from "../domain/board-host";

/**
 * The desktop mount — the ONLY board file allowed to import the app store/composition.
 * The shared pane stays host-agnostic; this wrapper wires its two capabilities into the
 * desktop shell: "Open in folder" opens the artifact's tab, "Ask about this" queues the
 * question on the copilot dock (which auto-sends when idle, with the board already
 * reported as the current surface).
 */
export function DesktopBoardPane({ boardId, title }: { boardId: string; title?: string }) {
  const { runtime } = useAppComposition();
  // The replica row re-emits when a sync frame lands for this artifact; its updatedAt is
  // the pane's revision signal — a board saved on the iPad refreshes here on the frame.
  const artifact = useArtifact(boardId);

  const host = useMemo<BoardHost>(
    () => ({
      onOpenArtifact: (artifactId) => useAppStore.getState().openArtifact(artifactId),
      onAskAbout: ({ title: objectTitle, text }) =>
        useAppStore.getState().queueCopilotText(text ? `${objectTitle} — ${text}` : objectTitle),
    }),
    [],
  );

  return (
    <BoardPane
      boardId={boardId}
      title={title}
      client={runtime.apiClient}
      host={host}
      revision={artifact?.updatedLabel ?? null}
    />
  );
}
