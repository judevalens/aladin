> **Status:** draft PRD — **rev 2 (2026-08-11)**. Not locked. Product intent for the next
> Aladin surface — **"Tutor."** Companion plan: `~/.claude/plans/tutor-surface.md`.
>
> **What changed in rev 2.** The ingestion engine shipped and now runs in prod (a 361-page
> paper → 361 pages, 6147 typed regions, 870 chunks), which closes **D-B** and removes the
> subsystem this PRD called "the hard, net-new half." That reframes the surface: the unit
> of a lesson is no longer a document but a **plan item** in a study plan the user and the
> agent co-author and keep revising (**D-I**, §3, §4a, §6). Equations render as real math
> via vision→LaTeX (**D-J**), gated on the §13 T0 spike. §13 is the build order.
> Reads on top of `TRADING_PRD.md` (north star), `SHARD_MODEL.md` (the output medium),
> `DESIGN_SPEC.md` (tokens/components).

# PRD — Tutor (source → interactive teaching Shard)

> Drop in a source you want to understand — a white paper, a chapter, a dense PDF — and
> get back a **super-interactive lesson you can explore**, authored by Copilot and
> rendered as a Shard. This document states the intent: what the surface is for, what
> must be true, and how it behaves.

---

## 1. What this surface is

**Tutor turns a source into an interactive lesson.** The user brings raw material they
want to internalize — a research PDF, an ebook chapter — and Aladin hands back a
**teaching Shard**: a sandboxed, interactive React app that explains the material in
stages, checks understanding, and lets the user *manipulate* the ideas (sliders,
walkthroughs, quizzes, definitions on demand), rather than re-reading prose.

The flow, end to end:

```
  drop a PDF               ingest + author                 a lesson you explore
  ──────────               ───────────────                 ────────────────────
  source PDF   →   ingest engine  →   Lesson Author   →    Shard (interactive)
                   (images/eqns/       turn over               opens as a tab,
                    code → text)       Copilot + kit           lives in your library
```

Three moving parts, two of which **already exist**:

- **Copilot** authors the lesson. It already has the full Shard toolchain
  (`create_app → write_file → build_app → publish_app`) and is already prompted to build
  shards (`internal/service/copilot.go`). We give it a *teaching* brief and a source.
- **Shard** is the medium. Agent-authored multi-file React, esbuild-in-Go, sandboxed
  opaque-origin iframe, composed from `@aladin/kit`, rendered as a work-pane tab
  (`modules/doc-surface/`). No new runtime.
- **The ingest engine** was the one net-new subsystem. **It is now shipped and running in
  prod** (`INGESTION_PRD.md`): a dropped PDF becomes pages, typed regions, chunks and an
  outline, readable by an agent through `read_document` / `search_document`. The claim this
  bullet used to make — that an agent cannot teach what it cannot read — is answered.

The net-new work left is therefore *not* a subsystem. It is **the plan** (§6) — the object
that makes a 361-page source teachable in pieces — and **equation fidelity** (§13 T0), the
one step whose feasibility is still unproven.

## 2. Why it matters (product thesis)

The `TRADING_PRD.md` §1 corollary is the whole justification: *"when in doubt, build the
thing that teaches you the domain."* A novice trader with a strong technical background
learns markets **through** the harness. The fastest way to metabolize a dense source —
a factor-model paper, a chapter on options greeks, a talk on market microstructure — is
not to re-read it but to **take it apart interactively**. Tutor is that tool.

**This is not the "education layer" §1 rules out.** That exclusion is about onboarding,
guardrails, and hand-holding for *strangers whose judgment we can't model*. Tutor is the
opposite: a **single-player instrument for the user's own understanding**, that works on
*any* source they choose, with zero curriculum we have to maintain. Self-first, exactly
as §1 demands.

**The lesson is a durable artifact, not a throwaway view.** Per the standing feedback
that primitives must tie back to the platform (not "just a pretty list"), a generated
lesson is a Shard artifact that lives in a folder, survives refresh, is reopenable, and
carries provenance back to its source. Over time the user accrues a **personal library
of interactive explanations of the things they chose to understand** — the same "nothing
is thrown away" bet the research log makes (`TRADING_PRD.md` §2), applied to learning.

