import { invoke } from "@tauri-apps/api/core";
import type { ApiClient } from "@/shared/api/client";
import type {
  ArtifactRef,
  AttachedEntity,
  EntityHit,
  GraphPane,
  MentionRef,
  RefHit,
} from "@/modules/graph/graph-pane-types";
import type { EntityContextPayload } from "@/modules/entities/entity-context-types";
import type {
  EntityListParams,
  EntityListResult,
  MergeQueueItem,
} from "@/modules/entities/entity-list-types";

export interface GraphPaneRepo {
  getPane(artifactId: string): Promise<GraphPane>;
  searchEntities(query: string): Promise<EntityHit[]>;
  createEntity(name: string, kind: string): Promise<EntityHit>;
  listEntities(artifactId: string): Promise<AttachedEntity[]>;
  attachEntity(artifactId: string, entityId: string): Promise<void>;
  detachEntity(artifactId: string, entityId: string): Promise<void>;
  syncMentions(artifactId: string, mentions: MentionRef[]): Promise<void>;
  // `#` cross-references (Y2).
  searchRefs(query: string): Promise<RefHit[]>;
  syncRefs(artifactId: string, refs: ArtifactRef[]): Promise<void>;
  // The Entities index: browse/search the whole registry. Distinct from searchEntities
  // (a typeahead — empty query returns nothing) and from listEntities (artifact-scoped).
  listAllEntities(params: EntityListParams): Promise<EntityListResult>;
  // The inbox: all pending merge decisions across the layer.
  mergeQueue(): Promise<MergeQueueItem[]>;
  // Entity Context surface: one entity's identity + typed edges + verbatim material.
  getEntityContext(entityId: string): Promise<EntityContextPayload>;
  // Decide one of the judge's merge proposals.
  acceptEntityMerge(entityId: string, mergeId: string): Promise<void>;
  rejectEntityMerge(entityId: string, mergeId: string): Promise<void>;
  // Draw a connection: a typed, provenance-carrying edge (origin 'you').
  drawEntityEdge(entityId: string, edge: { rel: string; toId: string; why: string }): Promise<void>;
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

    // H2 — artifact<->entity links go through RUST, not the webview's REST client. The desktop
    // webview is cross-origin to the API, and syncMentions is a PUT: that exact route once failed
    // CORS preflight. Proxying through Rust removes the webview from the equation entirely.
    // These writes need no local apply — the SERVER emits a node frame for the artifact, so the
    // graph pane refreshes off the resulting DataEvent (reactive-via-syncer, no invalidation).
    listEntities: (artifactId) =>
      invoke<AttachedEntity[]>("db_list_artifact_entities", { artifactId }).then((r) => r ?? []),

    attachEntity: (artifactId, entityId) =>
      invoke<void>("db_attach_artifact_entity", { artifactId, entityId }),

    detachEntity: (artifactId, entityId) =>
      invoke<void>("db_detach_artifact_entity", { artifactId, entityId }),

    syncMentions: (artifactId, mentions) =>
      invoke<void>("db_sync_artifact_mentions", { artifactId, mentions }),

    searchRefs: (query) =>
      client.fetch<RefHit[]>(`/api/refs/search?q=${encodeURIComponent(query)}`),

    syncRefs: (artifactId, refs) =>
      client.fetch<void>(`/api/artifacts/${encodeURIComponent(artifactId)}/refs`, {
        method: "PUT",
        body: JSON.stringify({ refs }),
      }),

    listAllEntities: (params) => {
      const q = new URLSearchParams();
      if (params.query?.trim()) q.set("q", params.query.trim());
      if (params.kind) q.set("kind", params.kind);
      if (params.filter) q.set("filter", params.filter);
      if (params.sort) q.set("sort", params.sort);
      const qs = q.toString();
      return client.fetch<EntityListResult>(`/api/entities${qs ? `?${qs}` : ""}`);
    },

    mergeQueue: () => client.fetch<MergeQueueItem[]>(`/api/entities/merge-queue`),

    getEntityContext: (entityId) =>
      client.fetch<EntityContextPayload>(
        `/api/entities/${encodeURIComponent(entityId)}/context`,
      ),

    acceptEntityMerge: (entityId, mergeId) =>
      client.fetch<void>(
        `/api/entities/${encodeURIComponent(entityId)}/merges/${encodeURIComponent(mergeId)}/accept`,
        { method: "POST" },
      ),

    rejectEntityMerge: (entityId, mergeId) =>
      client.fetch<void>(
        `/api/entities/${encodeURIComponent(entityId)}/merges/${encodeURIComponent(mergeId)}/reject`,
        { method: "POST" },
      ),

    drawEntityEdge: (entityId, edge) =>
      client.fetch<void>(`/api/entities/${encodeURIComponent(entityId)}/edges`, {
        method: "POST",
        body: JSON.stringify(edge),
      }),
  };
}
