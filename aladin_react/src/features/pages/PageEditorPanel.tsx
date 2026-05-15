import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useCreateBlockNote } from "@blocknote/react";
import { BlockNoteView } from "@blocknote/shadcn";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { AladinPanel } from "@/components/ui/aladin";
import { api, ApiError } from "@/shared/api/client";
import { exportEditorToMarkdown, loadMarkdownIntoEditor } from "@/features/pages/markdown-adapter";

interface PageEditorPanelProps {
  pageId: string;
}

type SaveState = "idle" | "saving" | "saved" | "conflict" | "error";

export function PageEditorPanel({ pageId }: PageEditorPanelProps) {
  const queryClient = useQueryClient();
  const pageQuery = useQuery({
    queryKey: ["page", pageId],
    queryFn: () => api.getPage(pageId),
  });
  const editor = useCreateBlockNote();
  const [saveState, setSaveState] = useState<SaveState>("idle");
  const [message, setMessage] = useState<string | null>(null);
  const saveTimerRef = useRef<number | null>(null);
  const lastLoadedRevisionRef = useRef<number>(0);
  const lastSavedMarkdownRef = useRef("");
  const dirtyRef = useRef(false);

  const hydrateFromServer = useCallback(
    async (markdown: string, revision: number) => {
      await loadMarkdownIntoEditor(editor, markdown);
      lastLoadedRevisionRef.current = revision;
      lastSavedMarkdownRef.current = markdown;
      dirtyRef.current = false;
      setSaveState("idle");
      setMessage(null);
    },
    [editor],
  );

  useEffect(() => {
    if (!pageQuery.data) return;
    void hydrateFromServer(pageQuery.data.content, pageQuery.data.revision);
  }, [pageQuery.data?.id, pageQuery.data?.revision, hydrateFromServer]);

  const saveMutation = useMutation({
    mutationFn: async (markdown: string) =>
      api.savePage(pageId, {
        content: markdown,
        revision: lastLoadedRevisionRef.current,
      }),
    onSuccess: async (record) => {
      queryClient.setQueryData(["page", pageId], record);
      lastLoadedRevisionRef.current = record.revision;
      lastSavedMarkdownRef.current = record.content;
      dirtyRef.current = false;
      setSaveState("saved");
      setMessage("Saved");
    },
    onError: (error) => {
      if (error instanceof ApiError && error.status === 409) {
        setSaveState("conflict");
        setMessage("This page changed elsewhere. Reload before saving again.");
        return;
      }
      setSaveState("error");
      setMessage(error instanceof Error ? error.message : "Failed to save page.");
    },
  });

  const flushSave = useCallback(() => {
    if (saveMutation.isPending || !dirtyRef.current) return;
    const markdown = exportEditorToMarkdown(editor);
    if (markdown === lastSavedMarkdownRef.current) {
      dirtyRef.current = false;
      return;
    }
    setSaveState("saving");
    setMessage("Saving…");
    saveMutation.mutate(markdown);
  }, [editor, saveMutation]);

  const scheduleSave = useCallback(() => {
    if (saveTimerRef.current !== null) {
      window.clearTimeout(saveTimerRef.current);
    }
    saveTimerRef.current = window.setTimeout(() => {
      flushSave();
    }, 900);
  }, [flushSave]);

  useEffect(() => {
    return () => {
      if (saveTimerRef.current !== null) {
        window.clearTimeout(saveTimerRef.current);
      }
      flushSave();
    };
  }, [flushSave]);

  const statusTone = useMemo(() => {
    switch (saveState) {
      case "conflict":
      case "error":
        return "border-red-300 bg-red-50 text-red-700";
      default:
        return "border-aladin-border bg-aladin-panel-muted text-aladin-ink-secondary";
    }
  }, [saveState]);

  if (pageQuery.isLoading) {
    return <div className="p-6 text-sm text-aladin-ink-secondary">Loading page…</div>;
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
      <div className="flex items-center justify-between border-b border-aladin-divider px-6 py-4">
        <div className="space-y-1">
          <div className="text-xs font-semibold uppercase tracking-[0.2em] text-aladin-code-text">
            Markdown-backed BlockNote
          </div>
          <h2 className="text-3xl font-semibold tracking-[-0.04em] text-aladin-ink">
            {pageQuery.data?.title ?? "Untitled page"}
          </h2>
        </div>
        <div className={`rounded-control border px-3 py-2 text-xs font-medium ${statusTone}`}>
          {message ?? `Revision ${pageQuery.data?.revision ?? 0}`}
        </div>
      </div>

      {saveState === "conflict" ? (
        <div className="flex items-center justify-between gap-4 border-b border-red-200 bg-red-50 px-6 py-3 text-sm text-red-700">
          <span>{message}</span>
          <Button
            variant="secondary"
            size="sm"
            onClick={async () => {
              const latest = await queryClient.fetchQuery({
                queryKey: ["page", pageId],
                queryFn: () => api.getPage(pageId),
              });
              await hydrateFromServer(latest.content, latest.revision);
            }}
          >
            Reload server version
          </Button>
        </div>
      ) : null}

      <div className="min-h-0 flex-1 overflow-auto px-6 py-5" onBlurCapture={flushSave}>
        <AladinPanel className="min-h-full overflow-hidden">
          <BlockNoteView
            editor={editor}
            editable
            theme="light"
            className="min-h-[520px]"
            onChange={() => {
              dirtyRef.current = true;
              setSaveState("idle");
              setMessage("Draft");
              scheduleSave();
            }}
          />
        </AladinPanel>
      </div>
    </div>
  );
}
