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
      let row = await invoke<PageContentRow | null>("db_get_page_content", {
        id: pageId,
      });
      if (!row) {
        row = await invoke<PageContentRow>("db_pull_page_content", {
          id: pageId,
        });
      }
      const artifact = await artifacts.getById(pageId);
      return rowToRecord(row, artifact?.title ?? "");
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
