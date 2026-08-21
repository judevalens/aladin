import { useEffect, useMemo, useRef, useState } from "react";
import {
  DefaultKeyboardShortcutsDialog,
  DefaultKeyboardShortcutsDialogContent,
  Tldraw,
  type Editor,
  type TLComponents,
  type TLEditorSnapshot,
} from "tldraw";
import "tldraw/tldraw.css";

import type { ApiClient } from "@/shared/api/client";
import type { UserArtifact } from "@/shared/api/models";

type SaveState = "loading" | "saving" | "saved" | "error";

export function BoardPane({
  boardId,
  title,
  client,
}: {
  boardId: string;
  title?: string;
  client: ApiClient;
}) {
  const loadedRef = useRef(false);
  const saveTimerRef = useRef<number | null>(null);
  const cleanupRef = useRef<(() => void) | null>(null);
  const [saveState, setSaveState] = useState<SaveState>("loading");
  const [message, setMessage] = useState("");

  useEffect(() => {
    return () => {
      cleanupRef.current?.();
      if (saveTimerRef.current) window.clearTimeout(saveTimerRef.current);
    };
  }, []);

  const saveSnapshot = useMemo(
    () => async (editor: Editor) => {
      setSaveState("saving");
      try {
        await client.fetch<UserArtifact>(`/api/artifacts/${encodeURIComponent(boardId)}`, {
          method: "PATCH",
          body: JSON.stringify({ content: JSON.stringify(editor.getSnapshot()) }),
        });
        setSaveState("saved");
        setMessage("");
      } catch (error) {
        setSaveState("error");
        setMessage(error instanceof Error ? error.message : "could not save the board");
      }
    },
    [boardId, client],
  );

  function scheduleSave(editor: Editor) {
    if (!loadedRef.current) return;
    if (saveTimerRef.current) window.clearTimeout(saveTimerRef.current);
    saveTimerRef.current = window.setTimeout(() => {
      saveTimerRef.current = null;
      void saveSnapshot(editor);
    }, 700);
  }

  function handleMount(editor: Editor) {
    editor.user.updateUserPreferences({ colorScheme: "dark" });
    setSaveState("loading");

    cleanupRef.current?.();
    let cancelled = false;
    const unlisten = editor.store.listen(() => scheduleSave(editor), {
      scope: "document",
      source: "user",
    });
    cleanupRef.current = () => {
      cancelled = true;
      unlisten();
    };

    void client
      .fetch<UserArtifact>(`/api/artifacts/${encodeURIComponent(boardId)}`)
      .then((record) => {
        if (cancelled) return;
        const snapshot = parseSnapshot(record.content);
        if (snapshot) {
          editor.loadSnapshot(snapshot);
          requestAnimationFrame(() => editor.zoomToFit());
        }
        loadedRef.current = true;
        setSaveState("saved");
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        loadedRef.current = true;
        setSaveState("error");
        setMessage(error instanceof Error ? error.message : "could not load the board");
      });

    return cleanupRef.current;
  }

  const components = useMemo<TLComponents>(
    () => ({
      KeyboardShortcutsDialog: (props) => (
        <DefaultKeyboardShortcutsDialog {...props}>
          <DefaultKeyboardShortcutsDialogContent />
        </DefaultKeyboardShortcutsDialog>
      ),
    }),
    [],
  );

  return (
    <div className="flex h-full w-full flex-col bg-bg">
      <div className="flex h-9 shrink-0 items-center gap-3 border-b border-line bg-panel px-3">
        <div className="min-w-0 flex-1 truncate font-display text-body font-medium text-ink">
          {title || "Board"}
        </div>
        <div className="font-mono text-meta uppercase text-ink-4">{saveStateLabel(saveState)}</div>
      </div>
      {message ? (
        <div className="border-b border-line bg-card px-3 py-2 font-mono text-small text-against">
          {message}
        </div>
      ) : null}
      <div className="min-h-0 flex-1">
        <Tldraw components={components} onMount={handleMount} />
      </div>
    </div>
  );
}

function parseSnapshot(content: string): TLEditorSnapshot | null {
  const trimmed = content.trim();
  if (!trimmed) return null;
  try {
    const snapshot = JSON.parse(trimmed) as unknown;
    return isEditorSnapshot(snapshot) ? snapshot : null;
  } catch {
    return null;
  }
}

function isEditorSnapshot(value: unknown): value is TLEditorSnapshot {
  return typeof value === "object" && value !== null && "document" in value && "session" in value;
}

function saveStateLabel(state: SaveState) {
  switch (state) {
    case "loading":
      return "Loading";
    case "saving":
      return "Saving";
    case "saved":
      return "Saved";
    case "error":
      return "Offline";
  }
}
