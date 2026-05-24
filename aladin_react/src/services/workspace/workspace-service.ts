import type { Observable } from "rxjs";
import type { ArtifactRepo } from "@/repos/artifacts/artifact-repo";
import type { WorkspaceRepo } from "@/repos/workspace/workspace-repo";
import type { Result } from "@/shared/flow/result";
import type {
  Artifact,
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

  async renameFolder(folderId: string, title: string) {
    return this.workspaceRepo.renameFolder(folderId, title);
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
}
