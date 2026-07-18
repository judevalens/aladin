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
export interface FileUploadInput {
  file: File;
  folderId?: string | null;
  title?: string;
  summary?: string;
}

export interface ArtifactApi {
  getArtifact(artifactId: string): Promise<UserArtifact>;
  uploadVoiceArtifact(draft: VoiceCaptureDraft): Promise<UserArtifact>;
  uploadFileArtifact(input: FileUploadInput): Promise<UserArtifact>;
  /** Authenticated fetch of an artifact's binary resource (audio/file blob). */
  getResourceBlob(artifactId: string): Promise<Blob>;
}

/** Maps a MediaRecorder blob MIME type to the matching file extension. */
function audioExtension(mimeType: string): string {
  const type = mimeType.toLowerCase();
  // .m4a resolves to audio/mp4 (audio-only), the exact content-type WKWebView wants.
  if (type.includes("mp4") || type.includes("m4a") || type.includes("aac")) return "m4a";
  if (type.includes("ogg")) return "ogg";
  if (type.includes("wav")) return "wav";
  return "webm";
}

export function createArtifactApi(client: ApiClient): ArtifactApi {
  return {
    getArtifact: (artifactId) =>
      client.fetch<UserArtifact>(`/api/artifacts/${artifactId}`),
    getResourceBlob: (artifactId) =>
      client.fetchBlob(`/api/artifacts/${artifactId}/resource`),
    async uploadVoiceArtifact(draft) {
      if (!draft.audioBlob) {
        throw new Error("Record audio before saving.");
      }

      // Extension must match the recorded container so the backend derives the
      // right content-type (it keys off the file extension), which in turn drives
      // whether the player can decode it (WKWebView needs mp4, not webm).
      const ext = audioExtension(draft.audioBlob.type);
      const base = draft.title.trim().replace(/\s+/g, "-").toLowerCase() || "voice-note";
      const formData = new FormData();
      formData.append("type", "voice");
      formData.append("file", draft.audioBlob, `${base}.${ext}`);
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
    async uploadFileArtifact({ file, folderId, title, summary }) {
      const formData = new FormData();
      formData.append("type", "file");
      formData.append("file", file, file.name);
      formData.append("title", title?.trim() || file.name);
      if (summary?.trim()) {
        formData.append("summary", summary.trim());
      }
      if (folderId) {
        formData.append("folderId", folderId);
      }

      return client.fetch<UserArtifact>("/api/artifacts/upload", {
        method: "POST",
        body: formData,
      });
    },
  };
}
