import { invoke } from "@tauri-apps/api/core";
import type { PageMetadataRow } from "@/repos/local-repo-types";

export interface PageMetadataRepo {
  getMetadata(id: string): Promise<PageMetadataRow | null>;
  upsertMetadata(row: PageMetadataRow): Promise<void>;
}

export function createPageMetadataRepo(): PageMetadataRepo {
  return {
    getMetadata: (id) => invoke<PageMetadataRow | null>("db_get_page_metadata", { id }),
    upsertMetadata: (row) => invoke("db_upsert_page_metadata", { row }),
  };
}
