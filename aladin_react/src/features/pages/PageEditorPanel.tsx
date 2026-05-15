import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useCreateBlockNote } from "@blocknote/react";
import { BlockNoteView } from "@blocknote/shadcn";
import { Component, type ReactNode, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { AladinPanel } from "@/components/ui/aladin";
import { api, ApiError } from "@/shared/api/client";
import { exportEditorToMarkdown, loadMarkdownIntoEditor } from "@/features/pages/markdown-adapter";

interface PageEditorPanelProps {
  pageId: string;
}

type SaveState = "idle" | "saving" | "saved" | "conflict" | "error";

class BlockNoteRuntimeBoundary extends Component<
  {
    fallback: ReactNode;
    onError: (error: Error) => void;
    children: ReactNode;
  },
  { hasError: boolean }
> {
  state = { hasError: false };

  static getDerivedStateFromError() {
    return { hasError: true };
  }

  componentDidCatch(error: Error) {
    this.props.onError(error);
  }

  render() {
    if (this.state.hasError) {
      return this.props.fallback;
    }

    return this.props.children;
  }
}

export function PageEditorPanel({ pageId }: PageEditorPanelProps) {
  const queryClient = useQueryClient();
  const pageQuery = useQuery({
    queryKey: ["page", pageId],
    queryFn: () => api.getPage(pageId),
  });
  const [saveState, setSaveState] = useState<SaveState>("idle");
  const [message, setMessage] = useState<string | null>(null);
  const [draftMarkdown, setDraftMarkdown] = useState("");
  const [editorMode, setEditorMode] = useState<"blocknote" | "markdown-fallback">("blocknote");
  const [blockNoteError, setBlockNoteError] = useState<string | null>(null);
  const [editorBoundaryKey, setEditorBoundaryKey] = useState(0);
  const [sessionRevision, setSessionRevision] = useState(0);
  const editor = useCreateBlockNote({}, [pageId, editorBoundaryKey]);
  const saveTimerRef = useRef<number | null>(null);
  const flushSaveRef = useRef<() => void>(() => {});
  const sessionInitializedRef = useRef(false);
  const lastServerSnapshotRef = useRef<{ content: string; revision: number } | null>(null);
  const lastAcknowledgedRevisionRef = useRef(0);
  const lastSavedMarkdownRef = useRef("");
  const draftMarkdownRef = useRef("");
  const dirtyRef = useRef(false);
  const saveInFlightRef = useRef(false);
  const pendingSaveMarkdownRef = useRef<string | null>(null);
  const pendingSaveRevisionRef = useRef<number | null>(null);

  const switchToMarkdownFallback = useCallback((error: unknown) => {
    const nextMessage =
      error instanceof Error ? error.message : "BlockNote failed to initialize for this page.";
    setEditorMode("markdown-fallback");
    setBlockNoteError(nextMessage);
  }, []);

  const initializeEditorSession = useCallback(
    async (snapshot: { content: string; revision: number }) => {
      if (saveTimerRef.current !== null) {
        window.clearTimeout(saveTimerRef.current);
        saveTimerRef.current = null;
      }
      draftMarkdownRef.current = snapshot.content;
      setDraftMarkdown(snapshot.content);
      if (editorMode === "blocknote") {
        try {
          await loadMarkdownIntoEditor(editor, snapshot.content);
        } catch (error) {
          switchToMarkdownFallback(error);
        }
      }
      sessionInitializedRef.current = true;
      lastServerSnapshotRef.current = snapshot;
      lastAcknowledgedRevisionRef.current = snapshot.revision;
      lastSavedMarkdownRef.current = snapshot.content;
      dirtyRef.current = false;
      setSessionRevision(snapshot.revision);
      setSaveState("idle");
      setMessage(null);
    },
    [editor, editorMode, switchToMarkdownFallback],
  );

  useEffect(() => {
    if (!pageQuery.data) return;
    const snapshot = { content: pageQuery.data.content, revision: pageQuery.data.revision };
    lastServerSnapshotRef.current = snapshot;

    if (!sessionInitializedRef.current) {
      void initializeEditorSession(snapshot);
    }
  }, [pageQuery.data?.id, pageQuery.data?.revision, pageQuery.data?.content, initializeEditorSession]);

  useEffect(() => {
    setEditorMode("blocknote");
    setBlockNoteError(null);
    setMessage(null);
    setSaveState("idle");
    setDraftMarkdown("");
    setSessionRevision(0);
    sessionInitializedRef.current = false;
    lastServerSnapshotRef.current = null;
    draftMarkdownRef.current = "";
    lastSavedMarkdownRef.current = "";
    lastAcknowledgedRevisionRef.current = 0;
    dirtyRef.current = false;
    saveInFlightRef.current = false;
    pendingSaveMarkdownRef.current = null;
    pendingSaveRevisionRef.current = null;
  }, [pageId]);

  useEffect(() => {
    if (!sessionInitializedRef.current || editorMode !== "blocknote") return;
    void loadMarkdownIntoEditor(editor, draftMarkdownRef.current).catch((error) => {
      switchToMarkdownFallback(error);
    });
  }, [editor, editorMode, switchToMarkdownFallback]);

  const computeCurrentMarkdown = useCallback(() => {
    return editorMode === "markdown-fallback" ? draftMarkdownRef.current : exportEditorToMarkdown(editor);
  }, [editor, editorMode]);

  const scheduleSave = useCallback((delay = 900) => {
    if (saveTimerRef.current !== null) {
      window.clearTimeout(saveTimerRef.current);
    }
    saveTimerRef.current = window.setTimeout(() => {
      flushSaveRef.current();
    }, delay);
  }, []);

  const saveMutation = useMutation({
    mutationFn: async ({ markdown, revision }: { markdown: string; revision: number }) =>
      api.savePage(pageId, {
        content: markdown,
        revision,
      }),
    onSuccess: (record) => {
      queryClient.setQueryData(["page", pageId], record);
      const snapshot = {
        content: record.content,
        revision: record.revision,
      };
      lastServerSnapshotRef.current = snapshot;
      lastAcknowledgedRevisionRef.current = record.revision;
      lastSavedMarkdownRef.current = record.content;
      draftMarkdownRef.current = record.content;
      setSessionRevision(record.revision);
      saveInFlightRef.current = false;
      pendingSaveRevisionRef.current = null;
      pendingSaveMarkdownRef.current = null;

      let currentMarkdown = record.content;
      try {
        currentMarkdown = computeCurrentMarkdown();
      } catch {
        currentMarkdown = record.content;
      }

      if (currentMarkdown === record.content) {
        dirtyRef.current = false;
        setSaveState("saved");
        setMessage("Saved");
        return;
      }

      dirtyRef.current = true;
      setSaveState("idle");
      setMessage("Draft");
      scheduleSave(250);
    },
    onError: (error) => {
      saveInFlightRef.current = false;
      if (error instanceof ApiError && error.status === 409) {
        const latestSnapshot = lastServerSnapshotRef.current;
        if (
          latestSnapshot &&
          pendingSaveMarkdownRef.current !== null &&
          pendingSaveRevisionRef.current !== null &&
          latestSnapshot.revision >= pendingSaveRevisionRef.current &&
          latestSnapshot.content === pendingSaveMarkdownRef.current
        ) {
          lastAcknowledgedRevisionRef.current = latestSnapshot.revision;
          lastSavedMarkdownRef.current = latestSnapshot.content;
          draftMarkdownRef.current = latestSnapshot.content;
          dirtyRef.current = false;
          pendingSaveRevisionRef.current = null;
          pendingSaveMarkdownRef.current = null;
          setSessionRevision(latestSnapshot.revision);
          setSaveState("saved");
          setMessage("Saved");
          return;
        }
        setSaveState("conflict");
        setMessage("Save conflict. This draft was not saved.");
        return;
      }
      setSaveState("error");
      setMessage(error instanceof Error ? error.message : "Failed to save page.");
    },
    onSettled: () => {
      pendingSaveRevisionRef.current = null;
      pendingSaveMarkdownRef.current = null;
    },
  });

  const flushSave = useCallback(() => {
    if (saveTimerRef.current !== null) {
      window.clearTimeout(saveTimerRef.current);
      saveTimerRef.current = null;
    }
    if (saveInFlightRef.current || saveMutation.isPending || !dirtyRef.current) return;
    const markdown = computeCurrentMarkdown();
    if (markdown === lastSavedMarkdownRef.current) {
      dirtyRef.current = false;
      return;
    }
    const nextRevision = lastAcknowledgedRevisionRef.current + 1;
    saveInFlightRef.current = true;
    pendingSaveMarkdownRef.current = markdown;
    pendingSaveRevisionRef.current = nextRevision;
    setSaveState("saving");
    setMessage("Saving…");
    saveMutation.mutate({ markdown, revision: nextRevision });
  }, [computeCurrentMarkdown, saveMutation]);

  flushSaveRef.current = flushSave;

  useEffect(() => {
    return () => {
      if (saveTimerRef.current !== null) {
        window.clearTimeout(saveTimerRef.current);
      }
      flushSaveRef.current();
    };
  }, []);

  const statusTone = useMemo(() => {
    switch (saveState) {
      case "conflict":
      case "error":
        return "border-red-300 bg-red-50 text-red-700";
      default:
        return "border-gray-300 bg-white text-gray-700";
    }
  }, [saveState]);

  const markdownFallback = (
    <div className="flex min-h-[520px] flex-1 flex-col">
      <div className="flex items-center justify-between gap-4 border-b border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
        <div className="space-y-1">
          <div className="font-medium">The rich editor is unavailable for this page.</div>
          <div className="text-xs leading-5 text-amber-700">
            {blockNoteError ?? "You can keep working in raw markdown for now."}
          </div>
        </div>
        <Button
          variant="secondary"
          size="sm"
          onClick={() => {
            setEditorMode("blocknote");
            setBlockNoteError(null);
            setEditorBoundaryKey((current) => current + 1);
          }}
        >
          Retry editor
        </Button>
      </div>
      <textarea
        className="min-h-0 flex-1 resize-none border-0 bg-transparent px-5 py-4 font-mono text-sm leading-7 text-black outline-none"
        value={draftMarkdown}
        onChange={(event) => {
          draftMarkdownRef.current = event.target.value;
          setDraftMarkdown(event.target.value);
          dirtyRef.current = true;
          setSaveState("idle");
          setMessage("Draft");
          scheduleSave();
        }}
      />
    </div>
  );

  if (pageQuery.isLoading) {
    return <div className="p-6 text-sm text-gray-700">Loading page…</div>;
  }

  if (pageQuery.error) {
    return (
      <AladinPanel className="m-6 p-4 text-sm text-red-700">
        {pageQuery.error instanceof Error ? pageQuery.error.message : "Failed to load page."}
      </AladinPanel>
    );
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex items-center justify-between border-b border-gray-300 px-6 py-4">
        <div className="space-y-1">
          <div className="text-xs text-gray-500">
            Page
          </div>
          <h2 className="text-2xl font-semibold text-black">
            {pageQuery.data?.title ?? "Untitled page"}
          </h2>
        </div>
        <div className={`rounded-md border px-3 py-2 text-xs font-medium ${statusTone}`}>
          {message ?? `Revision ${sessionRevision}`}
        </div>
      </div>

      <div className="min-h-0 flex-1 px-6 pb-6 pt-5">
        <div className="mx-auto flex h-full w-full max-w-workspace-max flex-col">
          <AladinPanel className="flex min-h-0 flex-1 flex-col overflow-hidden" onBlurCapture={flushSave}>
            {editorMode === "markdown-fallback" ? (
              markdownFallback
            ) : (
              <div className="relative flex min-h-0 flex-1 flex-col">
                <div className="editor-scroll min-h-0 flex-1 overflow-y-auto overscroll-contain px-5 py-4">
                  <BlockNoteRuntimeBoundary
                    key={`${pageId}:${editorBoundaryKey}`}
                    fallback={markdownFallback}
                    onError={(error) => {
                      switchToMarkdownFallback(error);
                    }}
                  >
                    <BlockNoteView
                      editor={editor}
                      editable
                      theme="light"
                      className="min-h-full"
                      onChange={() => {
                        dirtyRef.current = true;
                        setSaveState((current) => (current === "idle" ? current : "idle"));
                        setMessage((current) => (current === "Draft" ? current : "Draft"));
                        scheduleSave();
                      }}
                    />
                  </BlockNoteRuntimeBoundary>
                </div>
              </div>
            )}
          </AladinPanel>
        </div>
      </div>
    </div>
  );
}
