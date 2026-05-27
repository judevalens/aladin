import { invoke } from "@tauri-apps/api/core";
import type { ArtifactRowRepo } from "@/repos/artifacts/artifact-row-repo";
import type {
  LocalPageContentSaveInput,
  PageContentRow,
} from "@/repos/local-repo-types";
import type {
  BlockNoteDocument,
  PageDocumentRecord,
  PageSaveRequest,
} from "@/shared/api/models";

export interface PageRepo {
  getPage(pageId: string): Promise<PageDocumentRecord>;
  savePage(pageId: string, input: PageSaveRequest): Promise<PageDocumentRecord>;
}

/**
 * Local-first page content repo. Reads come from SQLite; on a cache miss
 * we pull from the Go API once and populate local. Writes go through
 * `db_upsert_page_content`, which queues an outbox mutation that the
 * Rust sync engine flushes to PATCH /api/pages/{id}. The frontend
 * never talks to the Go API directly for page content after M7.
 *
 * The artifact title isn't part of page_content — fetch it from the
 * artifact row (already in local SQLite from the workspace tree sync).
 */
export function createPageRepo(artifacts: ArtifactRowRepo): PageRepo {
  return {
    async getPage(pageId) {
      const row = await invoke<PageContentRow | null>("db_get_page_content", {
        id: pageId,
      });
      const artifact = await artifacts.getById(pageId);
      if (row) {
        return rowToRecord(row, artifact?.title ?? "");
      }
      // No local content row. Two cases:
      //   1. Artifact is local and PENDING — this is a brand-new page
      //      that hasn't flushed to the server yet. Pulling is doomed
      //      (404 or connection refused if offline); start with an
      //      empty document. The first save populates page_content
      //      locally and the sync engine pushes it.
      //   2. Artifact is SYNCED (or we don't have it locally) — we
      //      genuinely need to fetch from the server. Surface pull
      //      errors; the user can't safely edit a page whose canonical
      //      content lives on a server we can't reach.
      if (artifact && artifact.syncStatus === "PENDING") {
        return emptyRecord(pageId, artifact?.title ?? "");
      }
      const pulled = await invoke<PageContentRow>("db_pull_page_content", {
        id: pageId,
      });
      return rowToRecord(pulled, artifact?.title ?? "");
    },
    async savePage(pageId, input) {
      const saveInput: LocalPageContentSaveInput = {
        id: pageId,
        blocks: JSON.stringify(input.blocks),
        revision: input.revision,
        updatedAt: Date.now(),
        mutationId: crypto.randomUUID(),
      };
      const row = await invoke<PageContentRow>("db_upsert_page_content", {
        input: saveInput,
      });
      const artifact = await artifacts.getById(pageId);
      return rowToRecord(row, artifact?.title ?? "");
    },
  };
}

function emptyRecord(id: string, title: string): PageDocumentRecord {
  return {
    id,
    title,
    blocks: [],
    revision: 0,
    updatedAt: new Date().toISOString(),
  };
}

function rowToRecord(row: PageContentRow, title: string): PageDocumentRecord {
  let blocks: BlockNoteDocument;
  try {
    blocks = JSON.parse(row.blocks);
    if (!Array.isArray(blocks)) blocks = [];
  } catch {
    blocks = [];
  }
  return {
    id: row.id,
    title,
    blocks,
    revision: row.revision,
    updatedAt: new Date(row.updatedAt).toISOString(),
  };
}
