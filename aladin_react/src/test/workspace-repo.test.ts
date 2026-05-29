import { describe, expect, it, vi } from "vitest";
import { createWorkspaceRepo } from "@/repos/workspace/workspace-repo";
import type { BrowserNodeRepo } from "@/repos/workspace/browser-node-repo";
import type { NodeRepo } from "@/repos/workspace/node-repo";
import type { NodeRow } from "@/repos/local-repo-types";
import type { LocalSyncRepo } from "@/repos/sync/local-sync-repo";

function browserRepoStub(): BrowserNodeRepo {
  return {
    listNodes: vi.fn(),
    getNode: vi.fn(),
    upsertNode: vi.fn(),
    createNode: vi.fn(),
    renameNode: vi.fn(),
    deleteNode: vi.fn(),
  };
}

function syncRepoStub(): LocalSyncRepo {
  return {
    setSession: vi.fn(),
    drainOutbox: vi.fn(),
    refreshWorkspace: vi.fn().mockResolvedValue(undefined),
    pullNow: vi.fn().mockResolvedValue(0),
  };
}

function nodeRow(partial: Partial<NodeRow> & Pick<NodeRow, "id" | "kind">): NodeRow {
  return {
    parentId: null,
    position: 0,
    title: null,
    artifactType: null,
    content: null,
    sourceUrl: null,
    summary: null,
    metadataJson: null,
    updatedAt: 0,
    ...partial,
  };
}

describe("workspace repo (nodes model)", () => {
  it("pulls the change-feed delta before reading local nodes on a remote refresh", async () => {
    const nodes: NodeRepo = {
      listNodes: vi
        .fn()
        .mockResolvedValue([nodeRow({ id: "folder-1", kind: "folder", title: "Folder" })]),
      getNode: vi.fn(),
    };
    const localSync = syncRepoStub();

    const repo = createWorkspaceRepo(browserRepoStub(), nodes, undefined, localSync);
    const tree = await repo.getBrowserTree({ policy: "remote" });

    expect(localSync.pullNow).toHaveBeenCalledTimes(1);
    expect(nodes.listNodes).toHaveBeenCalled();
    expect(tree).toEqual([
      {
        id: "folder-1",
        parentId: null,
        kind: "folder",
        title: "Folder",
        artifactId: undefined,
        artifactPreview: undefined,
        children: [],
      },
    ]);
  });

  it("materializes a nested tree with an artifact preview from a single node list", async () => {
    const nodes: NodeRepo = {
      listNodes: vi.fn().mockResolvedValue([
        nodeRow({ id: "folder-1", kind: "folder", title: "Docs" }),
        nodeRow({
          id: "note-1",
          kind: "artifact",
          parentId: "folder-1",
          title: "Hello",
          artifactType: "note",
          updatedAt: 0,
        }),
      ]),
      getNode: vi.fn(),
    };

    const repo = createWorkspaceRepo(browserRepoStub(), nodes, undefined, syncRepoStub());
    const tree = await repo.getBrowserTree({ policy: "local-first" });

    expect(tree).toHaveLength(1);
    const folder = tree[0];
    expect(folder.id).toBe("folder-1");
    expect(folder.children).toHaveLength(1);
    const note = folder.children[0];
    expect(note.kind).toBe("artifact");
    expect(note.artifactId).toBe("note-1");
    expect(note.artifactPreview?.kind).toBe("note");
  });

  it("does not pull when the local node tree is already populated", async () => {
    const nodes: NodeRepo = {
      listNodes: vi
        .fn()
        .mockResolvedValue([nodeRow({ id: "folder-1", kind: "folder", title: "Folder" })]),
      getNode: vi.fn(),
    };
    const localSync = syncRepoStub();

    const repo = createWorkspaceRepo(browserRepoStub(), nodes, undefined, localSync);
    await repo.getBrowserTree({ policy: "local-first" });

    expect(localSync.pullNow).not.toHaveBeenCalled();
  });
});
