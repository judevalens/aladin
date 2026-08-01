# Curation UX — PRD (DRAFT for design exploration)

> Status: **DRAFT** — for design-agent exploration. NOT part of the locked design set
> (`PRD.md` / `DESIGN_SPEC.md` / `BROWSER_SPEC.md`). Goal of this doc: model *how curation
> works*, not final visuals.

## TL;DR

Aladin's core is a **knowledge substrate** — entities (people, orgs, topics…) and claims
(contestable propositions about them). Most of it is **AI-extracted from ingested sources**, so
it accumulates three kinds of rot: **duplicates** (five "Sam Altman"s), **bad merges** (two
different "Apex" orgs collapsed into one), and **wrong facts** (a claim that's false, or not
actually about this entity).

Every surface in the app (entity profiles, Signals, insights) is a *projection* of this substrate
— so noise in the substrate is noise everywhere, amplified. **Curation is how a human keeps the
substrate trustworthy.** And because verified human judgment is the scarce asset in a world flooded
with AI-generated content, curation isn't a maintenance chore — it's the moat.

We need to design the surfaces where a user **reviews, merges, corrects, and verifies** the
substrate with minimal friction. Three surfaces:

- **A — The Review Queue** — a triage inbox of resolution decisions the AI can't make confidently.
- **B — Entity Profile curation** — verify/reject/correct in the context of reading an entity.
- **C — In-context verbs** — lightweight judgment on cards in the Signals feed.

## Who this is for

A solo **analyst / investor / operator** tracking a fast-moving domain — someone whose reputation
or P&L depends on being *right and early*. They rely on the substrate being clean because they
build positions on top of it. They are attention-poor and impatient: curation only happens if it
is fast, safe, and mostly a byproduct of things they'd do anyway.

## Non-negotiable principles (apply to every screen)

1. **Provenance is always visible.** Every datapoint shows whether it's **AI-generated** or
   **human-verified**, plus confidence. The user must know at a glance what to trust vs. what to check.
2. **Judgment is low-friction.** Keyboard-first, one-glance decisions, batchable where safe. A chore
   doesn't get done. Aim for a Superhuman-triage feel, not a database admin panel.
3. **Everything is reversible.** Undo and revert everywhere. Users act fast *because* nothing is
   permanent — an accidental merge is always one click from undone.
4. **Show the evidence for the decision.** Never ask "same entity?" or "is this claim right?" without
   the evidence (the two candidates, their sources, their context) inline. No blind judgments.
5. **AI proposes, human disposes, and the verdict is sticky.** A human verdict visibly outranks the
   AI and will not be silently overwritten by the next extraction pass. "I fixed this" must *stay* fixed.
6. **Curation rides on consumption.** Verbs live *inside* the reading surfaces (profile, feed), not
   only in a dedicated queue. The best curation is a side effect of reading.

## Objects & states (the shared vocabulary)

- **Entity** — a resolved thing. Has a canonical **name**, a **kind** (person/org/product/place/
  topic/concept/work), optional **disambiguators** (domain, handle, location), and a **trust tier**.
- **Claim** — a contestable proposition about ≥1 entity. Has **stance evidence** (N sources *assert*
  / M *deny* / K *hedge*) and **argument edges** (supports / contradicts / qualifies other claims),
  plus a trust tier.
- **Trust ladder** (the spine of the provenance language): **auto** (AI, unreviewed) → **believed**
  (default) → **verified** (human-confirmed; locked against the resolver).
- **Proposed merge** — the resolver's guess that two entities (or claims) are the same thing. States:
  **proposed → applied | rejected | reverted**. Carries a **confidence** and **evidence** (why it
  thinks so: name similarity, shared context, overlapping sources).

## Surface A — The Review Queue *(the Phase-0 core)*

**Purpose.** A triage inbox of the resolution decisions the AI deliberately kicked to a human:
mostly *"are these two the same?"* (duplicate merges) and *"are these actually different?"*
(ambiguous same-name splits). This is where duplicate/ambiguity debt gets burned down.

**Feels like.** An email/Superhuman inbox *for the graph* — a queue you clear, not a report you read.

