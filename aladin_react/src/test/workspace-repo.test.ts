import { describe, expect, it, vi } from "vitest";
import { createWorkspaceRepo } from "@/repos/workspace/workspace-repo";
import type { BrowserNodeRepo } from "@/repos/workspace/browser-node-repo";
import type { ArtifactRowRepo } from "@/repos/artifacts/artifact-row-repo";
import type { LocalSyncRepo } from "@/repos/sync/local-sync-repo";

describe("workspace repo", () => {
  it("refreshes through the Rust data layer before reading local rows", async () => {
    const browser: BrowserNodeRepo = {
      listNodes: vi.fn().mockResolvedValue([
        {
          id: "folder-1",
          parentId: null,
          title: "Folder",
          kind: "folder",
          artifactId: null,
          updatedAt: 1,
          syncStatus: "SYNCED",
          version: 1,
        },
      ]),
      getNode: vi.fn(),
      upsertNode: vi.fn(),
      createNode: vi.fn(),
      renameNode: vi.fn(),
      deleteNode: vi.fn(),
    };
    const artifacts: ArtifactRowRepo = {
      listAll: vi.fn().mockResolvedValue([]),
      getById: vi.fn(),
      getMany: vi.fn(),
      upsert: vi.fn(),
      create: vi.fn(),
      rename: vi.fn(),
      deleteById: vi.fn(),
    };
    const localSync: LocalSyncRepo = {
      setSession: vi.fn(),
      drainOutbox: vi.fn(),
      refreshWorkspace: vi.fn().mockResolvedValue(undefined),
    };

    const repo = createWorkspaceRepo(browser, artifacts, undefined, localSync);
    const tree = await repo.getBrowserTree({ policy: "remote" });

    expect(localSync.refreshWorkspace).toHaveBeenCalledTimes(1);
    expect(browser.listNodes).toHaveBeenCalled();
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
});
