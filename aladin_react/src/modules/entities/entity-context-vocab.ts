// The Entity Context relation + context vocabulary (design/ENTITY_CONTEXT_PRD.md §3).
// Production vocabulary, not fixtures — the mock and the real read-path both render
// through it.
//
// Keys are the wire values (snake_case), matching relationships.rel_type in Postgres.
// Tones resolve to Aladin theme tokens in the UI; deliberately low-chroma — amber is the
// only colour that pops.
import type { CtxMeta, RelMeta } from "./entity-context-types";

export const REL: Record<string, RelMeta> = {
  enables: { label: "enables", glyph: "→", tone: "for" },
  enabled_by: { label: "enabled by", glyph: "←", tone: "for" },
  supports: { label: "supported by", glyph: "⊕", tone: "catalyst" },
  contradicts: { label: "in tension with", glyph: "⊗", tone: "against" },
  part_of: { label: "part of", glyph: "⊂", tone: "echo" },
  instance: { label: "instance", glyph: "•", tone: "qualify" },
  competes: { label: "competes with", glyph: "⇄", tone: "amber" },
};

/** Relation options for the "Draw a connection" picker, in the PRD's order. */
export const REL_OPTIONS = Object.keys(REL);

export const CTX_META: Record<string, CtxMeta> = {
  quote: { icon: "source", label: "quote", tone: "ink2" },
  note: { icon: "add", label: "your note", tone: "amber" },
  question: { icon: "question", label: "open question", tone: "catalyst" },
};

/** Unknown relation types fall back to a neutral glyph/tone rather than crashing (PRD §5). */
export const REL_FALLBACK: RelMeta = { label: "related to", glyph: "•", tone: "neutral" };

export function relMeta(rel: string): RelMeta {
  return REL[rel] ?? REL_FALLBACK;
}

/**
 * Group edges by relation type for "How it relates". A flat list of nine links is a
 * dump; "enables · contrasts with · related to" is a shape you can read.
 *
 * Known relations come first in REL order, so the page's sections are stable rather than
 * data-ordered; unknown relations trail after in first-seen order (never dropped).
 */
export function groupEdgesByRel<T extends { rel: string }>(
  edges: T[],
): { rel: string; edges: T[] }[] {
  const byRel = new Map<string, T[]>();
  for (const edge of edges) {
    const bucket = byRel.get(edge.rel);
    if (bucket) bucket.push(edge);
    else byRel.set(edge.rel, [edge]);
  }
  const out: { rel: string; edges: T[] }[] = [];
  for (const rel of REL_OPTIONS) {
    const group = byRel.get(rel);
    if (group) {
      out.push({ rel, edges: group });
      byRel.delete(rel);
    }
  }
  for (const [rel, group] of byRel) out.push({ rel, edges: group });
  return out;
}
