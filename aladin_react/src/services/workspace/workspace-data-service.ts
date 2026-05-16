import { BehaviorSubject, type Observable } from "rxjs";
import type { ArtifactRepo } from "@/repos/artifacts/artifact-repo";
import type { WorkspaceRepo } from "@/repos/workspace/workspace-repo";
import type { Artifact, BrowserTreeNode } from "@/shared/api/models";
import {
  createErrorLoadable,
  createIdleLoadable,
  createLoadingLoadable,
  createReadyLoadable,
  type Loadable,
} from "@/shared/flow/loadable";

export class WorkspaceDataService {
  private readonly treeSubject = new BehaviorSubject<Loadable<BrowserTreeNode[]>>(
    createIdleLoadable([]),
  );
  private readonly artifactCacheSubject = new BehaviorSubject<Record<string, Artifact>>({});
  private readonly artifactInFlight = new Map<string, Promise<void>>();
  private treeLoadPromise: Promise<void> | null = null;

  constructor(
    private readonly workspaceRepo: WorkspaceRepo,
    private readonly artifactRepo: ArtifactRepo,
  ) {}

  observeTree(): Observable<Loadable<BrowserTreeNode[]>> {
    return this.treeSubject.asObservable();
  }

  getTreeSnapshot(): Loadable<BrowserTreeNode[]> {
    return this.treeSubject.getValue();
  }

  observeArtifacts(): Observable<Record<string, Artifact>> {
    return this.artifactCacheSubject.asObservable();
  }

  getArtifactsSnapshot(): Record<string, Artifact> {
    return this.artifactCacheSubject.getValue();
  }

  async ensureTreeLoaded() {
    const snapshot = this.getTreeSnapshot();
    if (snapshot.status === "ready" || snapshot.status === "loading") {
      return this.treeLoadPromise ?? Promise.resolve();
    }
    return this.refreshTree();
  }

  async refreshTree() {
    if (!this.treeLoadPromise) {
      const previous = this.getTreeSnapshot().data;
      this.treeSubject.next(createLoadingLoadable(previous));
      this.treeLoadPromise = this.workspaceRepo
        .getBrowserTree()
        .then((tree) => {
          this.treeSubject.next(createReadyLoadable(tree));
        })
        .catch((error) => {
          this.treeSubject.next(
            createErrorLoadable(
              previous,
              error instanceof Error ? error.message : "Failed to load browser tree.",
            ),
          );
          throw error;
        })
        .finally(() => {
          this.treeLoadPromise = null;
        });
    }
    return this.treeLoadPromise;
  }

  async createFolder(input: { parentId?: string | null; title: string }) {
    const folder = await this.workspaceRepo.createFolder(input);
    await this.refreshTree();
    return folder;
  }

  async renameFolder(folderId: string, title: string) {
    const folder = await this.workspaceRepo.renameFolder(folderId, title);
    await this.refreshTree();
    return folder;
  }

  async createArtifact(input: Parameters<ArtifactRepo["createArtifact"]>[0]) {
    const artifact = await this.artifactRepo.createArtifact(input);
    this.patchArtifact(artifact);
    await this.refreshTree();
    return artifact;
  }

  async renameArtifact(artifactId: string, title: string) {
    const artifact = await this.artifactRepo.renameArtifact(artifactId, title);
    this.patchArtifact(artifact);
    await this.refreshTree();
    return artifact;
  }

  async ensureArtifactsLoaded(artifactIds: string[]) {
    await Promise.all(artifactIds.map((artifactId) => this.ensureArtifactLoaded(artifactId)));
  }

  async ensureArtifactLoaded(artifactId: string) {
    const cached = this.getArtifactsSnapshot()[artifactId];
    if (cached) {
      return;
    }
    const existing = this.artifactInFlight.get(artifactId);
    if (existing) {
      return existing;
    }
    const request = this.artifactRepo
      .getArtifact(artifactId)
      .then((artifact) => {
        this.patchArtifact(artifact);
      })
      .finally(() => {
        this.artifactInFlight.delete(artifactId);
      });
    this.artifactInFlight.set(artifactId, request);
    return request;
  }

  async uploadVoiceArtifact(draft: Parameters<ArtifactRepo["uploadVoiceArtifact"]>[0]) {
    const artifact = await this.artifactRepo.uploadVoiceArtifact(draft);
    this.patchArtifact(artifact);
    await this.refreshTree();
    return artifact;
  }

  private patchArtifact(artifact: Artifact) {
    this.artifactCacheSubject.next({
      ...this.getArtifactsSnapshot(),
      [artifact.id]: artifact,
    });
  }
}
