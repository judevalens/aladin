import { BehaviorSubject, Subscription, type Observable } from "rxjs";
import type { PageEditorMode, PageSaveState } from "@/modules/pages/domain";
import { PageSessionService as PageSessionLogic } from "@/services/pages/page-session-service";
import type { PageDocumentService } from "@/services/pages/page-document-service";

export interface PageSessionSnapshot {
  pageId: string;
  sessionReady: boolean;
  initialMarkdown: string;
  editorMode: PageEditorMode;
  blockNoteError: string | null;
  editorBoundaryKey: number;
  saveState: PageSaveState;
  saveMessage: string | null;
}

function initialSessionSnapshot(pageId: string): PageSessionSnapshot {
  return {
    pageId,
    sessionReady: false,
    initialMarkdown: "",
    editorMode: "blocknote",
    blockNoteError: null,
    editorBoundaryKey: 0,
    saveState: "idle",
    saveMessage: null,
  };
}

interface PageSessionEntry {
  logic: PageSessionLogic;
  subject: BehaviorSubject<PageSessionSnapshot>;
  documentSub: Subscription | null;
  saveTimer: number | null;
  saveInFlight: boolean;
}

export class PageSessionsService {
  private readonly sessions = new Map<string, PageSessionEntry>();

  constructor(private readonly documents: PageDocumentService) {}

  session(pageId: string): Observable<PageSessionSnapshot> {
    return this.getOrCreate(pageId).subject;
  }

  updateDraft(pageId: string, markdown: string) {
    const entry = this.getOrCreate(pageId);
    entry.logic.setDraft(markdown);
    this.scheduleSave(pageId);
  }

  flushSave(pageId: string) {
    const entry = this.sessions.get(pageId);
    if (!entry) return;
    if (entry.saveTimer !== null) {
      window.clearTimeout(entry.saveTimer);
      entry.saveTimer = null;
    }
    if (entry.saveInFlight) return;
    const request = entry.logic.createSaveRequest();
    if (!request) return;

    entry.saveInFlight = true;
    this.patchSession(pageId, { saveState: "saving", saveMessage: "Saving…" });

    void this.documents
      .savePage(pageId, request)
      .then((record) => {
        const result = entry.logic.handleSaveSuccess(record);
        entry.saveInFlight = false;
        this.patchSession(pageId, {
          saveState: result.saveState,
          saveMessage: result.message,
        });
        if (result.shouldReschedule) {
          this.scheduleSave(pageId, 250);
        }
      })
      .catch((error) => {
        const result = entry.logic.handleSaveError(error);
        entry.saveInFlight = false;
        this.patchSession(pageId, {
          saveState: result.saveState,
          saveMessage: result.message,
        });
      });
  }

  setDriverError(pageId: string, error: unknown) {
    const entry = this.getOrCreate(pageId);
    const message =
      error instanceof Error
        ? error.message
        : "BlockNote failed to initialize for this page.";
    this.patchSession(pageId, {
      initialMarkdown: entry.logic.getDraft(),
      editorMode: "markdown-fallback",
      blockNoteError: message,
    });
  }

  retryRichEditor(pageId: string) {
    const entry = this.getOrCreate(pageId);
    const current = entry.subject.getValue();
    this.patchSession(pageId, {
      initialMarkdown: entry.logic.getDraft(),
      editorMode: "blocknote",
      blockNoteError: null,
      editorBoundaryKey: current.editorBoundaryKey + 1,
    });
  }

  disposePage(pageId: string) {
    const entry = this.sessions.get(pageId);
    if (!entry) return;
    if (entry.saveTimer !== null) {
      window.clearTimeout(entry.saveTimer);
      entry.saveTimer = null;
    }
    this.flushSave(pageId);
    entry.documentSub?.unsubscribe();
    entry.subject.complete();
    this.sessions.delete(pageId);
  }

  private patchSession(pageId: string, patch: Partial<PageSessionSnapshot>) {
    const entry = this.sessions.get(pageId);
    if (!entry) return;
    entry.subject.next({ ...entry.subject.getValue(), ...patch });
  }

  private scheduleSave(pageId: string, delay = 900) {
    const entry = this.getOrCreate(pageId);
    if (entry.saveTimer !== null) {
      window.clearTimeout(entry.saveTimer);
    }
    entry.saveTimer = window.setTimeout(() => {
      this.flushSave(pageId);
    }, delay);
  }

  private getOrCreate(pageId: string): PageSessionEntry {
    const existing = this.sessions.get(pageId);
    if (existing) return existing;

    const entry: PageSessionEntry = {
      logic: new PageSessionLogic(),
      subject: new BehaviorSubject<PageSessionSnapshot>(
        initialSessionSnapshot(pageId),
      ),
      documentSub: null,
      saveTimer: null,
      saveInFlight: false,
    };
    this.sessions.set(pageId, entry);

    entry.documentSub = this.documents.document(pageId).subscribe((result) => {
      if (!result.ok) return;
      if (entry.subject.getValue().sessionReady) return;
      const snapshot = entry.logic.initialize(result.value);
      this.patchSession(pageId, {
        sessionReady: true,
        initialMarkdown: snapshot.content,
      });
    });

    return entry;
  }
}
