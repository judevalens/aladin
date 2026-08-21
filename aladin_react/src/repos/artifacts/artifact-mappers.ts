import type { ApiClient } from "@/shared/api/client";
import type { Artifact, ArtifactKind, ArtifactProperty } from "@/shared/api/models";
import type { ArtifactRow, NodeRow } from "@/repos/local-repo-types";

/** Serialize an artifact's typed properties into the metadata bag stored as metadata_json. */
function propertiesToMetadataJson(properties: Artifact["properties"]): string | null {
  if (!properties || properties.length === 0) return null;
  return JSON.stringify({ properties });
}

/** Pull the typed properties array out of a stored metadata_json bag (tolerant of junk). */
export function propertiesFromMetadataJson(metadataJson: string | null): ArtifactProperty[] | null {
  if (!metadataJson) return null;
  try {
    const parsed = JSON.parse(metadataJson) as { properties?: ArtifactProperty[] };
    return Array.isArray(parsed.properties) ? parsed.properties : null;
  } catch {
    return null;
  }
}

export function artifactKindFromString(value: string | null | undefined): ArtifactKind {
  switch ((value ?? "").toLowerCase()) {
    case "page":
    case "note":
      return "note";
    case "link":
      return "link";
    case "voice":
      return "voice";
    case "file":
      return "file";
    case "app":
      return "app";
    case "board":
      return "board";
    default:
      return "note";
  }
}

export function artifactToRow(artifact: Artifact, updatedAtMs: number): ArtifactRow {
  return {
    id: artifact.id,
    folderId: artifact.folderId ?? null,
    title: artifact.title,
    kind: artifact.kind,
    content: artifact.content ?? null,
    sourceUrl: artifact.sourceUrl ?? null,
    resourceUrl: artifact.resourceUrl ?? null,
    summary: artifact.summary ?? null,
    metadataJson: propertiesToMetadataJson(artifact.properties),
    updatedAt: updatedAtMs,
    syncStatus: "SYNCED",
    version: 0,
  };
}

/**
 * Adapts a unified `nodes` row (kind = "artifact") to the legacy ArtifactRow
 * shape so it can flow through rowToArtifact for the work pane. The node id IS
 * the artifact id; summary lives in its own column, folded back into metadata
 * for rowToArtifact's summary parsing.
 */
export function nodeRowToArtifactRow(node: NodeRow): ArtifactRow {
  return {
    id: node.id,
    folderId: node.parentId,
    title: node.title ?? "",
    kind: node.artifactType ?? "page",
    content: node.content,
    sourceUrl: node.sourceUrl,
    resourceUrl: null,
    // summary + metadata are distinct columns now — pass both through untouched.
    summary: node.summary,
    metadataJson: node.metadataJson,
    updatedAt: node.updatedAt,
    syncStatus: "SYNCED",
    version: 0,
  };
}

export function rowToArtifact(client: ApiClient, row: ArtifactRow): Artifact {
  const kind = artifactKindFromString(row.kind);
  return {
    id: row.id,
    folderId: row.folderId,
    title: row.title,
    content: row.content ?? "",
    summary: row.summary,
    kind,
    updatedLabel: new Date(row.updatedAt).toISOString(),
    sourceUrl: row.sourceUrl,
    resourceUrl:
      kind === "voice" || kind === "file"
        ? row.resourceUrl ?? client.resolveUrl(`/api/artifacts/${row.id}/resource`)
        : null,
    properties: propertiesFromMetadataJson(row.metadataJson),
  };
}
