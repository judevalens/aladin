> **Status:** draft PRD (2026-07-24). Not locked. Product intent for the next Aladin
> surface — provisional name **"Learn."** Companion implementation plan:
> `~/.claude/plans/learn-surface.md`. Reads on top of `TRADING_PRD.md` (north star),
> `SHARD_MODEL.md` (the output medium), `DESIGN_SPEC.md` (tokens/components).

# PRD — Learn (source → interactive teaching Shard)

> Drop in a source you want to understand — a white paper, a chapter, a talk — and get
> back a **super-interactive lesson you can explore**, authored by Copilot and rendered
> as a Shard. This document states the intent: what the surface is for, what must be
> true, and how it behaves. The name "Learn" is provisional (see D-F).

---

## 1. What this surface is

**Learn turns a source into an interactive lesson.** The user brings raw material they
want to internalize — a research PDF, an ebook chapter, an article, a YouTube talk — and
Aladin hands back a **teaching Shard**: a sandboxed, interactive React app that explains
the material in stages, checks understanding, and lets the user *manipulate* the ideas
(sliders, walkthroughs, quizzes, definitions on demand), rather than re-reading prose.

The flow, end to end:

```
  drop a source            Copilot authors                 a lesson you explore
  ──────────────           ───────────────                 ────────────────────
  PDF / URL / video   →   extract text  →   Lesson Author  →   Shard (interactive)
                          (net-new)          turn over            opens as a tab,
                                             Copilot + kit        lives in your library
```

Three moving parts, two of which **already exist**:

- **Copilot** authors the lesson. It already has the full Shard toolchain
  (`create_app → write_file → build_app → publish_app`) and is already prompted to build
  shards (`internal/service/copilot.go`). We give it a *teaching* brief and a source.
- **Shard** is the medium. Agent-authored multi-file React, esbuild-in-Go, sandboxed
  opaque-origin iframe, composed from `@aladin/kit`, rendered as a work-pane tab
  (`modules/doc-surface/`). No new runtime.
- **Source extraction** is the one net-new subsystem. Today an uploaded PDF is bytes on
  disk with **no text representation** — `get_artifact` returns empty content for `file`
  artifacts and they aren't searchable (`internal/service/artifacts.go:497`,
  `internal/service/search.go:169`). An agent cannot teach what it cannot read. **This
  is the unlock, and most of the real work.**

## 2. Why it matters (product thesis)

The `TRADING_PRD.md` §1 corollary is the whole justification: *"when in doubt, build the
thing that teaches you the domain."* A novice trader with a strong technical background
learns markets **through** the harness. The fastest way to metabolize a dense source —
a factor-model paper, a chapter on options greeks, a talk on market microstructure — is
not to re-read it but to **take it apart interactively**. Learn is that tool.

**This is not the "education layer" §1 rules out.** That exclusion is about onboarding,
guardrails, and hand-holding for *strangers whose judgment we can't model*. Learn is the
opposite: a **single-player instrument for the user's own understanding**, that works on
*any* source they choose, with zero curriculum we have to maintain. Self-first, exactly
as §1 demands.

**The lesson is a durable artifact, not a throwaway view.** Per the standing feedback
that primitives must tie back to the platform (not "just a pretty list"), a generated
lesson is a Shard artifact that lives in a folder, survives refresh, is reopenable, and
carries provenance back to its source. Over time the user accrues a **personal library
of interactive explanations of the things they chose to understand** — which is the same
"nothing is thrown away" bet the research log makes (`TRADING_PRD.md` §2), applied to
learning instead of trades.

## 3. The core loop (what the user actually does)

1. **Bring a source.** Drop a PDF, paste an article URL, paste a YouTube link, or pick
   an artifact already in the workspace.
2. **Aladin extracts it.** Text (and, where present, structure — headings, sections) is
   pulled out, persisted, and made agent-readable. Legible progress; legible failure.
3. **Copilot authors the lesson.** A *Lesson Author* turn runs: Copilot reads the source
   and builds an interactive teaching Shard using the teaching kit primitives. The user
   **watches it build** in the Copilot dock (the tool trail already streams
   `create_app`/`write_file`/`build`).
4. **The user approves publish.** `publish_app` is gated (`copilot.go:54`) — the finished
   lesson holds for one click before it goes live. (This is the natural review gate.)
5. **The lesson opens and is explored.** It renders as a work-pane tab; the user reads,
   manipulates, and checks understanding. It's now in their Learn library.

## 4. What "super-interactive" and "explore" mean (the bar)

The user's phrase was *"a super interactive shard that teaches and lets you explore."*
Concretely, a good lesson is **not** a styled document. It should reliably contain:

- **Staged explanation** — the idea unfolds in sections/steps (hash-routed or stepper),
  not one wall of text. The user controls pace.
- **At least one manipulable model** — a `Simulator`: parameters the user changes and a
  result that recomputes live (e.g. move volatility, watch the option price move). This
  is the payoff that a PDF cannot give.
