import type { ApiClient } from "@/shared/api/client";
import type {
  UserArtifact,
  UserArtifactCreateRequest,
  UserArtifactUpdateRequest,
  VoiceCaptureDraft,
} from "@/shared/api/models";

export interface ArtifactApi {
  getArtifact(artifactId: string): Promise<UserArtifact>;
  createArtifact(input: UserArtifactCreateRequest): Promise<UserArtifact>;
  renameArtifact(artifactId: string, title: string): Promise<UserArtifact>;
  uploadVoiceArtifact(draft: VoiceCaptureDraft): Promise<UserArtifact>;
}

export function createArtifactApi(client: ApiClient): ArtifactApi {
  return {
    getArtifact: (artifactId) =>
      client.fetch<UserArtifact>(`/api/artifacts/${artifactId}`),
    createArtifact: (input) =>
      client.fetch<UserArtifact>("/api/artifacts/", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    renameArtifact: (artifactId, title) =>
      client.fetch<UserArtifact>(`/api/artifacts/${artifactId}`, {
        method: "PATCH",
        body: JSON.stringify({ title } satisfies UserArtifactUpdateRequest),
      }),
    async uploadVoiceArtifact(draft) {
      if (!draft.audioBlob) {
        throw new Error("Record audio before saving.");
      }

      const filename = `${draft.title.trim().replace(/\s+/g, "-").toLowerCase() || "voice-note"}.webm`;
      const formData = new FormData();
      formData.append("type", "voice");
      formData.append("file", draft.audioBlob, filename);
      formData.append("title", draft.title.trim());
      if (draft.description.trim()) {
        formData.append("summary", draft.description.trim());
      }
      if (draft.folderId) {
        formData.append("folderId", draft.folderId);
      }

      return client.fetch<UserArtifact>("/api/artifacts/upload", {
        method: "POST",
        body: formData,
      });
    },
  };
}
