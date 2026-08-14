import type {
  Artifact,
  ArtifactKind,
  ArtifactProperty,
  UserArtifact,
  UserArtifactCreateRequest,
  VoiceCaptureDraft,
} from "@/shared/api/models";
import type { ArtifactRow } from "@/repos/local-repo-types";
import type { ArtifactRowRepo } from "@/repos/artifacts/artifact-row-repo";
import {
  artifactToRow,
  propertiesFromMetadataJson,
  rowToArtifact,
} from "@/repos/artifacts/artifact-mappers";
import { mergePropertyDefs, type PropertyDef } from "@/repos/artifacts/property-defs";
import type { ArtifactApi, FileUploadInput } from "@/shared/api/artifact-api";
import type { ApiClient } from "@/shared/api/client";

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

  const rawProperties = (record.metadata as { properties?: unknown } | null)?.properties;
  return {
    id: record.id,
    folderId: record.folderId,
    title,
    content: record.content,
    summary: record.summary,
    kind,
    updatedLabel: record.updatedAt,
    sourceUrl: record.sourceUrl,
    resourceUrl:
      kind === "voice" || kind === "file" ? artifactResourceUrl(client, record.id) : undefined,
    properties: Array.isArray(rawProperties) ? (rawProperties as Artifact["properties"]) : null,
  };
}

function fromArtifactRow(client: ApiClient, row: ArtifactRow): Artifact {
  return rowToArtifact(client, row);
}

export interface ArtifactRepo {
  getArtifact(
    artifactId: string,
    options?: { policy?: "local-first" | "remote" },
  ): Promise<Artifact>;
  createArtifact(input: UserArtifactCreateRequest): Promise<Artifact>;
  renameArtifact(artifactId: string, title: string): Promise<Artifact>;
  /**
   * Deletes an artifact. Server-authoritative: the Rust command calls the API first and only
   * then applies the returned seq to the local replica, so a failed delete leaves nothing
   * half-removed and the tree updates off the sync frame rather than a local guess.
   */
  deleteArtifact(artifactId: string): Promise<void>;
  uploadVoiceArtifact(draft: VoiceCaptureDraft): Promise<Artifact>;
  uploadFileArtifact(input: FileUploadInput): Promise<Artifact>;
  getResourceBlob(artifactId: string): Promise<Blob>;
  updateProperties(artifactId: string, properties: ArtifactProperty[]): Promise<void>;
  /** The reusable property-type set, derived from every cached artifact's properties. */
  listPropertyDefs(): Promise<PropertyDef[]>;
}

export function createArtifactRepo(
  api: ArtifactApi,
  client: ApiClient,
  artifacts: ArtifactRowRepo,
): ArtifactRepo {
  async function cache(artifact: Artifact, updatedAtMs = Date.now()) {
    await artifacts.upsert(artifactToRow(artifact, updatedAtMs));
    return artifact;
  }

  async function fetchAndCacheArtifact(artifactId: string) {
    const artifact = toArtifact(client, await api.getArtifact(artifactId));
    await cache(artifact);
    return artifact;
  }

  return {
    async getArtifact(artifactId, options) {
      if (options?.policy !== "remote") {
        const local = await artifacts.getById(artifactId);
        if (local) {
          return rowToArtifact(client, local);
        }
      }
      return fetchAndCacheArtifact(artifactId);
    },
    async createArtifact(input) {
      if (!input.type?.trim()) {
        throw new Error("Artifact type is required");
      }
      const row = await artifacts.create({
        id: crypto.randomUUID(),
        folderId: input.folderId ?? null,
        type: input.type,
        title: input.title ?? "",
        content: input.content ?? null,
        summary: input.summary ?? null,
        sourceUrl: input.sourceUrl ?? null,
        updatedAt: Date.now(),
        mutationId: crypto.randomUUID(),
      });
      return fromArtifactRow(client, row);
    },
    async deleteArtifact(artifactId) {
      await artifacts.deleteById({
        id: artifactId,
        updatedAt: Date.now(),
        mutationId: crypto.randomUUID(),
      });
    },

    async renameArtifact(artifactId, title) {
      const row = await artifacts.rename({
        id: artifactId,
        title,
        updatedAt: Date.now(),
        mutationId: crypto.randomUUID(),
      });
      return fromArtifactRow(client, row);
    },
    async uploadVoiceArtifact(draft) {
      const artifact = toArtifact(client, await api.uploadVoiceArtifact(draft));
      return cache(artifact);
    },
    async uploadFileArtifact(input) {
      const artifact = toArtifact(client, await api.uploadFileArtifact(input));
      return cache(artifact);
    },
    getResourceBlob(artifactId) {
      return api.getResourceBlob(artifactId);
    },
    updateProperties(artifactId, properties) {
      return artifacts.updateProperties(artifactId, properties);
    },
    async listPropertyDefs() {
      const rows = await artifacts.listAll();
      const defs: PropertyDef[] = [];
      for (const row of rows) {
        for (const p of propertiesFromMetadataJson(row.metadataJson) ?? []) {
          defs.push({
            key: p.key,
            type: p.type,
            // For tags, the used chip values ARE the known set (autocomplete source).
            options: p.type === "tags" ? p.values ?? [] : p.options,
          });
        }
      }
      return mergePropertyDefs(defs);
    },
  };
}
