import { BlockNotePageEditorDriver } from "@/modules/pages/editor/page-editor-driver";
import type { PageEditorMode, PageSaveState } from "@/modules/pages/domain";

interface PageEditorViewProps {
  title: string;
  message: string | null;
  revision: number;
  saveState: PageSaveState;
  statusClassName: string;
  pageId: string;
  initialMarkdown: string;
  editorMode: PageEditorMode;
  blockNoteError: string | null;
  editorBoundaryKey: number;
  onDraftChange: (markdown: string) => void;
  onBlur: () => void;
  onDriverError: (error: unknown) => void;
  onRetryRichEditor: () => void;
}

export function PageEditorView({
  title,
  message,
  revision,
  statusClassName,
  pageId,
  initialMarkdown,
  editorMode,
  blockNoteError,
  editorBoundaryKey,
  onDraftChange,
  onBlur,
  onDriverError,
  onRetryRichEditor,
}: PageEditorViewProps) {
  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex items-center justify-between border-b border-[#e4e4e4] px-7 py-4">
        <div className="space-y-1">
          <div className="text-xs text-[#6b7280]">Page</div>
          <h2 className="text-2xl font-semibold text-[#111111]">{title}</h2>
        </div>
        <div className={`rounded-md border px-3 py-2 text-xs font-medium ${statusClassName}`}>
          {message ?? `Revision ${revision}`}
        </div>
      </div>

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
