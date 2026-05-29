import { invoke } from "@tauri-apps/api/core";
import type { NodeRow } from "@/repos/local-repo-types";

/**
 * Data-layer redesign, Phase A — reads the unified local `nodes` tree. This is
 * the authoritative local model: the pull engine applies the server change feed
 * into it, and local mutations mirror their result into it. Replaces the split
 * browser-node + artifact-row reads.
 */
export interface NodeRepo {
  listNodes(): Promise<NodeRow[]>;
  getNode(id: string): Promise<NodeRow | null>;
}

export function createNodeRepo(): NodeRepo {
  return {
    listNodes: () => invoke<NodeRow[]>("db_list_nodes"),
    getNode: (id) => invoke<NodeRow | null>("db_get_node", { id }),
  };
}
