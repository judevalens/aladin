import type {
  BrowserTreeNode,
  FolderCreateRequest,
  FolderNode,
  UserArtifactCreateRequest,
  Artifact,
  ResearchNodeMeta,
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
/**
 * Parses the research extension payload the sync frame carried. Tolerant on purpose: a
 * malformed or absent payload degrades the row to a plain container rather than
 * breaking the whole tree build.
 */
function parseResearchMeta(raw: string | null): ResearchNodeMeta | undefined {
  if (!raw) return undefined;
  try {
    const parsed = JSON.parse(raw) as Partial<ResearchNodeMeta>;
    return {
      runState: parsed.runState ?? "idle",
      execMode: parsed.execMode ?? "event",
      sourceKind: parsed.sourceKind ?? "authored",
    };
  } catch {
    return undefined;
  }
}

function toNodeTree(rows: NodeRow[]): BrowserTreeNode[] {
  const byId = new Map<string, BrowserTreeNode>();
  const folderIds = new Set<string>();
  const parentOf = new Map<string, string | null>();

  rows.forEach((row) => {
    const kind = row.kind.toLowerCase();
    const isArtifact = kind === "artifact";
    // A research folder keeps its own kind (RESEARCH_SURFACE_PRD §5) — collapsing it to
    // "folder" here is what would erase its icon and its state. Anything else
    // non-artifact still falls back to "folder" so an unknown future kind renders as a
    // container rather than vanishing.
    const isResearch = kind === "research";
    parentOf.set(row.id, row.parentId);
    byId.set(row.id, {
      id: row.id,
      parentId: row.parentId,
      kind: isArtifact ? "artifact" : isResearch ? "research" : "folder",
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
      research: isResearch ? parseResearchMeta(row.researchJson) : undefined,
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
  /**
   * Creates a research folder (RESEARCH_SURFACE_PRD §5). Unlike a plain folder this goes
   * to the SERVER, not the local mutation path: the node and its strategy extension row
   * must be written in one transaction with the outbox frame. The tree then updates when
   * that frame arrives — reactive via the syncer, never an optimistic patch.
   */
  createResearch(input: { parentId?: string | null; title: string; hypothesis?: string }): Promise<BrowserNodeRow>;
  createArtifact(input: UserArtifactCreateRequest): Promise<Artifact>;
  renameFolder(folderId: string, title: string): Promise<FolderNode>;
  /** Renames a research folder via its own endpoint; the tree updates off the frame. */
  renameResearch(nodeId: string, title: string): Promise<BrowserNodeRow>;
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
    async renameResearch(nodeId, title) {
      if (!client) {
        throw new Error("API client is required to rename a research folder");
      }
      return client.fetch<BrowserNodeRow>(`/api/research/${encodeURIComponent(nodeId)}`, {
        method: "PATCH",
        body: JSON.stringify({ title }),
      });
    },
    async createResearch(input) {
      if (!client) {
        throw new Error("API client is required to create a research folder");
      }
      return client.fetch<BrowserNodeRow>("/api/research", {
        method: "POST",
        body: JSON.stringify({
          parentId: input.parentId ?? null,
          title: input.title,
          hypothesis: input.hypothesis,
        }),
      });
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
