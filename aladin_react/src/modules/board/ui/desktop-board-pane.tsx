import { useMemo } from "react";

import { useAppComposition } from "@/app/composition/app-composition";
import { useAppStore } from "@/app/state/store";
import { BoardPane, type BoardSyncConfig } from "./board-pane";
import type { BoardHost } from "../domain/board-host";

/**
 * The desktop mount — the ONLY board file allowed to import the app store/composition.
 * The shared pane stays host-agnostic; this wrapper wires its capabilities into the
 * desktop shell: "Open in folder" opens the artifact's tab, "Ask about this" queues the
 * question on the copilot dock, and the board room server comes from the runtime config
 * with the desktop session's bearer per (re)connect.
 */
export function DesktopBoardPane({ boardId, title }: { boardId: string; title?: string }) {
  const { runtime } = useAppComposition();

  const host = useMemo<BoardHost>(
    () => ({
      onOpenArtifact: (artifactId) => useAppStore.getState().openArtifact(artifactId),
      onAskAbout: ({ title: objectTitle, text }) =>
        useAppStore.getState().queueCopilotText(text ? `${objectTitle} — ${text}` : objectTitle),
    }),
    [],
  );

  const sync = useMemo<BoardSyncConfig | null>(() => {
    const url = runtime.config.boardSyncWsUrl;
    if (!url) return null;
    return { url, getToken: () => runtime.desktopSession.getToken() ?? "" };
  }, [runtime]);

  return (
    <BoardPane boardId={boardId} title={title} client={runtime.apiClient} host={host} sync={sync} />
  );
}
