import { BehaviorSubject, type Observable } from "rxjs";
import type { PageRepo } from "@/repos/pages/page-repo";
import {
  PageSessionService as PageSessionLogic,
  statusToneForPageSave,
} from "@/services/pages/page-session-service";
import type { PageEditorMode, PageSaveState } from "@/modules/pages/domain";
import type { PageDocumentRecord } from "@/shared/api/models";

export interface PageScreenSnapshot {
  pageId: string;
  loading: boolean;
  errorMessage: string | null;
  title: string;
  initialMarkdown: string;
  sessionReady: boolean;
  saveState: PageSaveState;
  message: string | null;
  revision: number;
  editorMode: PageEditorMode;
  blockNoteError: string | null;
  editorBoundaryKey: number;
}

function createInitialPageScreenSnapshot(pageId: string): PageScreenSnapshot {
  return {
    pageId,
    loading: true,
    errorMessage: null,
    title: "Untitled page",
    initialMarkdown: "",
    sessionReady: false,
    saveState: "idle",
    message: null,
    revision: 0,
    editorMode: "blocknote",
    blockNoteError: null,
    editorBoundaryKey: 0,
  };
}

interface PageSessionEntry {
  logic: PageSessionLogic;
  subject: BehaviorSubject<PageScreenSnapshot>;
  saveTimer: number | null;
  saveInFlight: boolean;
  loadPromise: Promise<void> | null;
}

export class PageSessionsService {
  private readonly sessions = new Map<string, PageSessionEntry>();

  constructor(private readonly pageRepo: PageRepo) {}

  observePage(pageId: string): Observable<PageScreenSnapshot> {
    return this.getOrCreate(pageId).subject.asObservable();
  }

  getPageSnapshot(pageId: string): PageScreenSnapshot {
    return this.getOrCreate(pageId).subject.getValue();
  }

  getStatusTone(saveState: PageSaveState) {
    return statusToneForPageSave(saveState);
  }

  async ensureLoaded(pageId: string) {
    const session = this.getOrCreate(pageId);
    const snapshot = session.subject.getValue();
    if (snapshot.sessionReady || snapshot.loading && session.loadPromise) {
      return session.loadPromise ?? Promise.resolve();
    }

    if (!session.loadPromise) {
      session.subject.next({
        ...snapshot,
        loading: true,
        errorMessage: null,
      });
      session.loadPromise = this.pageRepo
        .getPage(pageId)
        .then((record) => {
          this.initializeSession(pageId, record);
        })
        .catch((error) => {
          session.subject.next({
            ...session.subject.getValue(),
            loading: false,
            errorMessage: error instanceof Error ? error.message : "Failed to load page.",
          });
          throw error;
        })
        .finally(() => {
          session.loadPromise = null;
        });
    }

    return session.loadPromise;
  }

  updateDraft(pageId: string, markdown: string) {
    const session = this.getOrCreate(pageId);
    session.logic.setDraft(markdown);
    session.subject.next({
      ...session.subject.getValue(),
      saveState: "idle",
      message: "Draft",
    });
    this.scheduleSave(pageId);
  }

  flushSave(pageId: string) {
    const session = this.getOrCreate(pageId);
    if (session.saveTimer !== null) {
      window.clearTimeout(session.saveTimer);
      session.saveTimer = null;
    }
    if (session.saveInFlight) {
      return;
    }
    const request = session.logic.createSaveRequest();
    if (!request) {
      return;
    }

    session.saveInFlight = true;
    session.subject.next({
      ...session.subject.getValue(),
      saveState: "saving",
      message: "Saving…",
    });

    void this.pageRepo
      .savePage(pageId, {
        content: request.content,
        revision: request.revision,
      })
      .then((record) => {
        const result = session.logic.handleSaveSuccess(record);
        session.saveInFlight = false;
        const current = session.subject.getValue();
        session.subject.next({
          ...current,
          title: record.title,
          revision: result.snapshot.revision,
          saveState: result.saveState,
          message: result.message,
          errorMessage: null,
        });
        if (result.shouldReschedule) {
          this.scheduleSave(pageId, 250);
        }
      })
      .catch((error) => {
        session.saveInFlight = false;
        const result = session.logic.handleSaveError(error);
        session.subject.next({
          ...session.subject.getValue(),
          revision: result.revision,
          saveState: result.saveState,
          message: result.message,
        });
      });
  }

  setDriverError(pageId: string, error: unknown) {
    const session = this.getOrCreate(pageId);
    const message =
      error instanceof Error ? error.message : "BlockNote failed to initialize for this page.";
    session.subject.next({
      ...session.subject.getValue(),
      initialMarkdown: session.logic.getDraft(),
      editorMode: "markdown-fallback",
      blockNoteError: message,
    });
  }

  retryRichEditor(pageId: string) {
    const session = this.getOrCreate(pageId);
    const current = session.subject.getValue();
    session.subject.next({
      ...current,
      initialMarkdown: session.logic.getDraft(),
      editorMode: "blocknote",
      blockNoteError: null,
      editorBoundaryKey: current.editorBoundaryKey + 1,
    });
  }

  disposePage(pageId: string) {
    const session = this.sessions.get(pageId);
    if (!session) {
      return;
    }
    if (session.saveTimer !== null) {
      window.clearTimeout(session.saveTimer);
    }
    this.flushSave(pageId);
  }

  private initializeSession(pageId: string, record: PageDocumentRecord) {
    const session = this.getOrCreate(pageId);
    const snapshot = session.logic.initialize(record);
    session.subject.next({
      pageId,
      loading: false,
      errorMessage: null,
      title: record.title,
      initialMarkdown: snapshot.content,
      sessionReady: true,
      saveState: "idle",
      message: null,
      revision: snapshot.revision,
      editorMode: "blocknote",
      blockNoteError: null,
      editorBoundaryKey: 0,
    });
  }

  private scheduleSave(pageId: string, delay = 900) {
    const session = this.getOrCreate(pageId);
    if (session.saveTimer !== null) {
      window.clearTimeout(session.saveTimer);
    }
    session.saveTimer = window.setTimeout(() => {
      this.flushSave(pageId);
    }, delay);
  }

  private getOrCreate(pageId: string) {
    const existing = this.sessions.get(pageId);
    if (existing) {
      return existing;
    }

    const entry: PageSessionEntry = {
      logic: new PageSessionLogic(),
      subject: new BehaviorSubject<PageScreenSnapshot>(createInitialPageScreenSnapshot(pageId)),
      saveTimer: null,
      saveInFlight: false,
      loadPromise: null,
    };
    this.sessions.set(pageId, entry);
    return entry;
  }
}
