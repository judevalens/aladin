import type { ApiClient } from "@/shared/api/client";
import type { EntityHit, GraphPane, MentionRef } from "@/modules/graph/graph-pane-types";

export interface GraphPaneRepo {
  getPane(artifactId: string): Promise<GraphPane>;
  searchEntities(query: string): Promise<EntityHit[]>;
  createEntity(name: string, kind: string): Promise<EntityHit>;
  attachEntity(artifactId: string, entityId: string): Promise<void>;
  detachEntity(artifactId: string, entityId: string): Promise<void>;
  syncMentions(artifactId: string, mentions: MentionRef[]): Promise<void>;
  extractClaims(artifactId: string, text: string): Promise<{ claimsStored: number }>;
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
  };
}
