import type { StateCreator } from "zustand";
import { createDefaultPageSessionMeta, type PageEditorMode, type PageSaveState, type PageSessionMeta } from "@/modules/pages/domain";

export interface PageSessionSlice {
  pageSessions: Record<string, PageSessionMeta>;
  ensurePageSession: (pageId: string) => void;
  resetPageSession: (pageId: string) => void;
  patchPageSession: (
    pageId: string,
    patch: Partial<PageSessionMeta> | ((current: PageSessionMeta) => Partial<PageSessionMeta>),
  ) => void;
  setPageEditorMode: (pageId: string, mode: PageEditorMode, blockNoteError?: string | null) => void;
  setPageSaveState: (pageId: string, saveState: PageSaveState, message: string | null) => void;
  setPageRevision: (pageId: string, revision: number) => void;
  bumpPageEditorBoundary: (pageId: string) => void;
}

function patchPageSessionRecord(
  pageSessions: Record<string, PageSessionMeta>,
  pageId: string,
  patch: Partial<PageSessionMeta> | ((current: PageSessionMeta) => Partial<PageSessionMeta>),
) {
  const current = pageSessions[pageId] ?? createDefaultPageSessionMeta();
  const nextPatch = typeof patch === "function" ? patch(current) : patch;
  return {
    ...pageSessions,
    [pageId]: {
      ...current,
      ...nextPatch,
    },
  };
}

export const createPageSessionSlice: StateCreator<PageSessionSlice, [], [], PageSessionSlice> = (set) => ({
  pageSessions: {},
  ensurePageSession: (pageId) =>
    set((state) => ({
      pageSessions: state.pageSessions[pageId]
        ? state.pageSessions
        : {
            ...state.pageSessions,
            [pageId]: createDefaultPageSessionMeta(),
          },
    })),
  resetPageSession: (pageId) =>
    set((state) => ({
      pageSessions: {
        ...state.pageSessions,
        [pageId]: createDefaultPageSessionMeta(),
      },
    })),
  patchPageSession: (pageId, patch) =>
    set((state) => ({
      pageSessions: patchPageSessionRecord(state.pageSessions, pageId, patch),
    })),
  setPageEditorMode: (pageId, mode, blockNoteError = null) =>
    set((state) => ({
      pageSessions: patchPageSessionRecord(state.pageSessions, pageId, {
        editorMode: mode,
        blockNoteError,
      }),
    })),
  setPageSaveState: (pageId, saveState, message) =>
    set((state) => ({
      pageSessions: patchPageSessionRecord(state.pageSessions, pageId, {
        saveState,
        message,
      }),
    })),
  setPageRevision: (pageId, revision) =>
    set((state) => ({
      pageSessions: patchPageSessionRecord(state.pageSessions, pageId, { revision }),
    })),
  bumpPageEditorBoundary: (pageId) =>
    set((state) => ({
      pageSessions: patchPageSessionRecord(state.pageSessions, pageId, (current) => ({
        editorBoundaryKey: current.editorBoundaryKey + 1,
      })),
    })),
});
