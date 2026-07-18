import { describe, expect, it, vi } from "vitest";
import { AuthSessionService } from "@/services/auth/auth-session-service";
import { WorkspaceSyncService } from "@/services/workspace/workspace-sync-service";
import type { AuthRepo } from "@/repos/auth/auth-repo";
import type { ArtifactRepo } from "@/repos/artifacts/artifact-repo";
import type { WorkspaceRepo } from "@/repos/workspace/workspace-repo";
import type { DesktopSessionStore } from "@/shared/runtime/desktop-session-store";

describe("observable services", () => {
  it("updates auth session state and persists desktop bearer tokens on login", async () => {
    const repo: AuthRepo = {
      me: vi.fn(),
      login: vi.fn().mockResolvedValue({
        user: { id: "user-1", email: "admin@email.com" },
        token: "desktop-token",
        expiresAt: "2026-05-16T00:00:00Z",
      }),
      register: vi.fn(),
      logout: vi.fn(),
    };
    let savedToken: { token: string; expiresAt?: string | null } | null = null;
    const sessionStore: DesktopSessionStore = {
      load: () => (savedToken ? { ...savedToken } : null),
      save: (record) => {
        savedToken = record;
      },
      clear: () => {
        savedToken = null;
      },
      getToken: () => savedToken?.token ?? null,
    };

    const service = new AuthSessionService(repo, sessionStore);

    await service.login({ email: "admin@email.com", password: "password" });

    expect(repo.login).toHaveBeenCalledWith({
      email: "admin@email.com",
      password: "password",
    });
    expect(savedToken).toEqual({
      token: "desktop-token",
      expiresAt: "2026-05-16T00:00:00Z",
    });
    let sessionSnapshot = null;
    const sessionSubscription = service.session().subscribe((next) => {
      sessionSnapshot = next;
    });
    sessionSubscription.unsubscribe();
    expect(sessionSnapshot).toEqual({
      status: "authenticated",
      user: { id: "user-1", email: "admin@email.com" },
    });
  });

  it("broadcasts the fetched artifact to all subscribers via artifactById", async () => {
    const workspaceRepo: WorkspaceRepo = {
      getBrowserTree: vi.fn(),
      getLocalNodeTree: vi.fn(),
      createFolder: vi.fn(),
      renameFolder: vi.fn(),
      createArtifact: vi.fn(),
    };
    const artifactRepo: ArtifactRepo = {
      getArtifact: vi.fn().mockImplementation(async (artifactId: string) => ({
        id: artifactId,
        folderId: "folder-1",
        title: "Artifact one",
        content: "hello",
        kind: "note",
        updatedLabel: "just now",
      })),
      createArtifact: vi.fn(),
      renameArtifact: vi.fn(),
      uploadVoiceArtifact: vi.fn(),
      uploadFileArtifact: vi.fn(),
      getResourceBlob: vi.fn(),
      updateProperties: vi.fn(),
      listPropertyDefs: vi.fn(),
    };
    const service = new WorkspaceSyncService(workspaceRepo, artifactRepo);

    const emissions: unknown[] = [];
    const subscription = service.artifactById("artifact-1").subscribe((next) => {
      emissions.push(next);
    });

    await new Promise((resolve) => setTimeout(resolve, 0));
    subscription.unsubscribe();

    expect(artifactRepo.getArtifact).toHaveBeenCalledWith("artifact-1");
    expect(emissions).toEqual([
      {
        ok: true,
        value: {
          id: "artifact-1",
          folderId: "folder-1",
          title: "Artifact one",
          content: "hello",
          kind: "note",
          updatedLabel: "just now",
        },
      },
    ]);
  });
});
