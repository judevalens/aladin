import { useCallback, useEffect, useMemo } from "react";
import { useAppComposition } from "@/app/composition/app-composition";
import { useObservableState } from "@/shared/flow/use-observable-state";
import type { PageEditorMode, PageSaveState } from "@/modules/pages/domain";

export interface PageState {
  loading: boolean;
  errorMessage: string | null;
  pageId: string;
  initialMarkdown: string;
  sessionReady: boolean;
  editorMode: PageEditorMode;
  blockNoteError: string | null;
  editorBoundaryKey: number;
  saveState: PageSaveState;
  saveMessage: string | null;
  onDraftChange: (markdown: string) => void;
  onBlur: () => void;
  onDriverError: (error: unknown) => void;
  onRetryRichEditor: () => void;
}

const FALLBACK_SESSION = {
  pageId: "",
  sessionReady: false,
  initialMarkdown: "",
  editorMode: "blocknote" as PageEditorMode,
  blockNoteError: null as string | null,
  editorBoundaryKey: 0,
  saveState: "idle" as PageSaveState,
  saveMessage: null as string | null,
};

export function usePageState(pageId: string): PageState {
  const { services } = useAppComposition();
  const { documents, sessions } = services.pages;

  useEffect(() => {
    return () => {
      sessions.disposePage(pageId);
    };
  }, [pageId, sessions]);

  const documentObservable = useMemo(
    () => documents.document(pageId),
    [documents, pageId],
  );
  const sessionObservable = useMemo(
    () => sessions.session(pageId),
    [pageId, sessions],
  );

  const documentState = useObservableState(documentObservable);
  const sessionState = useObservableState(sessionObservable);

  const session =
    sessionState.status === "data" ? sessionState.value : FALLBACK_SESSION;

  const handleDraftChange = useCallback(
    (markdown: string) => {
      sessions.updateDraft(pageId, markdown);
    },
    [pageId, sessions],
  );

  const handleDriverError = useCallback(
    (error: unknown) => {
      sessions.setDriverError(pageId, error);
    },
    [pageId, sessions],
  );

  const handleRetryRichEditor = useCallback(() => {
    sessions.retryRichEditor(pageId);
  }, [pageId, sessions]);

  return {
    loading: documentState.status === "loading",
    errorMessage:
      documentState.status === "error" ? documentState.error.message : null,
    pageId,
    initialMarkdown: session.initialMarkdown,
    sessionReady: session.sessionReady,
    editorMode: session.editorMode,
    blockNoteError: session.blockNoteError,
    editorBoundaryKey: session.editorBoundaryKey,
    saveState: session.saveState,
    saveMessage: session.saveMessage,
    onDraftChange: handleDraftChange,
    onBlur: () => sessions.flushSave(pageId),
    onDriverError: handleDriverError,
    onRetryRichEditor: handleRetryRichEditor,
  };
}
