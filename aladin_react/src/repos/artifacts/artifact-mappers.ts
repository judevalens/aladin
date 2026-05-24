import type { ApiClient } from "@/shared/api/client";
import type { Artifact, ArtifactKind } from "@/shared/api/models";
import type { ArtifactRow } from "@/repos/local-repo-types";

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
    metadataJson: artifact.summary ? JSON.stringify({ summary: artifact.summary }) : null,
    updatedAt: updatedAtMs,
    syncStatus: "SYNCED",
    version: 0,
  };
}

export function rowToArtifact(client: ApiClient, row: ArtifactRow): Artifact {
  const kind = artifactKindFromString(row.kind);
  let summary: string | null = null;
  if (row.metadataJson) {
    try {
      const parsed = JSON.parse(row.metadataJson) as { summary?: string };
      summary = parsed.summary ?? null;
    } catch {
      summary = null;
    }
  }
  return {
    id: row.id,
    folderId: row.folderId,
    title: row.title,
    content: row.content ?? "",
    summary,
    kind,
    updatedLabel: new Date(row.updatedAt).toISOString(),
    sourceUrl: row.sourceUrl,
    resourceUrl:
      kind === "voice" || kind === "file"
        ? row.resourceUrl ?? client.resolveUrl(`/api/artifacts/${row.id}/resource`)
        : null,
  };
}
