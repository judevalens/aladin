import type {
  BrowserTreeNode,
  FolderCreateRequest,
  FolderNode,
  UserArtifactCreateRequest,
  Artifact,
} from "@/shared/api/models";
import type { ArtifactRow, BrowserNodeRow } from "@/repos/local-repo-types";
import type { BrowserNodeRepo } from "@/repos/workspace/browser-node-repo";
import type { ArtifactRowRepo } from "@/repos/artifacts/artifact-row-repo";
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

function toLocalBrowserTree(
  browserNodes: BrowserNodeRow[],
  artifacts: ArtifactRow[],
): BrowserTreeNode[] {
  const folderNodes = new Map<string, BrowserTreeNode>();
  const artifactRowsById = new Map(artifacts.map((row) => [row.id, row]));
  const nodesById = new Map<string, BrowserTreeNode>();
  browserNodes.forEach((row) => {
    const node: BrowserTreeNode = {
      id: row.id,
      parentId: row.parentId,
      kind: row.kind.toLowerCase() === "artifact" ? "artifact" : "folder",
      title: row.title,
      artifactId: row.artifactId ?? undefined,
      artifactPreview:
        row.kind.toLowerCase() === "artifact"
          ? (() => {
              const artifact = artifactRowsById.get(row.artifactId ?? row.id);
              return {
                id: row.artifactId ?? row.id,
                title: row.title,
                kind: artifactKindFromString(artifact?.kind),
                updatedLabel: new Date(
                  artifact?.updatedAt ?? row.updatedAt,
                ).toISOString(),
              };
            })()
          : undefined,
      children: [],
    };
    nodesById.set(row.id, node);
    if (node.kind === "folder") {
      folderNodes.set(row.id, node);
    }
  });

  const roots: BrowserTreeNode[] = [];
  browserNodes.forEach((row) => {
    const node = nodesById.get(row.id);
    if (!node) return;
    const parentId = row.parentId;
    if (parentId && folderNodes.has(parentId)) {
      folderNodes.get(parentId)!.children.push(node);
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
  createFolder(input: FolderCreateRequest): Promise<FolderNode>;
  createArtifact(input: UserArtifactCreateRequest): Promise<Artifact>;
  renameFolder(folderId: string, title: string): Promise<FolderNode>;
}

export function createWorkspaceRepo(
  browser: BrowserNodeRepo,
  artifacts: ArtifactRowRepo,
  client?: ApiClient,
  localSync?: LocalSyncRepo,
): WorkspaceRepo {
  function createMutationId() {
    return crypto.randomUUID();
  }

  async function getLocalTree() {
    const [browserNodes, artifactRows] = await Promise.all([
      browser.listNodes(),
      artifacts.listAll(),
    ]);
    return toLocalBrowserTree(browserNodes, artifactRows);
  }

  async function fetchAndSyncRemoteTree() {
    if (!localSync) {
      throw new Error("Local sync client is required for workspace refresh");
    }
    await localSync.refreshWorkspace();
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
