import { ApiError } from "@/shared/api/client";
import type {
  BlockNoteDocument,
  PageDocumentRecord,
} from "@/shared/api/models";

export interface PageSnapshot {
  blocks: BlockNoteDocument;
  revision: number;
}

export function snapshotFromRecord(record: PageDocumentRecord): PageSnapshot {
  return {
    blocks: record.blocks,
    revision: record.revision,
  };
}

export function nextPageRevision(currentRevision: number) {
  return currentRevision + 1;
}

function blocksEqual(a: BlockNoteDocument, b: BlockNoteDocument): boolean {
  // Cheap structural equality for the dirty-check. BlockNote produces stable
  // ids and stable property ordering for a given document, so JSON.stringify
  // is a reasonable identity test. If this proves false-positive in practice
  // we can switch to a recursive walk that ignores cosmetic key order.
  return JSON.stringify(a) === JSON.stringify(b);
}

export function isAcknowledgedConflict(
  snapshot: PageSnapshot | null,
  pendingBlocks: BlockNoteDocument | null,
  pendingRevision: number | null,
) {
  return Boolean(
    snapshot &&
      pendingBlocks !== null &&
      pendingRevision !== null &&
      snapshot.revision >= pendingRevision &&
      blocksEqual(snapshot.blocks, pendingBlocks),
  );
}

export class PageSessionService {
  private lastServerSnapshot: PageSnapshot | null = null;
  private lastAcknowledgedRevision = 0;
  private lastSavedBlocks: BlockNoteDocument = [];
  private draftBlocks: BlockNoteDocument = [];
  private dirty = false;
  private pendingSaveBlocks: BlockNoteDocument | null = null;
  private pendingSaveRevision: number | null = null;

  reset() {
    this.lastServerSnapshot = null;
    this.lastAcknowledgedRevision = 0;
    this.lastSavedBlocks = [];
    this.draftBlocks = [];
    this.dirty = false;
    this.pendingSaveBlocks = null;
    this.pendingSaveRevision = null;
  }

  initialize(record: PageDocumentRecord) {
    const snapshot = snapshotFromRecord(record);
    this.lastServerSnapshot = snapshot;
    this.lastAcknowledgedRevision = snapshot.revision;
    this.lastSavedBlocks = snapshot.blocks;
    this.draftBlocks = snapshot.blocks;
    this.dirty = false;
    return snapshot;
  }

  snapshotFromRecord(record: PageDocumentRecord): PageSnapshot {
    return snapshotFromRecord(record);
  }

  setDraft(blocks: BlockNoteDocument) {
    this.draftBlocks = blocks;
    this.dirty = true;
  }

  getDraft(): BlockNoteDocument {
    return this.draftBlocks;
  }

  createSaveRequest() {
    if (!this.dirty || blocksEqual(this.draftBlocks, this.lastSavedBlocks)) {
      this.dirty = false;
      return null;
    }

    const nextRevision = nextPageRevision(this.lastAcknowledgedRevision);
    this.pendingSaveBlocks = this.draftBlocks;
    this.pendingSaveRevision = nextRevision;

    return {
      blocks: this.draftBlocks,
      revision: nextRevision,
    };
  }

  handleSaveSuccess(record: PageDocumentRecord) {
    const snapshot = snapshotFromRecord(record);
    this.lastServerSnapshot = snapshot;
    this.lastAcknowledgedRevision = snapshot.revision;
    this.lastSavedBlocks = snapshot.blocks;
    this.pendingSaveBlocks = null;
    this.pendingSaveRevision = null;

    if (blocksEqual(this.draftBlocks, snapshot.blocks)) {
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
        this.pendingSaveBlocks,
        this.pendingSaveRevision,
      )
    ) {
      this.lastAcknowledgedRevision = acknowledgedSnapshot.revision;
      this.lastSavedBlocks = acknowledgedSnapshot.blocks;
      this.dirty = false;
      this.pendingSaveBlocks = null;
      this.pendingSaveRevision = null;
      return {
        saveState: "saved" as const,
        message: "Saved",
        revision: acknowledgedSnapshot.revision,
      };
    }

    this.pendingSaveBlocks = null;
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