## 3. The core loop (what the user actually does)

> **rev 2 change.** The loop is no longer "drop a PDF, get a lesson." A dense source is
> not one lesson — the paper that motivated this rev is **361 pages**. The unit of work is
> a **study plan** the user and the agent write *together* and keep revising. Lessons are
> generated *from plan items*, on demand, one at a time. The plan is the durable thing;
> lessons are its output.

1. **Bring a source.** Drop a PDF (v1 scope — see §10 D-E). Ingest runs (§5): outline,
   pages, regions, chunks. Legible progress; legible failure.
2. **Plan it together.** The agent reads the outline and proposes a study plan — an
   ordered set of items, each scoped to a real span of the source, each with a stated
   objective ("after this you can price a collar from its legs"). The user **edits it**:
   drop what they already know, split what's too big, reorder, add "I want a worked
   example." The plan is the conversation's *artifact*, not its transcript.
3. **Author one lesson, when wanted.** The user picks an item and asks for it. A *Lesson
   Author* turn runs scoped to **that item's span only** — small enough to be reliable,
   which is exactly why the plan exists. They watch it build in the dock.
4. **Approve publish.** `publish_app` is gated (`copilot.go:54`) — the lesson holds for
   one click. The natural review gate.
5. **Explore it.** It opens as a work-pane tab, and the plan item now points at it.
6. **Come back and revise.** Days later the plan is still there, showing what's authored
   and what isn't. The user re-scopes an item, adds one the source suggested, re-authors a
   lesson that landed badly, or marks something learned. **The plan is revisable forever** —
   it is the learning analogue of the research log, not a one-shot wizard output.

The loop's shape matters: step 2 makes step 3 *possible*. A 361-page source can't be
taught in one turn, and silently teaching only the first 30 pages is the failure mode §8
forbids. Planning first is not ceremony — it's the mechanism that keeps each authoring
turn inside a window the agent can actually hold.

## 4. What it should feel like (the bar)

### 4a. The plan — a syllabus you argue with

The first thing the user sees after ingest is **not** a lesson and **not** a chat log. It
is a plan they can edit, that happens to have been drafted by an agent:

- **It knows the source.** Items are scoped to real spans (`pp. 88–104`, §3.2), taken from
  the outline the engine already extracts — never invented chapter names.
- **It states outcomes, not topics.** "Payoff algebra of collars" beats "Chapter 4."
  Each item says what the user will be able to *do*.
- **It is edited by talking or by hand.** "Skip the intro, I know Black–Scholes" removes
  items; so does clicking delete. Reordering, splitting a fat item, and re-scoping are
  first-class — the plan is a document the user owns, not a proposal they accept or reject.
- **It is honest about size.** An item covering 60 pages says so, and the surface suggests
  splitting it, because that item will produce a bad lesson.
- **It persists and it is revisable.** Reopened next week it shows what's authored, what
  isn't, what was marked learned. Adding an item later is normal, not a re-run.

The feel to aim for: **a reading list you and a knowledgeable colleague drew up together,
which remembers where you got to.** Not a wizard, not a chatbot transcript.

### 4b. The lesson — what "super-interactive" and "explore" mean

The user's phrase was *"a super interactive shard that teaches and lets you explore."*
Concretely, a good lesson is **not** a styled document. It should reliably contain:

- **Staged explanation** — the idea unfolds in sections/steps (hash-routed or stepper),
  not one wall of text. The user controls pace.
- **At least one manipulable model** — a `Simulator`: parameters the user changes and a
  result that recomputes live (e.g. move volatility, watch the option price move). This
  is the payoff a PDF cannot give.
- **Check-for-understanding** — inline `Quiz` / self-test with feedback, so the user
  learns actively.
- **Definitions on demand** — `Glossary`/`Define`: hover or tap a term for its meaning,
  so the lesson stays readable without dumbing down.
