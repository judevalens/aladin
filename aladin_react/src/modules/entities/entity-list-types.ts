// The Entities index (GET /api/entities) — browse the entity layer.

export type EntitySort = "attention" | "links" | "name";

/** Header status pill — "" is everything. */
export type EntityFilter = "" | "pending" | "unresolved";

/** One card on the index. */
export interface EntityListItem {
  id: string;
  name: string;
  kind: string;
  gist: string;
  trustTier: string;
  /** RFC3339 — rendered as "updated Nd ago" on the card. */
  updatedAt: string;
  links: number;
  sources: number;
  /** Merge proposals still awaiting your decision on this entity. */
  attention: number;
  aliases: string[];
}

/** The header band: the shape of the whole registry, not of the filtered result. */
export interface EntitySummary {
  total: number;
  /** Every unanswered merge proposal — the questions the judge is asking. */
  pendingDecisions: number;
  tiers: Record<string, number>;
}

export interface EntityListResult {
  entities: EntityListItem[];
  summary: EntitySummary;
}

export interface EntityListParams {
  query?: string;
  kind?: string;
  filter?: EntityFilter;
  sort?: EntitySort;
}

/** One decision on the inbox — a proposal to fold two entities together. */
export interface MergeQueueItem {
  mergeId: string;
  fromId: string;
  fromName: string;
  fromKind: string;
  intoId: string;
  intoName: string;
  intoKind: string;
  confidence: number;
  suggestion: "synonym" | "distinct" | "unsure";
  why: string;
}
