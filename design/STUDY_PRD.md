# PRD — Study (a surface that carries the method of self-learning)

> **Status:** draft — **rev 1 (2026-08-25)**. Not locked. Nothing here is settled until the
> owner marks it so.
>
> **Audience:** whoever builds the next slice of Aladin's learning surface — and the owner,
> deciding what this is before any of it is built.
>
> **Companions:** `TUTOR_PRD.md` rev 3 (the learning copilot — this document *contains* it,
> §10) · `RESEARCH_SURFACE_PRD.md` (the container/control-plane precedent this copies) ·
> the board handoff + `src/modules/board/` (the plane, shipped and multiplayer) ·
> `UI_ARCHITECTURE.md` (tokens, authoritative for styling).
>
> **What changed since TUTOR_PRD rev 3:** the canvas is no longer deferred — the board
> shipped (2026-08-20..24: live doc windows, excerpts with `(artifactId, page)` cites, ink,
> tasks, cards, paged-worksheet-capable engine, multiplayer rooms). Tutor §12a deferred the
> excerpt→source anchor because it had "nowhere to live"; it lives on the board as shape
> props today. That unblocks the frame this document states.

---

## 1. What this is

**A self-learner has no teacher. This surface is the part of teaching that isn't the
material: structure, sequence, questions, feedback, and memory.** The source still
teaches (Tutor's rule, kept). Aladin carries the *method* — so studying well stops being
discipline and becomes the path of least resistance.

The unit is a **study**: a thing you are teaching yourself. Options math. Kotlin
coroutines. The Kalman filter. A study owns its materials, its plan, its workspaces, its
open questions, and its history. Board, paper and log are **modes a study opens**, not
destinations the learner must remember to connect.

This is the research bench's move applied to learning: the research folder made "a
strategy under investigation" a first-class container with a typed Overview; the study
does the same for "a subject under self-instruction."

## 2. The loop (what a teacher does, as furniture)

Six stations. The surface's job is to make each one one tap from wherever you are, and to
make "where was I?" always answerable.

| station | what a teacher provides | what the surface provides | state |
|---|---|---|---|
| **Gather** | the reading list | dump anything — PDF, video link, page — it becomes a citable artifact in the study | ingest shipped (PDF); video = §7 |
| **Orient** | the syllabus | the **plan** (Tutor §6, unchanged): ordered items scoped to real spans, co-authored, revisable forever | net-new (Tutor's one net-new object) |
| **Engage** | "read this actively" | the **board**: source windows, excerpts-with-cites, ink. Capture is a verb on the reader, not a paste | board shipped; capture + wormhole net-new |
| **Practice** | problem sets, checked | **paper**: the paged board regime for working exercises in ink; the copilot can read the worked page and respond | engine ready; regime net-new |
| **Test** | the quiz | **quiz-me**: the copilot reads the board/worksheets/transcripts and asks; retrieval, not recognition | MCP read shipped; write + flow net-new |
| **Consolidate** | "summarize; we'll revisit" | **distill** to the study's log; **open questions** that resurface instead of expiring | net-new |

Two design rules make it a *surface* rather than a checklist:

1. **The loop lives in the furniture.** On a doc window: `Excerpt · Work this · Quiz me ·
   I'm stuck here`. Adjacent affordances, one tap, never leaving the material.
2. **"Where was I?" is always answerable.** Opening a study shows: the plan with position,
   every material's last location (per-window page / timestamp — already how windows
   work), open questions, and what's due to resurface. A teacher's "last time we…" as a
   screen.

## 3. The study container

A tree node kind `study` (exactly as `research` is a kind), whose **Overview is a typed
control plane**, not prose:

- the **plan** (Tutor §6's object, rendered here);
- **materials** with per-material position ("§4.2, p. 94" / "lecture 3, 41:20") and
  ingest status;
- **open questions** (§5), newest and oldest both visible — the oldest is the point;
- **continue** — one affordance that reopens exactly where you stopped: the board at its
  camera, the worksheet at its page, the reader at its position.

Contained like the research folder: boards, worksheets, notes, aids created inside a study
live in its subtree. No new rail item; a study is reached the way research is.

## 4. The modes (one ink engine, three regimes)

- **Board — the map.** The infinite plane as shipped. The study's spatial memory:
  materials as live windows, excerpts as evidence, ink as headings and derivation margins.
- **Paper — the worksheet.** The same board engine, paged: fixed-width column, vertical
  scroll, page-rule background, pencil-first, camera clamped (`cameraOptions.constraints`
  + a background layer + prefs — configuration, not a new engine). Spawned from a source
  window ("Work this") and **born citing its exercise** (`source, §4.7, p. 96`). A
  worksheet is an artifact; the parent board shows it as a live window — scratch work
  stays on the map (board rule 2 already guarantees this shape).
- **Log — the ledger.** The study's BlockNote note(s). Prose is the compression test;
  the log is where a session's understanding survives. Fed by **distill** (§6), written
  by the learner.

Implementation stance: `paper` is a property of a board artifact (`plane | paged`), not a
new artifact kind — rooms, picker, MCP, sync are unchanged. A third kind would grow a case
in every downstream system for a camera setting.

## 5. Open questions — confusion as an object

The self-learner's silent failure is the question nobody wrote down. `I'm stuck here` (on
a window, an excerpt, a worksheet page) creates an **open question**: the text of the
confusion + a cite to where it arose. It appears on the study Overview and — this is the
teacher-shaped part — **refuses to expire quietly**: the Overview orders open questions
oldest-first, and quiz-me draws from them before anything else. Closing one requires an
answer (prose, an aid, or "no longer relevant" — explicitly).

Thin model: an artifact kind is overkill; a `study_questions` table (`id, study_id, text,
cite_artifact_id, cite_locator, status, created_at, closed_at, answer_ref`) or — v0 —
tasks on the study's question board. Decide at build time; the *behaviour* (visible,
oldest-first, feeds quiz-me, explicit close) is the product.

## 6. The connective verbs (net-new, and the actual product)

| verb | from | does |
|---|---|---|
| **Excerpt** | reader / doc window selection | creates an excerpt with `(artifactId, locator)` cite where you are — on the active board, or held for placement |
| **Wormhole** | any cite chip (board excerpt, log quote, worksheet header, open question) | opens the source **at the cited locator** — page or timestamp — and highlights. The trust move: every claim one tap from its evidence |
| **Work this** | a source window | spawns a cited worksheet, opens it paged, pencil ready |
| **Quiz me** | board / worksheet / study | copilot reads the actual objects (MCP board read exists; add worksheet + question read) and asks; wrong answers can open the source at the relevant span (pointer over paraphrase — Tutor §7 kept) |
| **I'm stuck** | anywhere | open question with cite (§5) |
| **Distill** | board / session | copilot drafts a log entry from the board's structure — claims as prose, excerpts as quoted blocks **with cites carried over**, unchecked tasks as open items. The learner edits; correcting the draft is itself retrieval practice |

**Locator generalization** (one decision that keeps video cheap): a cite is
`(artifactId, locator)` where locator is `page:N` today and `t:SECONDS` when video lands
(§7). Everything downstream (wormhole, distill, quiz-me) treats locators opaquely.

## 7. Video (second source type, not v1)

A lecture is a document whose pages are timestamps. Dump YouTube links → link artifacts +
board video windows (custom shape over the YouTube IFrame API — the stock tldraw embed
can't seek or report time); playback position is per-window view state; **clip** = excerpt
cited `t:2480`, prefilled from the transcript when captions are ingestable (best-effort —
caption fetching is semi-official territory); wormhole seeks. iPad needs
`allowsInlineMediaPlayback` on the WKWebView config. The iframe-inside-a-shape case is the
old aid-object spike, finally with a reason to be proven.

## 8. Resurfacing (the memory a learner doesn't have)

No scheduler UI, no due counts, no streaks (see non-goals). Resurfacing is two quiet
behaviours: the Overview's **oldest-open-question-first** ordering, and quiz-me's source
priority: open questions → cards/excerpts *not touched recently* (room `lastChangedClock`
— data multiplayer already maintains) → the rest. The learner never manages a queue; the
surface just keeps asking about the stale parts. Board rule 5 (no review runner **on the
plane**) stands — resurfacing lives in the study, not on the board.

## 9. Non-goals

- **Teaching.** The source teaches. Generated restatement stays a non-goal (Tutor §9);
  the default answer to "explain" remains a pointer to where it's explained.
- **Gamification.** No streaks, XP, or guilt mechanics. The accountability object is the
  open question, which is real, not synthetic.
- **A scheduler.** No SRS algorithm, no due-date math in v1. §8 is ordering, not Anki.
- **Multi-user courses / sharing.** One learner. Multiplayer is device-sync, not classroom.
- **A new rail item.** Studies live in the tree like research folders.

## 10. Relationship to TUTOR_PRD

Tutor rev 3 is the **agent half** of this surface and survives intact: the plan object
(§6 there = Orient here), aids-on-request, pointer-over-paraphrase, the trust guarantee,
the ingest engine. What this document changes: §12a's "canvas is deferred" is overtaken by
events (the board shipped; the excerpt anchor lives as shape props); the container is a
`study` node rather than a template folder (§12b's primitives still apply); and the
surface gains the loop verbs (§6 here) Tutor had no home for. When this locks, fold or
supersede — don't maintain two.

## 11. Build order (each slice usable alone)

- **S-A Wormhole.** Cite chips open the source at the locator (board excerpt → reader at
  page; "Open source" learns the last inch). Smallest slice, biggest trust gain.
- **S-B Capture.** Selection → excerpt-with-cite from the reader/doc window. With S-A this
  closes read → evidence → back.
- **S-C Paper.** The paged regime + "Work this" spawn-with-cite.
- **S-D Study container.** `study` node kind + Overview control plane (plan rendered,
  materials with positions, continue). This is where Tutor's plan object gets built.
- **S-E Questions + quiz-me.** Open questions, MCP board/worksheet write for the copilot,
  quiz flow with pointer-on-miss, §8 ordering.
- **S-F Distill.** Board → log draft with cites.
- **S-G Video.** §7, behind the locator generalization decided in S-A.

## 12. Success criteria

- Sitting down to study starts with **one tap** (continue) and zero "what should I do"
  decisions — the plan answers it.
- Any claim on any surface reaches its source location in **one tap** (wormhole).
- A month later, the study's log + boards let you *find* what you learned — and the open
  questions show honestly what you never did.
- The copilot can run a quiz **entirely from your own objects** (excerpts, ink labels,
  worksheets, open questions) with zero invented content.
- Nothing in the loop required leaving the material to go operate study software.
