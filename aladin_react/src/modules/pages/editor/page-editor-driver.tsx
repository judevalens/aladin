import { useCreateBlockNote } from "@blocknote/react";
import { BlockNoteView } from "@blocknote/shadcn";
import { Component, type ReactNode, useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import type { PageEditorMode } from "@/modules/pages/domain";
import {
  exportEditorToMarkdown,
  loadMarkdownIntoEditor,
} from "@/modules/pages/editor/markdown-adapter";
import "@blocknote/shadcn/style.css";

export interface PageEditorDriverProps {
  pageId: string;
  markdown: string;
  mode: PageEditorMode;
  blockNoteError: string | null;
  editorBoundaryKey: number;
  onDraftChange: (markdown: string) => void;
  onBlur: () => void;
  onRetryRichEditor: () => void;
  onDriverError: (error: unknown) => void;
}

interface BlockNoteRuntimeBoundaryProps {
  fallback: ReactNode;
  onError: (error: Error) => void;
  children: ReactNode;
}

class BlockNoteRuntimeBoundary extends Component<
  BlockNoteRuntimeBoundaryProps,
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

export interface PageEditorDriver {
  load(markdown: string): Promise<void>;
  export(): string;
  onChange(listener: (markdown: string) => void): () => void;
  retry?(): void;
  attachCollaboration?(input: unknown): void;
}

export function BlockNotePageEditorDriver({
  pageId,
  markdown,
  mode,
  blockNoteError,
  editorBoundaryKey,
  onDraftChange,
  onBlur,
  onRetryRichEditor,
  onDriverError,
}: PageEditorDriverProps) {
  const editor = useCreateBlockNote({}, [pageId, editorBoundaryKey]);
  const [fallbackMarkdown, setFallbackMarkdown] = useState(markdown);

  useEffect(() => {
    setFallbackMarkdown(markdown);
  }, [markdown, pageId, editorBoundaryKey, mode]);

  useEffect(() => {
    if (mode !== "blocknote") return;

    void loadMarkdownIntoEditor(editor, markdown).catch((error) => {
      onDriverError(error);
    });
  }, [editor, markdown, mode, onDriverError]);

  if (mode === "markdown-fallback") {
    return (
      <div className="flex min-h-[520px] flex-1 flex-col">
        <div className="flex items-center justify-between gap-4 border-b border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
          <div className="space-y-1">
            <div className="font-medium">
              The rich editor is unavailable for this page.
            </div>
            <div className="text-xs leading-5 text-amber-700">
              {blockNoteError ??
                "You can keep working in raw markdown for now."}
            </div>
          </div>
          <Button variant="secondary" size="sm" onClick={onRetryRichEditor}>
            Retry editor
          </Button>
        </div>
        <textarea
          className="min-h-0 flex-1 resize-none border-0 bg-transparent px-5 py-4 font-mono text-sm leading-7 text-black outline-none"
          value={fallbackMarkdown}
          onBlur={onBlur}
          onChange={(event) => {
            const nextMarkdown = event.target.value;
            setFallbackMarkdown(nextMarkdown);
            onDraftChange(nextMarkdown);
          }}
        />
      </div>
    );
  }

  return (
    <div
      className="relative flex min-h-0 flex-1 flex-col"
      onBlurCapture={onBlur}
    >
      <div className="min-h-0 flex-1 overflow-y-auto px-8 py-4">
        <BlockNoteRuntimeBoundary
          key={`${pageId}:${editorBoundaryKey}`}
          onError={onDriverError}
          fallback={
            <div className="border border-gray-300 bg-white px-4 py-3 text-sm text-gray-700">
              Switching to markdown fallback…
            </div>
          }
        >
          <BlockNoteView
            editor={editor}
            slashMenu={true}
            formattingToolbar={true}
            linkToolbar={true}
            onChange={() => {
              onDraftChange(exportEditorToMarkdown(editor));
            }}
            theme="light"
          />
        </BlockNoteRuntimeBoundary>
      </div>
    </div>
  );
}
