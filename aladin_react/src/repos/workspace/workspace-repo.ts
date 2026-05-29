import type {
  BrowserTreeNode,
  FolderCreateRequest,
  FolderNode,
  UserArtifactCreateRequest,
  Artifact,
} from "@/shared/api/models";
import type { BrowserNodeRow, NodeRow } from "@/repos/local-repo-types";
import type { BrowserNodeRepo } from "@/repos/workspace/browser-node-repo";
import type { NodeRepo } from "@/repos/workspace/node-repo";
import { artifactKindFromString, rowToArtifact } from "@/repos/artifacts/artifact-mappers";
import type { ApiClient } from "@/shared/api/client";
import type { LocalSyncRepo } from "@/repos/sync/local-sync-repo";

function fromBrowserNodeRow(row: BrowserNodeRow): FolderNode {
  return {
    id: row.id,
    parentId: row.parentId,
    title: row.title,
  };
}

/**
 * Materializes the nested browser tree from the flat, unified `nodes` rows.
 * A node's artifact preview is derived from its own columns (no separate
 * artifact join). Orphan- and cycle-tolerant: a node whose parent is missing,
 * is not a folder, or whose ancestor chain loops is placed at the root so the
 * tree always stays finite and renders.
 */
function toNodeTree(rows: NodeRow[]): BrowserTreeNode[] {
  const byId = new Map<string, BrowserTreeNode>();
  const folderIds = new Set<string>();
  const parentOf = new Map<string, string | null>();

  rows.forEach((row) => {
    const isArtifact = row.kind.toLowerCase() === "artifact";
    parentOf.set(row.id, row.parentId);
    byId.set(row.id, {
      id: row.id,
      parentId: row.parentId,
      kind: isArtifact ? "artifact" : "folder",
      title: row.title ?? "",
      artifactId: isArtifact ? row.id : undefined,
      artifactPreview: isArtifact
        ? {
            id: row.id,
            title: row.title ?? "",
            kind: artifactKindFromString(row.artifactType),
            updatedLabel: new Date(row.updatedAt).toISOString(),
          }
        : undefined,
      children: [],
    });
    if (!isArtifact) folderIds.add(row.id);
  });

  const inCycle = (id: string): boolean => {
    const seen = new Set<string>();
    let cur: string | null | undefined = id;
    while (cur) {
      if (seen.has(cur)) return true;
      seen.add(cur);
      cur = parentOf.get(cur) ?? null;
    }
    return false;
  };

  const roots: BrowserTreeNode[] = [];
  rows.forEach((row) => {
    const node = byId.get(row.id);
    if (!node) return;
    const parentId = row.parentId;
    if (
      parentId &&
      parentId !== row.id &&
      folderIds.has(parentId) &&
      byId.has(parentId) &&
      !inCycle(row.id)
    ) {
      byId.get(parentId)!.children.push(node);
    } else {
      roots.push(node);
    }
  });
  return roots;
}

export interface WorkspaceRepo {
  getBrowserTree(options?: {
    policy?: "local-first" | "remote";
  }): Promise<BrowserTreeNode[]>;
  /** Reads the tree directly from the local `nodes` model (no remote fetch). */
  getLocalNodeTree(): Promise<BrowserTreeNode[]>;
  createFolder(input: FolderCreateRequest): Promise<FolderNode>;
  createArtifact(input: UserArtifactCreateRequest): Promise<Artifact>;
  renameFolder(folderId: string, title: string): Promise<FolderNode>;
}

export function createWorkspaceRepo(
  browser: BrowserNodeRepo,
  nodes: NodeRepo,
  client?: ApiClient,
  localSync?: LocalSyncRepo,
): WorkspaceRepo {
  function createMutationId() {
    return crypto.randomUUID();
  }

  // Data-layer redesign: the tree is materialized from the unified local
  // `nodes` model — converged by the pull engine and local-write mirrors.
  async function getLocalTree() {
    return toNodeTree(await nodes.listNodes());
  }

  async function fetchAndSyncRemoteTree() {
    if (!localSync) {
      throw new Error("Local sync client is required for workspace refresh");
    }
    await localSync.pullNow();
    return getLocalTree();
  }

  return {
    async getBrowserTree(options) {
      if (options?.policy !== "remote") {
        const localTree = await getLocalTree();
        if (localTree.length > 0) return localTree;
      }
      return fetchAndSyncRemoteTree();
    },
    async getLocalNodeTree() {
      return getLocalTree();
    },
    async createFolder(input) {
      const result = await browser.createNode({
        id: createMutationId(),
        parentId: input.parentId ?? null,
        kind: "folder",
        title: input.title,
        updatedAt: Date.now(),
        mutationId: createMutationId(),
      });
      return fromBrowserNodeRow(result.node);
    },
    async createArtifact(input) {
      if (!input.type?.trim()) {
        throw new Error("Artifact type is required");
      }
      if (!client) {
        throw new Error("API client is required for artifact projection");
      }
      const result = await browser.createNode({
        id: createMutationId(),
        parentId: input.folderId ?? null,
        kind: "artifact",
        title: input.title ?? "",
        artifactType: input.type,
        content: input.content ?? null,
        summary: input.summary ?? null,
        sourceUrl: input.sourceUrl ?? null,
        updatedAt: Date.now(),
        mutationId: createMutationId(),
      });
      if (!result.artifact) {
        throw new Error("Artifact create did not return an artifact payload");
      }
      return rowToArtifact(client, result.artifact);
    },
    async renameFolder(folderId, title) {
      const current = await browser.getNode(folderId);
      const row = await browser.renameNode({
        id: folderId,
        parentId: current?.parentId ?? null,
        title,
        updatedAt: Date.now(),
        mutationId: createMutationId(),
      });
      return fromBrowserNodeRow(row);
    },
  };
}