- **Check-for-understanding** — inline `Quiz` / self-test with feedback, so the user
  learns actively.
- **Definitions on demand** — `Glossary`/`Define`: hover or tap a term for its meaning,
  so the lesson stays readable without dumbing down.
- **Grounding** — key claims trace back to the source (a section reference / quote), so
  the user trusts it and can go deeper. **The lesson transforms the source; it must not
  silently invent beyond it** (see §7).

"Explore" has two tiers:

- **v1 — self-contained interactivity.** Everything above, inside the shard's sandbox.
  Sliders, quizzes, expandable detail, cross-section navigation. No live Aladin data.
- **v2 — connected exploration.** Links out to the entity/ticker a lesson touches, and
  live Aladin data bound into the lesson via the kit data bridge (`useNode`/`useNodes`).
  **Deferred** — the shard data bridge is a deliberate stub today (host answers only
  `ping`; `doc-surface-ui.tsx:136`) and shard data-wiring is paused until the data model
  settles. Do not build the KG-bound lesson first.

## 5. Architecture (reuse, don't reinvent)

```
  L0  Extraction        upload/URL/video → extracted_text        [net-new]
        │               (async, via the existing asynq pipeline + reaper)
        ▼
  L1  Agent-readable    get_artifact returns text · search indexes files
        │               source artifact ── attached to ──┐
        ▼                                                 ▼
  L2  Lesson Author     a Copilot turn: teaching system prompt + source artifact
        │               → create_app → write_file (teaching kit) → preview/verify
        │               → build_app → publish_app (GATED: user approves)
        ▼
  L3  The lesson        a Shard artifact (kind:"app") with source_artifact_id
                        rendered in the work pane · listed in the Learn library
```

The only new *engine* is L0. L1 is a small wiring change (`get_artifact` +
`search` learn to read extracted text). L2 is a new system prompt + kit primitives + a
way to attach a specific source to a turn. L3 is a surface (nav destination + explorer +
work-pane reuse).

### The rule that must not bend

**One authoring path.** A lesson is authored by the *same* Copilot + Shard toolchain the
user already has, not a parallel generator. If Learn ever forks its own agent/runtime,
we maintain two of everything and they drift. Learn is an **orchestration and a brief**
over the existing engine, plus the extraction it was always missing.

## 6. Data model

Thin, in the spirit of `TRADING_PRD.md` §2 — add the minimum, throw nothing away.

### Extracted source text (net-new)
Uploaded/linked artifacts gain a text representation an agent can read.

| field | notes |
|---|---|
| `artifact_id` | the `file` / `link` artifact this text belongs to |
| `text` | extracted plain text (+ lightweight structure markers where cheap: headings) |
| `extractor` | which path produced it (`pdf`, `article`, `transcript`) — for reproducibility |
| `status` | `pending \| ok \| failed` (+ error) — extraction is async and can fail legibly |
| `extracted_at` | provenance |

*(Shape TBD — a dedicated `artifact_text` table vs. a column on the artifact vs. folding
into the `records` enrichment pipeline. See D-B.)*

`get_artifact` returns this text for `file`/`link` kinds (today it returns `""`); the
`search` federation learns to index it (today files are excluded, `search.go:169`).

### Lesson provenance (net-new, thin)
A lesson **is** a Shard artifact (`kind:"app"`) — no new heavy table. It gains:

| field | notes |
|---|---|
| `source_artifact_id` | the source it was generated from — the provenance FK |
| (marker) | distinguishes a "lesson" shard from a hand-built shard (a metadata flag) |

Optional, later: the shard's `anchors.json` `refs[]` carry entity ids the lesson teaches
(the manifest already supports this) — the seam for v2 connected exploration.

## 7. The trust guarantee (and how it differs from Entity Context)

Entity Context has a **verbatim** guarantee — it never rewrites the user's material.
Learn is the *opposite* by design: a lesson **transforms** the source into an
explanation. That makes grounding the safeguard instead of verbatim-ness:

- The lesson must **stay faithful to the source** and **cite back to it** for its key
  claims (section/quote references), so the user can verify and go deeper.
- It must not **silently fabricate** facts, figures, or data the source doesn't contain.
  Where the model adds scaffolding (analogies, worked examples), that's fine and useful —
  but numeric claims and factual assertions trace to the material.
- When the source is thin or extraction was poor, the lesson **says so** rather than
  confabulating a confident lesson over missing text.

## 8. Behavior & edge cases the surface must handle

- **Extraction failure** — encrypted/scanned/image-only PDF, a URL that 404s or is
  paywalled, a video with no transcript. Fail **legibly** with a real message and a
  retry; never a silent empty lesson. (Ties to the standing "no silent caps" instinct.)
- **Huge source** — a 200-page book. v1 may cap/scope (e.g. "this chapter," or a
  section picker) — and must **say what it scoped**, not silently truncate.