- **Faithful rendering of the hard parts** — the equations, diagrams, and code from the
  source appear *as first-class interactive content* (a rendered formula the user can
  poke, a runnable/steppable code block, a redrawn diagram), because the ingest engine
  captured them as text rather than dropping them.
- **Grounding** — key claims trace back to the source (a section reference / quote), so
  the user trusts it and can go deeper. **The lesson transforms the source; it must not
  silently invent beyond it** (see §7).

"Explore" has two tiers:

- **v1 — self-contained interactivity.** Everything above, inside the shard's sandbox.
- **v2 — connected exploration.** Links out to the entity/ticker a lesson touches, and
  live Aladin data bound into the lesson via the kit data bridge (`useNode`/`useNodes`).
  **Deferred** — the shard data bridge is a deliberate stub today (host answers only
  `ping`; `doc-surface-ui.tsx:136`) and shard data-wiring is paused until the data model
  settles. Do not build the KG-bound lesson first.

## 5. Architecture (reuse, don't reinvent)

```
  L0  Ingest engine     PDF → structured text (figures/equations/code)   [net-new]
        │               multimodal transcription · async · reliable
        ▼
  L1  Agent-readable    get_artifact returns text · search indexes files
        │               source artifact ── attached to ──┐
        ▼                                                 ▼
  L2  Lesson Author     a Copilot turn: teaching system prompt + source artifact
        │               → create_app → write_file (teaching kit) → preview/verify
        │               → build_app → publish_app (GATED: user approves)
        ▼
  L3  The lesson        a Shard artifact (kind:"app") with source_artifact_id
                        rendered in the work pane · listed in the Tutor library
```

### The ingest engine (L0) — the hard, net-new half

> **Update 2026-08-01: L0 is no longer net-new.** `design/INGESTION_PRD.md` builds it —
> v1 (text, outline, status, retrieval) is shipped on `feat/ingestion-engine`, and the
> layout-segmentation half is designed and benchmarked. It reached the same decisions this
> section did (subprocess not sidecar, swappable interface, outbox status, scan-robust).
>
> Two changes to what follows, both making it cheaper:
> - **Step 3's cost lever becomes region-level.** Segmentation labels `figure` /
>   `isolate_formula` / `table`, so vision runs on *crops*, not whole pages.
> - **Step 1 is often unnecessary.** For OCR'd scans the page image is already embedded and
>   the text layer already exists; only the visual regions need a model at all.
>
> Read INGESTION_PRD §10 and §13b–e before building against this section.

The requirement (user, D-A): *"a proper ingest engine that can handle images, equations,
code — transpose them to text."* A plain PDF text-layer extractor will not do that. The
backbone is **multimodal transcription**:

1. **Rasterize** each PDF page to an image. *Decided:* a Go package that shells out to a
   CLI rasterizer via `exec.Command` (no persistent sidecar) — `pdftoppm` (poppler) or
   `mutool draw` (mupdf), behind a small swappable `Rasterizer` interface. The binary is a
   documented system dependency (dev machine + backend container); the input path is
   validated/sandboxed and the exec is page-bounded + timed out.
2. **Transcribe** each page with a **vision-capable model** into structured markdown:
   - prose with headings/sections preserved,
   - **equations → LaTeX** (`$$…$$`),
   - **code → fenced code blocks** (language-tagged),
   - **figures/diagrams → captioned descriptions** (and, where cheap, the cropped image
     retained as a data asset the lesson can redraw or embed).
   This is robust to scanned/image-only PDFs, which a text-layer extractor fails on
   entirely.
3. **Optional fast-path:** for clean digital PDFs, take the embedded text layer directly
   and only fall back to vision for pages that are figure/formula-heavy — a cost lever,
   not required for correctness.
4. **Assemble** the per-page output into one structured document; **persist** it as the
   agent-readable text (§6).

