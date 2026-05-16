import { AladinPanel } from "@/components/ui/aladin";
import { usePageScreen } from "@/modules/pages/hooks/use-page-screen";
import { PageEditorView } from "@/modules/pages/ui/page-editor-view";

export function PageEditorScreen({ pageId }: { pageId: string }) {
  const screen = usePageScreen(pageId);

  if (screen.loading && !screen.sessionReady) {
    return <div className="px-7 py-6 text-sm text-gray-700">Loading page…</div>;
  }

  if (screen.errorMessage) {
    return <div className="px-7 py-6 text-sm text-red-700">{screen.errorMessage}</div>;
  }

  return (
    <PageEditorView
      title={screen.title}
      message={screen.message}
      revision={screen.revision}
      saveState={screen.saveState}
      statusClassName={screen.statusClassName}
      pageId={screen.pageId}
      initialMarkdown={screen.initialMarkdown}
      editorMode={screen.editorMode}
      blockNoteError={screen.blockNoteError}
      editorBoundaryKey={screen.editorBoundaryKey}
      onDraftChange={screen.onDraftChange}
      onBlur={screen.onBlur}
      onDriverError={screen.onDriverError}
      onRetryRichEditor={screen.onRetryRichEditor}
    />
  );
}
