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
      <div className="flex items-center justify-between border-b border-[#e7e5e4] px-8 py-3">
        <div className="min-w-0 flex-1">
          <h2 className="truncate text-[15px] font-semibold tracking-[-0.01em] text-[#0a0a0a]">{title}</h2>
        </div>
        <div className={`rounded-md border px-2 py-1 text-[11px] font-medium tabular-nums ${statusClassName}`}>
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
