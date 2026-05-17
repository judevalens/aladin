import { useCallback, useEffect, useMemo } from "react";
import { useAppComposition } from "@/app/composition/app-composition";
import { useObservableState } from "@/shared/flow/use-observable-state";

export interface PageScreenState {
  loading: boolean;
  errorMessage: string | null;
  title: string;
  pageId: string;
  initialMarkdown: string;
  sessionReady: boolean;
  saveState: import("@/modules/pages/domain").PageSaveState;
  message: string | null;
  revision: number;
  editorMode: import("@/modules/pages/domain").PageEditorMode;
  blockNoteError: string | null;
  editorBoundaryKey: number;
  statusClassName: string;
  onDraftChange: (markdown: string) => void;
  onBlur: () => void;
  onDriverError: (error: unknown) => void;
  onRetryRichEditor: () => void;
}

export function usePageScreen(pageId: string): PageScreenState {
  const { services } = useAppComposition();

  useEffect(() => {
    void services.pages.sessions.ensureLoaded(pageId);
    return () => {
      services.pages.sessions.disposePage(pageId);
    };
  }, [pageId, services.pages.sessions]);

  const page = useObservableState(
    () => services.pages.sessions.observePage(pageId),
    () => services.pages.sessions.getPageSnapshot(pageId),
    [services.pages.sessions, pageId],
  );

  const handleDraftChange = useCallback(
    (markdown: string) => {
      services.pages.sessions.updateDraft(pageId, markdown);
    },
    [pageId, services.pages.sessions],
  );

  const handleDriverError = useCallback(
    (error: unknown) => {
      services.pages.sessions.setDriverError(pageId, error);
    },
    [pageId, services.pages.sessions],
  );

  const handleRetryRichEditor = useCallback(() => {
    services.pages.sessions.retryRichEditor(pageId);
  }, [pageId, services.pages.sessions]);

  const statusClassName = useMemo(
    () => services.pages.sessions.getStatusTone(page.saveState),
    [page.saveState, services.pages.sessions],
  );

  return {
    loading: page.loading,
    errorMessage: page.errorMessage,
    title: page.title,
    pageId,
    initialMarkdown: page.initialMarkdown,
    sessionReady: page.sessionReady,
    saveState: page.saveState,
    message: page.message,
    revision: page.revision,
    editorMode: page.editorMode,
    blockNoteError: page.blockNoteError,
    editorBoundaryKey: page.editorBoundaryKey,
    statusClassName,
    onDraftChange: handleDraftChange,
    onBlur: () => services.pages.sessions.flushSave(pageId),
    onDriverError: handleDriverError,
    onRetryRichEditor: handleRetryRichEditor,
  };
}
