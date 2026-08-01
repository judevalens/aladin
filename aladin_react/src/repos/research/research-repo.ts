import type { ApiClient } from "@/shared/api/client";

/**
 * A research folder's strategy facts — the fields the tree frame deliberately does NOT
 * carry (RESEARCH_SURFACE_PRD §5). Light fields (run state, exec mode, source kind) ride
 * the node's sync frame so every tree row can render them; these heavier ones are fetched
 * when the Overview opens, which is what stops a manifest edit from re-broadcasting to
 * every row in the tree.
 */
export interface ResearchFolder {
  nodeId: string;
  title: string;
  parentId: string | null;
  hypothesis: string;
  sourceKind: string;
  sourceRef?: string;
  commitSha?: string;
  codeHash?: string;
  execMode: string;
  runState: string;
}

export interface ResearchRepo {
  get(nodeId: string): Promise<ResearchFolder>;
  setHypothesis(nodeId: string, hypothesis: string): Promise<void>;
}

export function createResearchRepo(client: ApiClient): ResearchRepo {
  return {
    get(nodeId) {
      return client.fetch<ResearchFolder>(`/api/research/${encodeURIComponent(nodeId)}`);
    },
    async setHypothesis(nodeId, hypothesis) {
      await client.fetch(`/api/research/${encodeURIComponent(nodeId)}`, {
        method: "PATCH",
        body: JSON.stringify({ hypothesis }),
      });
    },
  };
}
