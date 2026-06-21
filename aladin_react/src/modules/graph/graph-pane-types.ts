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

/** One projected @entity occurrence in a page (PUT …/entity-mentions). */
export interface MentionRef {
  entityId: string;
  blockId: string;
  surface: string;
}

export interface GraphPane {
  thesis: GraphThesis | null;
  claims: GraphClaim[];
  cites: GraphCite[];
  entities: GraphEntity[];
}
