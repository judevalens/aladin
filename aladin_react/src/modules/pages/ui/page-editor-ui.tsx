import { BlockNotePageEditorDriver } from "@/modules/pages/editor/page-editor-driver";
import { usePageState } from "@/modules/pages/hooks/use-page-state";

export function PageEditorUI({ pageId }: { pageId: string }) {
  const state = usePageState(pageId);

  if (state.loading && !state.sessionReady) {
    return <div className="px-7 py-6 text-sm text-gray-700">Loading page…</div>;
  }

  if (state.errorMessage) {
    return <div className="px-7 py-6 text-sm text-red-700">{state.errorMessage}</div>;
  }

  const {
    initialBlocks,
    editorMode,
    blockNoteError,
    editorBoundaryKey,
    onDraftChange,
    onBlur,
    onDriverError,
    onRetryRichEditor,
  } = state;

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="min-h-0 flex-1 overflow-hidden">
        <div className="mx-auto flex h-full w-full max-w-workspace-max flex-col">
          <BlockNotePageEditorDriver
            pageId={pageId}
            initialBlocks={initialBlocks}
            mode={editorMode}
            blockNoteError={blockNoteError}
            editorBoundaryKey={editorBoundaryKey}
            onDraftChange={onDraftChange}
            onBlur={onBlur}
            onDriverError={onDriverError}
            onRetryRichEditor={onRetryRichEditor}
          />
        </div>
      </div>
    </div>
  );
}