Runs **async** through the existing asynq pipeline + reaper (`internal/pipeline`) so a
stranded ingest is re-driven; status transitions ride the transactional outbox (same
pattern as `shard_build_state`). Reuses the app's existing LLM infrastructure rather than
a bespoke ML stack.

### The rule that must not bend

**One authoring path.** A lesson is authored by the *same* Copilot + Shard toolchain the
user already has, not a parallel generator. If Tutor ever forks its own agent/runtime, we
maintain two of everything and they drift. Tutor is an **ingest engine + a brief** over
the existing engine.

## 6. Data model

Thin, in the spirit of `TRADING_PRD.md` §2 — add the minimum, throw nothing away.

### Ingested source text — **SHIPPED, D-B closed**

This is no longer a design question. The ingestion engine (`INGESTION_PRD.md`) shipped the
tables, and they are richer than the single `text` blob this section originally proposed.
Verified in prod against a 361-page paper: **6147 regions, 870 chunks, 361 pages.**

| table | what it holds | Tutor uses it for |
|---|---|---|
| `artifact_documents` | `status`, `extractor`, `page_count` — exactly the provenance fields specced here | ingest state; refusing to plan over a failed source (§7) |
| `artifact_pages` | per-page text | the span an authoring turn actually reads |
| `artifact_regions` | typed boxes: `title`, `plain text`, `isolate_formula`, `formula_caption`, `table`, `figure`, `abandon` | **the equation pipeline (§13), figure handling, dropping headers/footers** |
| `artifact_chunks` | the navigable tree (870 for that paper) | the outline the plan is scoped against |

Retrieval already exists over MCP — `read_document`, `search_document`, and the outline
path (`internal/mcp/workspace_tools.go:560`). The original worry, that `get_artifact`
returns `""` for files, is moot: the agent reads documents through dedicated tools instead.
`search.go:169` still excludes files from federated search — a real gap, but not Tutor's.

### Study plan (net-new) — the surface's spine

The plan is the durable object; a lesson is its output. Thin, per `TRADING_PRD.md` §2.

