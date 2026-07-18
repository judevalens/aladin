// Entity Context surface — data model per design/ENTITY_CONTEXT_PRD.md §3.
// Phase A (spike) renders mock data; these types are the contract the real
// graph read-path will eventually satisfy.

// Entity kinds are open: the surface renders `kind` as a plain label, and the backend's
// vocabulary (person | org | concept | location | other) is its own concern.
export type EntityKind = string;

export type Platform = "x" | "hn" | "reddit" | "rss" | "filing" | "paper" | "bluesky";

/** The focused node. */
export interface Entity {
  /** Absent in the design mock; present on real reads. */
  id?: string;
  name: string;
  kind: EntityKind;
  /** May be "" — the surface then renders no gist paragraph. */
  gist: string;
  /** Sources monitoring this entity; empty → chip row hidden. */
  watchers: Platform[];
  /** Provenance line, e.g. "tracked 3 weeks · 14 pieces of context". */
  since: string;
  /** An entity the @ picker minted that background resolution hasn't settled yet. */
  placeholder?: boolean;
}

/** Who asserted an edge — drives the YOURS / FOUND tag. */
export type EdgeOrigin = "you" | "watcher";

/** A typed, directional relationship to another entity. */
export interface Edge {
  /** Relation key — one of REL's keys; unknown keys fall back to a neutral glyph/tone. */
  rel: string;
  /** Target entity id — drives navigation. Absent in the design mock. */
  toId?: string;
  /** Name of the target entity. */
  to: string;
  /** Target's kind. */
  kind: string;
  /** The user's / watcher's reasoning for the edge — the substance. */
  why: string;
  origin: EdgeOrigin;
}

export type ContextType = "quote" | "note" | "question";

/** Raw accreted material — body is verbatim, never edited. */
export interface ContextItem {
  type: ContextType;
  body: string;
  /** Attribution: @handle, publication, or "me". */
  who: string;
  /** null → platform chip hidden. */
  platform: Platform | null;
  /** Relative time. */
  time: string;
  /** For notes: the page the block came from. */
  sourceId?: string;
}

/** The judge's read on a pair. "unsure" = it abstained or hasn't run — never a guess. */
export type MergeSuggestion = "synonym" | "distinct" | "unsure";

/**
 * One unanswered question about this entity's identity: is this other entity the same
 * thing? Every field is evidence for YOU to decide on — the system proposes, it never
 * quietly folds identities together.
 */
export interface PendingMerge {
  mergeId: string;
  otherId: string;
  otherName: string;
  otherKind: string;
  /** Similarity that surfaced the pair (0–1). */
  confidence: number;
  suggestion: MergeSuggestion;
  /** Plain-language evidence, e.g. "name overlap 78%". */
  why: string;
  /** The judge's own words, when it gave any. */
  reason?: string;
}

/** GET /api/entities/{id}/context — the whole surface payload for one entity. */
export interface EntityContextPayload {
  entity: Entity;
  edges: Edge[];
  context: ContextItem[];
  merges: PendingMerge[];
}

/** Semantic tone keys — resolved to Aladin theme tokens by the UI. */
export type Tone =
  | "for"
  | "against"
  | "catalyst"
  | "echo"
  | "amber"
  | "qualify"
  | "neutral"
  | "ink2";

export interface RelMeta {
  label: string;
  glyph: string;
  tone: Tone;
}

export interface CtxMeta {
  icon: "source" | "add" | "question";
  label: string;
  tone: Tone;
}
