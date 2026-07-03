import type { ApiClient } from "@/shared/api/client";
import type {
  ArtifactRef,
  AttachedEntity,
  ConnectResult,
  EntityHit,
  GraphPane,
  MentionRef,
  RefHit,
} from "@/modules/graph/graph-pane-types";

export interface GraphPaneRepo {
  getPane(artifactId: string): Promise<GraphPane>;
  searchEntities(query: string): Promise<EntityHit[]>;
  createEntity(name: string, kind: string): Promise<EntityHit>;
  listEntities(artifactId: string): Promise<AttachedEntity[]>;
  attachEntity(artifactId: string, entityId: string): Promise<void>;
  detachEntity(artifactId: string, entityId: string): Promise<void>;
  syncMentions(artifactId: string, mentions: MentionRef[]): Promise<void>;
  extractClaims(artifactId: string, text: string): Promise<{ claimsStored: number }>;
  // `#` cross-references (Y2) + the Connect trigger (Y3).
  searchRefs(query: string): Promise<RefHit[]>;
  syncRefs(artifactId: string, refs: ArtifactRef[]): Promise<void>;
  connect(artifactId: string, text: string): Promise<ConnectResult>;
}

export function createGraphPaneRepo(client: ApiClient): GraphPaneRepo {
  return {
    getPane: (artifactId) =>
      client.fetch<GraphPane>(`/api/graph-pane?artifact=${encodeURIComponent(artifactId)}`),

    searchEntities: (query) =>
      client.fetch<EntityHit[]>(`/api/entities/search?q=${encodeURIComponent(query)}`),

    createEntity: (name, kind) =>
      client.fetch<EntityHit>(`/api/entities`, {
        method: "POST",
        body: JSON.stringify({ name, kind }),
      }),

    listEntities: (artifactId) =>
      client.fetch<AttachedEntity[]>(`/api/artifacts/${encodeURIComponent(artifactId)}/entities`),

    attachEntity: (artifactId, entityId) =>
      client.fetch<void>(`/api/artifacts/${encodeURIComponent(artifactId)}/entities`, {
        method: "POST",
        body: JSON.stringify({ entityId }),
      }),

    detachEntity: (artifactId, entityId) =>
      client.fetch<void>(
        `/api/artifacts/${encodeURIComponent(artifactId)}/entities/${encodeURIComponent(entityId)}`,
        { method: "DELETE" },
      ),

    syncMentions: (artifactId, mentions) =>
      client.fetch<void>(`/api/artifacts/${encodeURIComponent(artifactId)}/entity-mentions`, {
        method: "PUT",
        body: JSON.stringify({ mentions }),
      }),

    extractClaims: (artifactId, text) =>
      client.fetch<{ claimsStored: number }>(
        `/api/artifacts/${encodeURIComponent(artifactId)}/extract-claims`,
        { method: "POST", body: JSON.stringify({ text }) },
      ),

    searchRefs: (query) =>
      client.fetch<RefHit[]>(`/api/refs/search?q=${encodeURIComponent(query)}`),

    syncRefs: (artifactId, refs) =>
      client.fetch<void>(`/api/artifacts/${encodeURIComponent(artifactId)}/refs`, {
        method: "PUT",
        body: JSON.stringify({ refs }),
      }),

    connect: (artifactId, text) =>
      client.fetch<ConnectResult>(`/api/artifacts/${encodeURIComponent(artifactId)}/ingest`, {
        method: "POST",
        body: JSON.stringify({ text }),
      }),
  };
}
