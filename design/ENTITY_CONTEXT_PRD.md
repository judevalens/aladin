> Source: design handoff `design_handoff_entity_context/PRD.md` (external bundle), imported 2026-07-17. Companion screenshot: `screens/entity-context-01-top.png`.

# PRD — Entity Context surface

> The single most polished surface in Aladin, and the one we are hardening for production.
> This document states the *intent*: what the surface is for, what must be true, and how it should behave.
> The bundled HTML is the visual/behavioral reference. The coding agent decides what to keep vs. rebuild.

---

## 1. What this surface is

The **Entity Context page** is the detail view for a single node in the user's knowledge graph — an
`asset`, `company`, `person`, or `concept`. In Aladin the unit of value is not the note or the document;
it is **the entity and how it wires to other entities**. This page is where that web is made legible.

Reference entity in the mock: the concept **"Memory as a moat."**

The page has a strict information hierarchy, top to bottom:

1. **Identity** — what this entity is (kind, gist, how long it's been tracked, which watchers feed it).
2. **A pending structural change** — a suggested new edge a watcher just proposed, awaiting accept/dismiss.
3. **How it relates** *(the lead)* — the typed, directional edges to other entities. **This is the payoff.**
   The "understanding" lives in the relationships, not in a summary.
4. **Context** *(underneath)* — the raw accreted material (quotes, your notes, open questions), **verbatim,
   never paraphrased into a "claim."**

## 2. Why it matters (product thesis)

Aladin's moat is *accumulated user context*, and this page is the proof of that thesis rendered as UI.
Two non-negotiable principles drive every decision here:

- **Relationships are the understanding.** The value of an entity is how it connects. Edges lead; the
  source material supports. Never invert this — do not turn the page into a document reader with links bolted on.
- **Context is never summarized.** Quotes, notes, and questions appear as the user (or their sources)
  actually wrote them. Aladin may *route* and *connect* material, but it must not rewrite it. This is a
  trust guarantee, not a styling choice.

## 3. Data model

Everything on the page derives from three record types (aligned with the core engine — entities / theses / claims).

### Entity (the focused node)
| field | type | notes |
|---|---|---|
| `name` | string | display title |
| `kind` | `concept \| entity \| asset \| company \| person` | shown as an uppercase mono label |
| `gist` | string | one-sentence definition; the only prose the page authors |
| `watchers` | platform[] | sources monitoring this entity (`x`, `rss`, `reddit`, `hn`, `filing`, `paper`) |
| `since` | string | provenance line, e.g. "tracked 3 weeks · 14 pieces of context" |

### Edge (a typed, directional relationship to another entity)
| field | type | notes |
|---|---|---|
| `rel` | relation key | one of the 7 relation types below |
| `to` | string | name of the target entity |
| `kind` | string | target's kind (`concept`, `entity`, …) |
| `why` | string | the user's / watcher's reasoning for the edge — the substance |
| `origin` | `you \| watcher` | authored by the user vs. discovered by a watcher → drives the YOURS / FOUND tag |

**Relation types** (typed edges — directional, each with a glyph + low-chroma tone):

| key | label | glyph | tone |
|---|---|---|---|
| `enables` | enables | → | `#7ba98c` |
| `enabledBy` | enabled by | ← | `#7ba98c` |
| `supports` | supported by | ⊕ | `#808a99` |
| `contradicts` | in tension with | ⊗ | `#c58579` |
| `partOf` | part of | ⊂ | `#948da6` |
| `instance` | instance | • | `#bd9a63` |
| `competes` | competes with | ⇄ | `#c9925a` |

### Context item (raw accreted material)
| field | type | notes |
|---|---|---|
| `type` | `quote \| note \| question` | drives icon + label + treatment |
| `body` | string | **verbatim** text — never edited |
| `who` | string | attribution (`@handle`, publication, or `me`) |
| `platform` | platform \| null | shows a platform chip when present |
| `time` | string | relative time |

Context types: **quote** (`#8f8d98`, left-rule treatment), **your note** (amber `#c9925a`, brighter text),
**open question** (`#808a99`).

## 4. Behavior we want when this is hardened

The mock renders these as static/inert. Hardened intent (coding agent decides implementation + persistence layer):

1. **Suggested-edge triage (the amber RelSignal banner).**
   - **Keep edge** → commits the suggested edge into "How it relates" (appears as a FOUND edge), banner dismisses.
   - **Dismiss** → banner dismisses, no edge written.
   - There may be zero or several pending suggestions; render one banner per pending suggestion, or none.

2. **Draw a connection** (the dashed CTA under the edge list).
   - Opens an affordance to pick a **relation type** + a **target entity** (existing or new) + a **why**.
   - On confirm, writes a new edge with `origin: 'you'` (renders a YOURS tag).

3. **Edge → navigation.** Clicking an edge row navigates to that target entity's own Context page
   (same surface, re-centered). Needs a back-trail / history so the user can "pull the thread."

4. **Add to Context.** The user can append a **note** or **question** to Context; it is stored verbatim,
   attributed to `me`, timestamped. Aladin-ingested quotes arrive via watchers, not manual entry.

5. **State transitions & persistence.** In the prototype, state is session-only. For production, edges,
   context items, and suggestion accept/dismiss must persist (the graph is the product — see product.md
   "Parked: Persistence"). Writes should go through a **typed, provenance-carrying write-path**
   (who asserted it, when, inferred vs. asserted) — never a silent mutation.

## 5. Edge cases the hardened surface must handle

- **Empty entity** — freshly created, **no edges and no context yet.** Needs a real first-run state that
  invites the first connection / first captured context, not a blank void.
- **Long strings** — long `name` (wrap, don't truncate the title), long `gist`, long quotes. `text-wrap: pretty`.
- **Many edges (20+)** — the list should stay scannable; consider grouping by relation type or by origin,
  and keep the edge count visible. Do not paginate away the substance.
- **Missing fields** — no `platform` on a context item (hide the chip), no watchers (hide the chip row),
  **unknown relation type** (fall back to a neutral glyph/tone rather than crashing).
- **No pending suggestion** — the amber banner simply doesn't render.
- **Loading** — while watchers are working, a calm skeleton for the edge/context lists (no spinners-as-decoration).

## 6. Explicit non-goals

- No paraphrasing/summarizing of Context material. Ever.
- No turning "How it relates" into a generic backlinks list — edges are **typed and directional** and carry a *why*.
- No heavy graph-canvas visualization on this page; that's the separate Graph view. This surface is the
  **readable, relationship-first** detail view.
- No new accent colors. Amber is the only thing that pops; relation/semantic hues are deliberately low-chroma.

## 7. Success criteria

- A user can land on any entity and, within seconds, understand **what it is** and **how it connects**.
- Accepting a suggested edge, drawing a connection, and adding a note all **visibly grow the graph** and survive refresh.
- The surface holds up on a brand-new empty entity and on a dense, heavily-connected one — with no layout breakage.
- Nothing on the page misrepresents the user's material (verbatim guarantee intact; provenance always visible).
