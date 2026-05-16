import type { ApiClient } from "@/shared/api/client";
import type {
  Artifact,
  ArtifactKind,
  UserArtifact,
  UserArtifactCreateRequest,
  UserArtifactUpdateRequest,
  VoiceCaptureDraft,
} from "@/shared/api/models";

function artifactKind(type?: string | null): ArtifactKind {
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

function metadataString(metadata: Record<string, unknown>, key: string) {
  const value = metadata[key];
  return typeof value === "string" ? value : undefined;
}

function artifactResourceUrl(client: ApiClient, id: string) {
  return client.resolveUrl(`/api/artifacts/${id}/resource`);
}

function toArtifact(client: ApiClient, record: UserArtifact): Artifact {
  const kind = artifactKind(record.type);
  const title =
    record.title ||
    metadataString(record.metadata, "originalFilename") ||
    metadataString(record.metadata, "storageKey") ||
    "Untitled artifact";

  return {
    id: record.id,
    folderId: record.folderId,
    title,
    content: record.content,
    summary: record.summary,
    kind,
    updatedLabel: record.updatedAt,
    sourceUrl: record.sourceUrl,
    resourceUrl: kind === "voice" || kind === "file" ? artifactResourceUrl(client, record.id) : undefined,
  };
}

export interface ArtifactRepo {
  getArtifact(artifactId: string): Promise<Artifact>;
  createArtifact(input: UserArtifactCreateRequest): Promise<Artifact>;
  renameArtifact(artifactId: string, title: string): Promise<Artifact>;
  uploadVoiceArtifact(draft: VoiceCaptureDraft): Promise<Artifact>;
}

export function createArtifactRepo(client: ApiClient): ArtifactRepo {
  return {
    async getArtifact(artifactId) {
      return toArtifact(client, await client.fetch<UserArtifact>(`/api/artifacts/${artifactId}`));
    },
    async createArtifact(input) {
      return toArtifact(
        client,
        await client.fetch<UserArtifact>("/api/artifacts/", {
          method: "POST",
          body: JSON.stringify(input),
        }),
      );
    },
    async renameArtifact(artifactId, title) {
      return toArtifact(
        client,
        await client.fetch<UserArtifact>(`/api/artifacts/${artifactId}`, {
          method: "PATCH",
          body: JSON.stringify({ title } satisfies UserArtifactUpdateRequest),
        }),
      );
    },
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

      return toArtifact(
        client,
        await client.fetch<UserArtifact>("/api/artifacts/upload", {
          method: "POST",
          body: formData,
        }),
      );
    },
  };
}
