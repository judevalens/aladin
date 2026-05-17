import { BlockNotePageEditorDriver } from "@/modules/pages/editor/page-editor-driver";
import type { PageScreenState } from "@/modules/pages/hooks/use-page-screen";

export function PageEditorView({
  state,
}: {
  state: PageScreenState;
}) {
  const {
    pageId,
    initialMarkdown,
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
            markdown={initialMarkdown}
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
