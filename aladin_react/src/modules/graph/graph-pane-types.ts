// "On the graph" pane payload — mirrors GET /api/graph-pane (backend service.GraphPane).

export interface GraphEntity {
  id: string;
  name: string;
  kind: string;
  mentions: number;
  origin: string; // tag | mention
}

/** A typeahead hit from GET /api/entities/search. */
export interface EntityHit {
  id: string;
  name: string;
  kind: string;
  scope: string;
  trustTier: string;
  /** Other known surfaces (synonyms), excluding the canonical name — for picker display. */
  aliases: string[];
}

/** An entity linked to a page (GET /api/artifacts/{id}/entities): a tag or projected mention. */
export interface AttachedEntity {
  id: string;
  name: string;
  kind: string;
  origin: string; // tag | mention
  blockId?: string;
}

/** One projected @entity occurrence in a page (PUT …/entity-mentions). */
export interface MentionRef {
  entityId: string;
  blockId: string;
  surface: string;
  /** The block's plain text — rendered verbatim as "your note" on the entity surface. */
  snippet: string;
}

/** Reference target kind for the `#` picker. */
export type RefKind = "page" | "shard";

/** A typeahead hit from GET /api/refs/search (page | shard). */
export interface RefHit {
  kind: RefKind;
  id: string;
  label: string;
  detail?: string;
}

/** One projected `#` reference occurrence in a page (PUT …/refs). */
export interface ArtifactRef {
  kind: RefKind;
  targetId: string;
  blockId: string;
  surface: string;
}

/** Another artifact connected to the pane's subject. */
export interface GraphLinkedArtifact {
  id: string;
  title: string;
  kind: string; // artifact type: note | link | voice | file | app | page | shard
  relation: "referenced_by" | "references" | "shared_entity";
}

export interface GraphPane {
  entities: GraphEntity[];
  linkedArtifacts: GraphLinkedArtifact[];
}
