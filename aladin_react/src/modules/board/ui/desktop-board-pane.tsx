import { useMemo } from "react";
import { invoke } from "@tauri-apps/api/core";

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
  // Only the frontmost board catches reader excerpts — two mounted boards must not both
  // drain the inbox. Also feeds focus (background boards drop key handling).
  const isActiveTab = useAppStore((s) => s.workspace.activeTabKey === boardId);

  const captures = useMemo<NonNullable<BoardHost["captures"]>>(
    () => ({
      take: () => useAppStore.getState().takeQueuedExcerpts(),
      subscribe: (onChange) => useAppStore.subscribe(onChange),
    }),
    [],
  );

  const host = useMemo<BoardHost>(
    () => ({
      onOpenExternalUrl: runtime.config.isDesktopApp
        ? (url) => invoke<void>("open_external_url", { url })
        : undefined,
      onOpenArtifact: (artifactId, at) =>
        at?.page != null
          ? useAppStore.getState().openArtifactAt(artifactId, at.page)
          : useAppStore.getState().openArtifact(artifactId),
      onAskAbout: ({ title: objectTitle, text }) =>
        useAppStore.getState().queueCopilotText(text ? `${objectTitle} — ${text}` : objectTitle),
      captures,
    }),
    [captures, runtime.config.isDesktopApp],
  );

  const sync = useMemo<BoardSyncConfig | null>(() => {
    const url = runtime.config.boardSyncWsUrl;
    if (!url) return null;
    return { url, getToken: () => runtime.desktopSession.getToken() ?? "" };
  }, [runtime]);

  return (
    <BoardPane
      boardId={boardId}
      title={title}
      chrome="plane"
      client={runtime.apiClient}
      host={host}
      sync={sync}
      active={isActiveTab}
    />
  );
}
