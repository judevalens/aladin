// "On the graph" pane payload — mirrors GET /api/graph-pane (backend service.GraphPane).

export interface GraphThesis {
  id: string;
  text: string;
  polarity: string;
  trustTier: string;
  updatedAt: string;
}

export interface GraphClaim {
  id: string;
  text: string;
  grounded: boolean;
  sources: number;
}

export interface GraphCite {
  id: string;
  title: string;
  sourceUrl: string;
  provider: string;
  createdAt: string;
}

export interface GraphEntity {
  id: string;
  name: string;
  kind: string;
  mentions: number;
  origin: string; // tag | mention | claim
}

/** A typeahead hit from GET /api/entities/search. */
export interface EntityHit {
  id: string;
  name: string;
  kind: string;
  scope: string;
  trustTier: string;
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
}

/** Reference target kind for the `#` picker. */
export type RefKind = "claim" | "page" | "shard";

/** A typeahead hit from GET /api/refs/search (claim | page | shard). */
export interface RefHit {
  kind: RefKind;
  id: string;
  label: string;
  detail?: string; // claim polarity; empty for artifacts
}

/** One projected `#` reference occurrence in a page (PUT …/refs). */
export interface ArtifactRef {
  kind: RefKind;
  targetId: string;
  blockId: string;
  surface: string;
}

/** A page claim with its discovered support/contradiction (POST …/ingest, Y3). */
export interface ClaimConnection {
  claimId: string;
  text: string;
  polarity: string;
  support: number;
  contradict: number;
}

/** The Y3 "Connect" payload. */
export interface ConnectResult {
  claimsStored: number;
  connections: ClaimConnection[];
}

export interface GraphPane {
  thesis: GraphThesis | null;
  claims: GraphClaim[];
  cites: GraphCite[];
  entities: GraphEntity[];
}