| field | notes |
|---|---|
| `id` / `user_id` | |
| `source_artifact_id` | the ingested source it plans over (one source in v1; multi-source is a later join table, don't build it now) |
| `title` | agent-proposed, user-editable |
| `goal` | what the user said they wanted out of the source — the brief every authoring turn inherits |
| `created_at` / `updated_at` | |

**Plan item** — ordered, scoped, revisable. This is the unit a lesson is authored from.

| field | notes |
|---|---|
| `plan_id` / `ordinal` | ordering is user-editable; reordering must not renumber anything the user can see |
| `title` | "Payoff algebra of collars" |
| `objective` | what the user can do afterwards — the teaching brief |
| `scope` | the span: chunk ids and/or a page range. **Must reference the real document**, so an authoring turn can fetch exactly that text and nothing else |
| `status` | `planned \| authored \| learned` — user-set, not inferred |
| `lesson_artifact_id` | the Shard once authored (nullable). Re-authoring replaces it; the plan item survives |

Two properties the schema must protect: **an item outlives its lesson** (re-author freely),
and **scope is a reference, not a copy** (re-ingesting the source must not orphan the plan).

### Lesson provenance (net-new, thin)
A lesson **is** a Shard artifact (`kind:"app"`) — no new heavy table. It gains:

| field | notes |
|---|---|
| `source_artifact_id` | the source it was generated from — the provenance FK |
| `plan_item_id` | the item it was authored for — how the plan finds its lessons |
| (marker) | distinguishes a "lesson" shard from a hand-built shard (a metadata flag) |

Optional, later: the shard's `anchors.json` `refs[]` carry entity ids the lesson teaches
(the manifest already supports this) — the seam for v2 connected exploration.

## 7. The trust guarantee (and how it differs from Entity Context)

Entity Context has a **verbatim** guarantee — it never rewrites the user's material.
Tutor is the *opposite* by design: a lesson **transforms** the source into an
explanation. That makes grounding the safeguard instead of verbatim-ness:

- The lesson must **stay faithful to the source** and **cite back to it** for its key
  claims (section/quote references), so the user can verify and go deeper.
- It must not **silently fabricate** facts, figures, equations, or data the source
  doesn't contain. Model-added scaffolding (analogies, worked examples) is fine and
  useful — but numeric/factual assertions trace to the material.
- When ingest was poor or the source is thin, the lesson **says so** rather than
  confabulating a confident lesson over missing text. (The ingest `status`/quality is
  visible to the author turn.)

## 8. Behavior & edge cases the surface must handle

- **Ingest failure** — encrypted PDF, a rasterizer error, a page the model can't
  transcribe. Fail **legibly** with a real message and a retry; never a silent empty
  lesson. (Ties to the standing "no silent caps" instinct.)
- **Huge source** — a 200-page book. v1 may cap/scope (e.g. "this chapter," or a page/
  section picker) — and must **say what it scoped**, not silently truncate. Ingest cost
  scales per page, so scoping is also a cost control.
- **Generation is slow** — ingest is per-page model calls, and authoring a shard is many
  tool calls. The dock's existing tool trail + status affordances carry authoring; the
  Tutor surface shows real "ingesting…" / "building your lesson…" states, not a
  spinner-as-decoration.
- **Publish approval** — the finished lesson holds at `publish_app` for one click. If the
  user walks away, the draft is recoverable (the shard exists on disk pre-publish).
- **Build/mount failure** — a shard can build yet not mount; `publish_app` already gates
  on `verifyMount` and refuses to publish a broken lesson. Surface that state honestly.
- **Empty library (first run)** — a real first-run state that invites the first source,
  not a blank void.
- **Re-generation** — the user dislikes the lesson and wants another pass. A lesson
  should be regenerable from the *already-ingested* source (no re-ingest), keeping the
  source attachment.

## 9. Explicit non-goals

- **Not a curriculum / MOOC for strangers.** No authored course content we maintain, no
  onboarding layer. The user supplies the source; single-player (`TRADING_PRD.md` §1, §3).
- **Not a document reader.** The output is an interactive lesson, not a PDF viewer. If the
  user just wants to read the source, that's the existing artifact, not Tutor.
- **Not a new agent or runtime.** Reuses Copilot + Shard. No forked authoring path (§5).
- **Not a verbatim surface.** Tutor transforms; grounding (not verbatim-ness) is the
  guarantee (§7). Don't import Entity Context's rules here.
- **Not live-data-bound in v1.** The kit data bridge stays stubbed; connected exploration
  is v2 (§4), gated on shard data-wiring being unpaused.
- **Not a bespoke ML/parser stack.** The ingest engine leans on the app's existing
  multimodal LLM + a thin rasterizer, not a from-scratch document-layout model.

## 10. Decisions

| # | Decision | Status |
|---|---|---|
| **D-A** | **Ingest engine** | **RESOLVED — a proper ingest engine.** Multimodal transcription: rasterize PDF pages → vision model → structured markdown (equations→LaTeX, code→fenced, figures→captioned), with an optional text-layer fast-path (§5). Handles images/equations/code per the requirement. |
| **D-E** | **v1 source types** | **RESOLVED — PDF only.** Article URL and YouTube/transcript are fast-follows (L4); each slots into the ingest engine as another front-end. |
| **D-F** | **Surface name** | **RESOLVED — "Tutor."** Nav destination `/tutor`, module `modules/tutor`, spike `/spike/tutor`. |
| **D-B** | **Where ingested text lives** | **RESOLVED by shipping — `artifact_documents` / `_pages` / `_regions` / `_chunks`** (§6). Not the proposed `artifact_text` blob: typed regions are what make the equation pipeline (§13) possible at all. Verified in prod on a 361-page source. Fanning into `records` for the entity layer stays open, and is not Tutor's call. |
| **D-I** | **Unit of a lesson** | **RESOLVED — a plan item.** Not the document (361 pages can't be one turn) and not auto-chaptering (fires N expensive turns for sections nobody asked for). The user and agent co-author a revisable plan; lessons are authored per item, on demand (§3, §4a). |
| **D-J** | **How equations reach the lesson** | **RESOLVED — vision → LaTeX, rendered.** Formula crops transcribe to LaTeX and render (KaTeX) in the shard, so math is typeset content a lesson can step through and bind to a simulator. Gated on the §13 T0 spike: this is the one assumption that can sink the surface. |
| **D-C** | **Generation driver** — general Copilot capability vs. a dedicated "Lesson Author" turn? | Open. *Rec:* a dedicated Lesson-Author system prompt + a surface-kicked turn, streamed in the existing dock. Add a `tutor` surface kind to `systemPrompt(surface)` (`copilot.go:824`). |
| **D-D** | **Teaching kit primitives** — build `Quiz`/`Simulator`/`Glossary`/`StepThrough`/`Reveal`/`Figure` into `@aladin/kit`, or freehand? | Open. *Rec:* build them into the kit (`docsurface/kit.tsx`) — consistent, reliable output; the authoring guide teaches their use. |
| **D-G** | **Source→turn attachment** — copilot turns are text-only today. | Open. *Rec:* explicit `source_artifact_id` on the turn; the agent reads it via `get_artifact`. Don't stuff the paper into `surfaceContext` (caps at 1800 chars, `copilot.go:871`). |
| **D-H** | **How much "explore" in v1** | Open. *Rec:* self-contained interactivity only; live entity/ticker links + data bridge deferred to v2 (§4). |

## 11. Success criteria

- A user drops a white paper and, in one guided flow, ends up with a **published,
  interactive lesson** that opens as a tab and that they can actually learn from — staged
  explanation, at least one manipulable model, a check-for-understanding, definitions on
  demand, the source's equations/diagrams/code rendered as first-class content —
  **grounded in the source**.
- The lesson is a **durable artifact** in the workspace (in a folder, survives refresh,
  reopenable, provenance back to its source).
- The **ingest engine reliably** transposes a typical text-or-figure PDF into structured
  text (equations as LaTeX, code fenced, figures captioned); failures are **legible and
  retryable**, never a silent empty lesson.
- The whole thing **reuses Copilot + Shard** — no parallel engine, one authoring path.
- Over weeks, the user has a **library of interactive explanations** of the sources they
  chose to understand — the learning analogue of the research log.

## 12. Relationship to the broader research surface

The original ask was a *"research / educational"* surface. Tutor is the **educational**
half. This section records how it connects to the **research** half — not to spec that
surface here (it isn't scoped yet), but to keep the seams open while Tutor is designed.

**The ingest engine (§5, L0) is not Tutor's — it is the front door to research.** "Turn
any source into structured, agent-readable text" is a general capability; Tutor is its
*first consumer*, not its owner. The same ingested text feeds three consumers:

- **Tutor** — an interactive lesson (understand a source deeply). *This surface.*
- **The entity / claim layer** — fanned into the `records` enrichment pipeline
  (`internal/pipeline`) → entities, topics, claims; the source then surfaces in Entity
  Context / Insights (connect it to what the user already knows).
- **Securities research** — a filing, paper, or transcript attaches to the ticker/entity
  it's about, and its content becomes searchable evidence behind a thesis.

```
  ingest a source ──► understand it        (Tutor: interactive lesson)   ← educational
        │         ├─► take notes on it      (Pages)                       ← research
        │         └─► connect it            (Entities / claim layer)
        ▼
  understanding accretes ──► theses / strategies   (the research log, TRADING_PRD §2)
```

Three things already make these **one system, not two**:

- **Copilot is the shared engine.** Research = *help me investigate*; Tutor = *help me
  teach myself*. Same agent, different brief / surface kind (the `tutor` surface kind of
  D-C is one of several).
- **Shards + Pages are the shared output media.** Research produces notes and theses
  (Pages); Tutor produces interactive lessons (Shards). Both are durable workspace
  artifacts.
- **Entities are the shared connective layer.** A lesson on "momentum factor" links to the
  concept entity; a research note on AAPL links to the AAPL entity. The shard
  `anchors.json refs[]` is the seam (Tutor L5), and Entity Context is the hub.

All of it points at the same north star — the research log's *"here's why I believed
this"* (`TRADING_PRD.md` §2).

**The design lever that keeps this open:** build L0 as a **standalone ingest capability**,
triggered the moment a source lands, **not a Tutor-internal step.** If ingestion lives
inside Tutor, the research surface has to reach into Tutor later to reuse it; if it is a
first-class capability, both surfaces simply consume the same `artifact_text`. This costs
nothing extra now (the plan already runs L0 off artifact upload, not off a Tutor action)
and is the single most important thing to get right for the broader surface.

## 13. Build order (rev 2)

Sequenced so the assumption most likely to be wrong is tested first, and so nothing
downstream is built on an unproven foundation. **T0 is a gate, not a phase.**

### T0 — Equation fidelity spike *(gate: build or redesign)*

**The question:** can a vision model turn a formula crop into LaTeX that renders as the
same equation? Everything else in Tutor is assembly of things that already work; this is
the only genuinely unproven step, and §6 shows why it's decisive — that paper has **527
`isolate_formula` regions**. Math *is* the content.

What today's engine already gives, from the text layer:

```
Lmax = K2 −K1 −C
fT = ST −S0 −(ST −K)+ + C = K −S0 −(K −ST)+ + C
```

Legible, structurally lost: `K2` is really K₂, `ST` is S_T, `(ST −K)+` is a positive part.
A lesson rendering that verbatim looks broken to anyone who knows the material.

**Do:** crop ~20 `isolate_formula` regions (their bboxes are already stored), send each to
a vision model, render the LaTeX, compare side by side with the crop.
**Pass:** the large majority round-trip to the same equation, and failures are *detectable*
(low confidence / malformed LaTeX) rather than silently wrong — a confidently wrong formula
is worse than none, per §7.
**If it fails:** fall back to embedding the crop image (inert math, still faithful) and
re-scope §4b's "poke the equation" promise before building the surface, not after.

No surface, no schema, no plan object. An afternoon.

### T1 — The plan object and surface

The spine (§6): `study_plan` + `plan_item`, the `/tutor` destination, create-a-plan from an
ingested source, agent proposes from the outline, user edits/reorders/re-scopes, persists.
Reactivity rides the outbox sync spine — **not** REST-list + `reload()` (standing rule).
Ships useful on its own: a scoped, revisable reading plan over a source is already worth
having with zero lessons authored.

### T2 — Lesson authoring from an item

D-C (a `tutor` surface kind in `systemPrompt(surface)`), D-G (attach the item's scope to
the turn — its chunk/page span, *not* the paper stuffed into `surfaceContext`, which caps
at 1800 chars), and the gated `publish_app` review. Uses the existing Copilot + Shard path;
no forked authoring engine (§5's rule that must not bend).

### T3 — The teaching kit

D-D: `Quiz` / `Simulator` / `Glossary` / `StepThrough` / `Reveal` / `Figure` / `Formula` in
`@aladin/kit`. Deliberately *after* T2: author two or three real lessons freehand first and
let the primitives fall out of what the agent actually reaches for. Building the kit first
means guessing.

### T4 — Library and progress

Plan list, per-item status, re-author, "what I've learned." The accrual §2 promises.

**Explicitly not in this order:** connected exploration (v2, D-H) stays gated on shard
data-wiring being unpaused; multi-source plans; anything that forks the authoring path.

### What already exists (do not rebuild)

| need | status |
|---|---|
| PDF → pages/regions/chunks/outline | **shipped, running in prod** (`INGESTION_PRD`) |
| agent reads a document | **shipped** — `read_document`, `search_document`, outline (`workspace_tools.go:560`) |
| Shard runtime, build, sandbox, publish gate | **shipped** |
| Copilot authoring turns + tool trail | **shipped** |
| region bboxes for cropping | **shipped** — `artifact_regions.x0/y0/x1/y1` |
