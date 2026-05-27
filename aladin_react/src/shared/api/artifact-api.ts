import type { ApiClient } from "@/shared/api/client";
import type {
  UserArtifact,
  VoiceCaptureDraft,
} from "@/shared/api/models";

/**
 * The two direct-HTTP artifact endpoints the frontend still uses:
 *   - getArtifact: cache-miss fallback for the local-first read path.
 *   - uploadVoiceArtifact: multipart blob upload; can't easily route
 *     through Tauri yet, so it stays direct for now.
 *
 * createArtifact and renameArtifact used to live here but were removed
 * in M7.6 — every caller already goes through the workspace repo's
 * Tauri-routed path (db_create_artifact / db_rename_artifact), so the
 * direct fetch was dead code that risked re-introducing a dual-write.
 */
export interface ArtifactApi {
  getArtifact(artifactId: string): Promise<UserArtifact>;
  uploadVoiceArtifact(draft: VoiceCaptureDraft): Promise<UserArtifact>;
}

export function createArtifactApi(client: ApiClient): ArtifactApi {
  return {
    getArtifact: (artifactId) =>
      client.fetch<UserArtifact>(`/api/artifacts/${artifactId}`),
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
