import type { ApiClient } from "@/shared/api/client";
import type {
  BrowserTreeNode,
  BrowserTreeRecord,
  FolderCreateRequest,
  FolderNode,
  FolderRecord,
  FolderUpdateRequest,
} from "@/shared/api/models";

function toFolderNode(record: FolderRecord): FolderNode {
  return {
    id: record.id,
    parentId: record.parentId,
    title: record.title,
  };
}

function artifactKind(type?: string | null) {
  switch ((type ?? "").toLowerCase()) {
    case "page":
    case "note":
      return "note";
    case "link":
      return "link";
    case "voice":
      return "voice";
    case "file":
      return "file";
    default:
      return "note";
  }
}

function toBrowserTreeNode(record: BrowserTreeRecord): BrowserTreeNode {
  const kind = record.kind.toLowerCase() === "folder" ? "folder" : "artifact";
  return {
    id: record.id,
    parentId: record.parentId,
    kind,
    title: record.title,
    artifactId: record.artifactId,
    artifactPreview:
      kind === "artifact" && record.artifactId
        ? {
            id: record.artifactId,
            title: record.title,
            kind: artifactKind(record.artifactType),
            updatedLabel: record.updatedAt,
          }
        : undefined,
    children: record.children.map(toBrowserTreeNode),
  };
}

export interface WorkspaceRepo {
  getBrowserTree(): Promise<BrowserTreeNode[]>;
  createFolder(input: FolderCreateRequest): Promise<FolderNode>;
  renameFolder(folderId: string, title: string): Promise<FolderNode>;
}

export function createWorkspaceRepo(client: ApiClient): WorkspaceRepo {
  return {
    async getBrowserTree() {
      const tree = await client.fetch<BrowserTreeRecord[]>("/api/browser/tree");
      return tree.map(toBrowserTreeNode);
    },
    async createFolder(input) {
      return toFolderNode(
        await client.fetch<FolderRecord>("/api/folders/", {
          method: "POST",
          body: JSON.stringify(input),
        }),
      );
    },
    async renameFolder(folderId, title) {
      return toFolderNode(
        await client.fetch<FolderRecord>(`/api/folders/${folderId}`, {
          method: "PATCH",
          body: JSON.stringify({ title } satisfies FolderUpdateRequest),
        }),
      );
    },
  };
}
