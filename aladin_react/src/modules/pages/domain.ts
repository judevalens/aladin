export type PageEditorMode = "blocknote" | "markdown-fallback";
export type PageSaveState = "idle" | "saving" | "saved" | "conflict" | "error";

export interface PageSessionMeta {
  saveState: PageSaveState;
  message: string | null;
  revision: number;
  editorMode: PageEditorMode;
  blockNoteError: string | null;
  editorBoundaryKey: number;
}

export function createDefaultPageSessionMeta(): PageSessionMeta {
  return {
    saveState: "idle",
    message: null,
    revision: 0,
    editorMode: "blocknote",
    blockNoteError: null,
    editorBoundaryKey: 0,
  };
}
