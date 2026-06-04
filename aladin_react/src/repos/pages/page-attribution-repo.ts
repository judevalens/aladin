import type { ApiClient } from "@/shared/api/client";

// Block-level agent presence — the per-block attribution map for a page:
// blockId -> who last authored/edited that block via an agent (the collab
// bridge is the only writer that knows an agent did it). The page editor reads
// this and renders a per-block marker. See backend GET /api/pages/{id}/attribution.
export interface BlockAttributionEntry {
  by: string;
  at: string;
}

export type BlockAttribution = Record<string, BlockAttributionEntry>;

// One coalesced edit session in a page's history (humans + agents).
export interface PageEditEntry {
  editorKind: "human" | "agent";
  editorName: string;
  occurredAt: string;
  endedAt: string;
  edits: number;
}

export interface PageAttributionRepo {
  getAttribution(pageId: string): Promise<BlockAttribution>;
  getHistory(pageId: string): Promise<PageEditEntry[]>;
}

export function createPageAttributionRepo(client: ApiClient): PageAttributionRepo {
  return {
    getAttribution: (pageId) =>
      client
        .fetch<BlockAttribution>(`/api/pages/${pageId}/attribution`)
        .catch(() => ({})),
    getHistory: (pageId) =>
      client
        .fetch<PageEditEntry[]>(`/api/pages/${pageId}/history`)
        .catch(() => []),
  };
}
