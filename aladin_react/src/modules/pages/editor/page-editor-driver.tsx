import type { PartialBlock } from "@blocknote/core";
import { useCreateBlockNote } from "@blocknote/react";
import { BlockNoteView } from "@blocknote/shadcn";
import { Component, type ReactNode, useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import type { PageEditorMode } from "@/modules/pages/domain";
import type { BlockNoteDocument } from "@/shared/api/models";
import "@blocknote/shadcn/style.css";

export interface PageEditorDriverProps {
  pageId: string;
  initialBlocks: BlockNoteDocument;
  mode: PageEditorMode;
  blockNoteError: string | null;
  editorBoundaryKey: number;
  onDraftChange: (blocks: BlockNoteDocument) => void;
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

function asPartialBlocks(blocks: BlockNoteDocument): PartialBlock[] {
  // The wire format is unknown[]; BlockNote's editor accepts PartialBlock[]
  // structurally. We cast at this single boundary so the rest of the app
  // doesn't need to know BlockNote's internal types.
  return blocks as PartialBlock[];
}

export function BlockNotePageEditorDriver({
  pageId,
  initialBlocks,
  mode,
  blockNoteError,
  editorBoundaryKey,
  onDraftChange,
  onBlur,
  onRetryRichEditor,
  onDriverError,
}: PageEditorDriverProps) {
  const editor = useCreateBlockNote({}, [pageId, editorBoundaryKey]);
  const [fallbackJSON, setFallbackJSON] = useState(() =>
    JSON.stringify(initialBlocks, null, 2),
  );

  useEffect(() => {
    setFallbackJSON(JSON.stringify(initialBlocks, null, 2));
  }, [initialBlocks, pageId, editorBoundaryKey, mode]);

  useEffect(() => {
    if (mode !== "blocknote") return;
    try {
      const blocks = asPartialBlocks(initialBlocks);
      if (blocks.length === 0) {
        editor.replaceBlocks(editor.document, [{ type: "paragraph" }]);
      } else {
        editor.replaceBlocks(editor.document, blocks);
      }
    } catch (error) {
      onDriverError(error);
    }
  }, [editor, initialBlocks, mode, onDriverError]);

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
                "Editing is read-only until BlockNote recovers."}
            </div>
          </div>
          <Button variant="secondary" size="sm" onClick={onRetryRichEditor}>
            Retry editor
          </Button>
        </div>
        <textarea
          readOnly
          className="min-h-0 flex-1 resize-none border-0 bg-transparent px-5 py-4 font-mono text-xs leading-6 text-black outline-none"
          value={fallbackJSON}
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
              Switching to read-only JSON view…
            </div>
          }
        >
          <BlockNoteView
            editor={editor}
            slashMenu={true}
            formattingToolbar={true}
            linkToolbar={true}
            onChange={() => {
              onDraftChange(editor.document as BlockNoteDocument);
            }}
            theme="light"
          />
        </BlockNoteRuntimeBoundary>
      </div>
    </div>
  );
}