**One review item shows:**
- The two candidates **side by side** (name, kind, disambiguators, trust, tiny stats — e.g. # claims,
  # sources).
- The **evidence for the proposal**: why the AI thinks they match (name similarity %, shared
  context/sources), and where each candidate appears.
- A **confidence** signal.
- **Actions:** `Merge` (accept) · `Keep separate` (reject) · `Skip` · *(later)* `Split`. Keyboard
  bindings for each. Every action is reversible (undo toast; applied merges revertible from history).

**Batching.** A **high-confidence** group can be bulk-accepted ("merge all 12 obvious ones"); the
**ambiguous middle** is reviewed one at a time. Make the two clearly separate zones.

**Entry points.** A rail/nav entry ("Review") with a **count badge** of pending items; plus
contextual entry from an entity profile ("3 possible duplicates →").

**States to design:** empty ("queue clear ✓"), loading, high- vs low-confidence sections, conflicting
evidence, mid-action undo, post-bulk confirmation.

## Surface B — Entity Profile curation *(Phase 1)*

The entity profile is the substrate rendered as a page: an AI **summary**, its **claims**, related
**insights**, and **sources**. Curation lets the user fix what the AI got wrong, in place:

- **Summary** — AI-generated, **editable**. Editing pins it as **verified/human-authored**; the AI
  may later flag *"new info available"* but must never overwrite. Show the state clearly:
  *AI-generated* vs *edited by you*.
- **Claims list** — each claim can be **Verified** (trust ↑), **Rejected / crossed off** (hidden +
  teaches ranking), or flagged **"not about this entity"** (removes the subject link — this is a
  *different* verdict from "the claim is false"; both matter). Provenance + evidence (assert/deny
  sources) visible inline, expandable.
- **Data points** — **name / kind / disambiguators** are editable; an edit sets **verified** and
  **locks the field** from the resolver. Show which fields are AI-guessed vs human-set.
- **Duplicates** — "possible duplicates" surfaced here, linking into the Review Queue flow (A).

## Surface C — In-context verbs *(Signals feed)*

Lightweight judgment on signal/claim cards *while reading*: `believe` · `reject` · `save to thesis`
· `dismiss`. One-tap, reversible, feeds ranking. Lighter than A/B — but the **verb set and their
meaning must be identical** across all three surfaces (a "reject" means the same thing everywhere).

## Provenance & trust visual language *(design a system, not one-offs)*

This is the backbone that repeats on every surface — please propose it as a coherent system, not
per-screen decoration. It must express, consistently:

- **AI-generated** vs **human-verified** vs **human-edited**.
- **Confidence** (tier? bar? number? — propose the model).
- **Evidence strength** (how many sources assert vs deny; is it contested?).
- **Locked** (human-verified, resolver-protected) fields.

Get this language right and every curation screen mostly falls out of it.

## Key flows to storyboard

1. **Burn down the queue** — open Review → judge an ambiguous merge *with evidence* → Merge/Keep
   separate → next → (undo the last one to show reversibility).
2. **Correct an entity** — open "Sam Altman" → summary is subtly wrong → edit → it's now *verified* →
   scroll claims → cross off a false one, flag one as "not about him" → fix the `kind`.
3. **In-feed verdict** — reading Signals → cross off a wrong claim → it disappears and (implicitly)
   teaches ranking.

## Edge cases & states

Empty queue · low-confidence flood · conflicting evidence · undo/revert · bulk-accept · the *correct*
"keep separate" (two real "Apex" orgs) · a rejected merge that must **not** be re-proposed / nag ·
an entity with zero human-verified fields (all-AI) vs a heavily-curated one.

## Non-goals (v1)

- Multi-user / shared / collaborative curation (single-user first).
- The sharing / knowledge-network layer (later north star).
- Server-side auto-merge *policy* (handled in the backend; the UI only surfaces what's proposed).

## References

- **Design system:** Aladin **dark-minimal IDE** aesthetic; tokens in
  `aladin_react/src/index.css` (surfaces `bg-rail/panel/card/…`, ink ramp, `bg-amber`,
  `border-line`, semantic `text-for` / `text-against`). Follow `design/DESIGN_SPEC.md`.
- **Match the existing surfaces:** the built **Insights** surface
  (`aladin_react/src/modules/insights`) — curation should feel like the same product,
  same card grammar. (The Signals surface was removed with the claim layer.)
- **Reference renders:** `design/screens/`.
