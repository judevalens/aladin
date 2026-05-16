import { ApiError } from "@/shared/api/client";
import type { PageDocumentRecord } from "@/shared/api/models";
import type { PageSaveState } from "@/modules/pages/domain";

export interface PageSnapshot {
  content: string;
  revision: number;
}

export function snapshotFromRecord(record: PageDocumentRecord): PageSnapshot {
  return {
    content: record.content,
    revision: record.revision,
  };
}

export function nextPageRevision(currentRevision: number) {
  return currentRevision + 1;
}

export function isAcknowledgedConflict(
  snapshot: PageSnapshot | null,
  pendingMarkdown: string | null,
  pendingRevision: number | null,
) {
  return Boolean(
    snapshot &&
      pendingMarkdown !== null &&
      pendingRevision !== null &&
      snapshot.revision >= pendingRevision &&
      snapshot.content === pendingMarkdown,
  );
}

export function statusToneForPageSave(saveState: PageSaveState) {
  switch (saveState) {
    case "conflict":
    case "error":
      return "border-red-300 bg-red-50 text-red-700";
    default:
      return "border-gray-300 bg-white text-gray-700";
  }
}

export class PageSessionService {
  private lastServerSnapshot: PageSnapshot | null = null;
  private lastAcknowledgedRevision = 0;
  private lastSavedMarkdown = "";
  private draftMarkdown = "";
  private dirty = false;
  private pendingSaveMarkdown: string | null = null;
  private pendingSaveRevision: number | null = null;

  reset() {
    this.lastServerSnapshot = null;
    this.lastAcknowledgedRevision = 0;
    this.lastSavedMarkdown = "";
    this.draftMarkdown = "";
    this.dirty = false;
    this.pendingSaveMarkdown = null;
    this.pendingSaveRevision = null;
  }

  initialize(record: PageDocumentRecord) {
    const snapshot = snapshotFromRecord(record);
    this.lastServerSnapshot = snapshot;
    this.lastAcknowledgedRevision = snapshot.revision;
    this.lastSavedMarkdown = snapshot.content;
    this.draftMarkdown = snapshot.content;
    this.dirty = false;
    return snapshot;
  }

  snapshotFromRecord(record: PageDocumentRecord): PageSnapshot {
    return snapshotFromRecord(record);
  }

  setDraft(markdown: string) {
    this.draftMarkdown = markdown;
    this.dirty = true;
  }

  getDraft() {
    return this.draftMarkdown;
  }

  getRevision() {
    return this.lastAcknowledgedRevision;
  }

  getStatusTone(saveState: PageSaveState) {
    return statusToneForPageSave(saveState);
  }

  createSaveRequest() {
    if (!this.dirty || this.draftMarkdown === this.lastSavedMarkdown) {
      this.dirty = false;
      return null;
    }

    const nextRevision = nextPageRevision(this.lastAcknowledgedRevision);
    this.pendingSaveMarkdown = this.draftMarkdown;
    this.pendingSaveRevision = nextRevision;

    return {
      content: this.draftMarkdown,
      revision: nextRevision,
    };
  }

  handleSaveSuccess(record: PageDocumentRecord) {
    const snapshot = snapshotFromRecord(record);
    this.lastServerSnapshot = snapshot;
    this.lastAcknowledgedRevision = snapshot.revision;
    this.lastSavedMarkdown = snapshot.content;
    this.pendingSaveMarkdown = null;
    this.pendingSaveRevision = null;

    if (this.draftMarkdown === snapshot.content) {
      this.dirty = false;
      return {
        snapshot,
        saveState: "saved" as const,
        message: "Saved",
      };
    }

    this.dirty = true;
    return {
      snapshot,
      saveState: "idle" as const,
      message: "Draft",
      shouldReschedule: true,
    };
  }

  handleSaveError(error: unknown) {
    const acknowledgedSnapshot = this.lastServerSnapshot;

    if (
      error instanceof ApiError &&
      error.status === 409 &&
      acknowledgedSnapshot &&
      isAcknowledgedConflict(
        acknowledgedSnapshot,
        this.pendingSaveMarkdown,
        this.pendingSaveRevision,
      )
    ) {
      this.lastAcknowledgedRevision = acknowledgedSnapshot.revision;
      this.lastSavedMarkdown = acknowledgedSnapshot.content;
      this.dirty = false;
      this.pendingSaveMarkdown = null;
      this.pendingSaveRevision = null;
      return {
        saveState: "saved" as const,
        message: "Saved",
        revision: acknowledgedSnapshot.revision,
      };
    }

    this.pendingSaveMarkdown = null;
    this.pendingSaveRevision = null;

    if (error instanceof ApiError && error.status === 409) {
      return {
        saveState: "conflict" as const,
        message: "Save conflict. This draft was not saved.",
        revision: this.lastAcknowledgedRevision,
      };
    }

    return {
      saveState: "error" as const,
      message: error instanceof Error ? error.message : "Failed to save page.",
      revision: this.lastAcknowledgedRevision,
    };
  }
}