- **Generation is slow** — authoring a shard is many tool calls. The dock's existing tool
  trail + status affordances carry this; the Learn surface shows a real "building your
  lesson…" state, not a spinner-as-decoration.
- **Publish approval** — the finished lesson holds at `publish_app` for one click. If the
  user walks away, the draft is recoverable (the shard exists on disk pre-publish).
- **Build/mount failure** — a shard can build yet not mount; `publish_app` already gates
  on `verifyMount` and refuses to publish a broken lesson. Surface that state honestly.
- **Empty library (first run)** — a real first-run state that invites the first source,
  not a blank void.
- **Re-generation** — the user dislikes the lesson and wants another pass. A lesson
  should be regenerable from the same source (a new shard, or a new draft), keeping the
  source attachment.

## 9. Explicit non-goals

- **Not a curriculum / MOOC for strangers.** No authored course content we maintain, no
  onboarding layer. The user supplies the source; the surface is single-player
  (`TRADING_PRD.md` §1, §3).
- **Not a document reader.** The output is an interactive lesson, not a PDF/EPUB viewer.
  If the user just wants to read the source, that's the existing artifact, not Learn.
- **Not a new agent or runtime.** Reuses Copilot + Shard. No forked authoring path (§5).
- **Not a verbatim surface.** Learn transforms; grounding (not verbatim-ness) is the
  guarantee (§7). Don't import Entity Context's rules here.
- **Not live-data-bound in v1.** The kit data bridge stays stubbed; connected
  exploration is v2 (§4), gated on shard data-wiring being unpaused.
- **Not building our own PDF/ML stack.** Extraction should lean on a proven library or a
  thin sidecar, not a from-scratch parser (D-A).

## 10. Open decisions

| # | Decision | Recommendation |
|---|---|---|
| **D-A** | **Extraction engine** — Go PDF lib, a thin extraction sidecar, or model-native PDF (hand the bytes to Claude as a document block)? | **Extract-at-ingest into reusable text** as the backbone (benefits search, records, `get_artifact` — not just Learn). Pragmatic library/sidecar for PDF + article. **Model-native PDF (figures/equations fidelity) is a v2 upgrade** for figure-heavy papers. |
| **D-B** | **Where extracted text lives** — new `artifact_text` table, a column, or the `records` enrichment pipeline? | Persist against the artifact (dedicated table, clean status/lifecycle) **and** optionally fan into the `records` pipeline (`internal/pipeline`) so extracted sources also enrich the entity layer. Reuse asynq + reaper for reliability. |
| **D-C** | **Generation driver** — a general Copilot capability, or a dedicated "Lesson Author" turn? | **Dedicated Lesson-Author system prompt + a surface-kicked turn**, streamed in the existing dock. Add a `learn` surface kind alongside the current `systemPrompt(surface)` switch (`copilot.go:824`). |
| **D-D** | **Teaching kit primitives** — build `Quiz`/`Simulator`/`Glossary`/`StepThrough`/`Reveal` into `@aladin/kit`, or let the agent freehand each lesson? | **Build them into the kit.** Consistent, reliable output beats brittle freehand; the kit is embedded Go (`docsurface/kit.tsx`) and the authoring guide teaches their use. This is what makes generation trustworthy. |
| **D-E** | **v1 source types** | **PDF + article URL first** (cleanest extraction). **YouTube/transcript deferred** to a fast-follow (external dependency, availability/ToS risk). Voice notes (already stored, never transcribed) are a natural later add. |
| **D-F** | **Surface name** | "Learn" (provisional). Alternatives: "Study," "Tutor." One-word nav destination to match Signals/Insights/Entities. |
| **D-G** | **Source→turn attachment** — copilot turns are text-only today (no attachment path). | **Explicit artifact reference on the turn** (pass `source_artifact_id`; the agent reads it via `get_artifact`). Don't stuff the whole paper into the system prompt (the `surfaceContext` block caps at 1800 chars, `copilot.go:871`). |
| **D-H** | **How much "explore" in v1** | **Self-contained interactivity only.** Live entity/ticker links + data bridge deferred to v2 (§4). |

## 11. Success criteria

- A user drops a white paper and, in one guided flow, ends up with a **published,
  interactive lesson** that opens as a tab and that they can actually learn from — staged
  explanation, at least one manipulable model, a check-for-understanding, definitions on
  demand — **grounded in the source**.
- The lesson is a **durable artifact** in the workspace (in a folder, survives refresh,
  reopenable, provenance back to its source).
- **Extraction is reliable** for typical text PDFs and articles; failures are **legible
  and retryable**, never a silent empty lesson.
- The whole thing **reuses Copilot + Shard** — no parallel engine, one authoring path.
- Over weeks, the user has a **library of interactive explanations** of the sources they
  chose to understand — the learning analogue of the research log.
