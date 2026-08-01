import type { Observable } from "rxjs";
import type { ArtifactRepo } from "@/repos/artifacts/artifact-repo";
import type { FileUploadInput } from "@/shared/api/artifact-api";
import type { WorkspaceRepo } from "@/repos/workspace/workspace-repo";
import type { Result } from "@/shared/flow/result";
import type {
  Artifact,
  ArtifactProperty,
  BrowserTreeNode,
  UserArtifactCreateRequest,
  VoiceCaptureDraft,
} from "@/shared/api/models";
import type { WorkspaceSyncService } from "@/services/workspace/workspace-sync-service";

export class WorkspaceService {
  constructor(
    private readonly workspaceRepo: WorkspaceRepo,
    private readonly artifactRepo: ArtifactRepo,
    private readonly sync: WorkspaceSyncService,
  ) {}

  tree(): Observable<Result<BrowserTreeNode[]>> {
    return this.sync.tree();
  }

  artifactById(artifactId: string): Observable<Result<Artifact>> {
    return this.sync.artifactById(artifactId);
  }

  async createFolder(input: { parentId?: string | null; title: string }) {
    return this.workspaceRepo.createFolder(input);
  }

  async createResearch(input: { parentId?: string | null; title: string; hypothesis?: string }) {
    return this.workspaceRepo.createResearch(input);
  }

  async renameFolder(folderId: string, title: string) {
    return this.workspaceRepo.renameFolder(folderId, title);
  }

  async renameResearch(nodeId: string, title: string) {
    return this.workspaceRepo.renameResearch(nodeId, title);
  }

  refreshTree() {
    return this.sync.refreshTree();
  }

  async createArtifact(input: UserArtifactCreateRequest) {
    return this.workspaceRepo.createArtifact(input);
  }

  async renameArtifact(artifactId: string, title: string) {
    return this.artifactRepo.renameArtifact(artifactId, title);
  }

  async uploadVoiceArtifact(draft: VoiceCaptureDraft) {
    const artifact = await this.artifactRepo.uploadVoiceArtifact(draft);
    this.sync.publishArtifact(artifact);
    return artifact;
  }

  async uploadFileArtifact(input: FileUploadInput) {
    const artifact = await this.artifactRepo.uploadFileArtifact(input);
    this.sync.publishArtifact(artifact);
    this.sync.refreshTree();
    return artifact;
  }

  /**
   * Set an artifact's typed properties. The Rust command applies the write and
   * emits a NodeUpserted frame, so the reactive `artifactById` stream updates on
   * its own — no manual publish needed here.
   */
  async updateArtifactProperties(artifactId: string, properties: ArtifactProperty[]) {
    await this.artifactRepo.updateProperties(artifactId, properties);
  }

  /** The reusable property-type set (presets are merged in by the caller/UI). */
  listPropertyDefs() {
    return this.artifactRepo.listPropertyDefs();
  }
}
