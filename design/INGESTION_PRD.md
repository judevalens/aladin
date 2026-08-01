# PRD — Ingestion

> **Audience:** whoever builds and extends the ingestion engine.
> **Date:** 2026-07-31. Revision 1.
> **Status:** v1 building on `feat/ingestion-engine`.
>
> Companions: `RESEARCH_SURFACE_PRD.md` (§21 research artifacts, §3 #9 context artifact) ·
> `TRADING_PRD.md` (§2 the research log is the product) · `PIPELINE.md` (the *other*
> ingestion — external syncers into `records`; see §2 for why they stay separate).

---

## 0. The one-line version

**Ingestion makes a dropped document readable.** Text out, structure out, stop.

---

## 1. The gap, stated precisely

You can upload a PDF today. `POST /api/artifacts/upload` stores the bytes on disk and
writes an `artifacts` row. That is the entire lifecycle. Nothing reads it.

So a paper dropped into a research folder is:

- **unreadable** — no viewer, you'd open it outside the app, which defeats storing it here
- **unsearchable** — the command palette and page search can't see a word of it
- **invisible to agents** — `get_artifact` returns a filename, so §3 #9's "all of it becomes
  a context artifact other agents can run over" is false for exactly the material that
  matters most
- **un-navigable** — a 300-page book with no outline is a scroll bar

The Research Overview now lists a folder's material. Right now that list is honest about
*what* is there and silent about whether any of it is usable. It isn't.

## 2. What this is NOT  **LOCKED**

**Not the knowledge-graph pipeline, again.**

Aladin already had an ingestion engine: `records` → enrich → resolve entities → extract
claims → discourse → insights. It was built, it worked, and the trading pivot abandoned
the product it served. The claim layer was deleted outright this week.

The failure mode was not the code. It was that the pipeline tried to *understand*
documents before anything could *read* them. Entity resolution on a corpus you cannot
open is sophistication in the wrong place.

So the hard rule for v1:

> Ingestion extracts. It does not interpret.

Text and structure are facts about the file. Entities, topics, claims, and summaries are
opinions about its meaning, and every one of them is a separate, later, explicitly-chosen
step. If someone wants enrichment, it reads the ingested text — it does not get bolted
into the extractor.

**Also not** the external syncers. `internal/pipeline` pulls Bluesky/HN/Reddit into
`records` — machine-paced, feed-shaped, other people's content. This is user-paced,
document-shaped, material you deliberately put somewhere. Different trigger, different
lifetime, different table. Keeping them separate is what stops document ingestion from
inheriting a pipeline built for a product that no longer exists.

## 3. Scope: PDF first  **LOCKED**

| Input | v1 | Why |
|---|---|---|
| **PDF** | ✅ | Papers and books are PDFs. It's the format with real structure to recover, and the one with a navigation problem worth solving. |
| Pasted text | — | Already a page. Nothing to extract. |
| Links / YouTube | ✗ | Needs fetching and transcripts — different infrastructure, and the content isn't yours to store. Its own phase. |
| Voice | ✗ | Transcription is a distinct engine. Later. |
| Images / scans | ✗ | Needs OCR. **Detected and named** (§4), not silently empty. |

One format, done properly, beats four half-done. The engine's seam is
`Extract(path) → (text, sections)`, so a second format is a new extractor, not a new
pipeline.

## 4. Status is first-class  **LOCKED**

Extraction fails, and it fails in ways worth distinguishing. A spinner that never
resolves is the worst outcome, so every artifact carries a status:

| status | meaning |
|---|---|
| `pending` | queued, not started |
| `ingesting` | worker running |
| `ready` | text extracted |
| `unsupported` | the file is fine, we can't read it — **a scanned PDF with no text layer lands here** |
| `failed` | something broke; `error` says what |

**`unsupported` is the one that earns its place.** A scanned book yields zero characters
and that is not a crash — it's a document that needs OCR first. Reporting "0 pages of
text" as success is a lie, and reporting it as `failed` sends you debugging the wrong
thing. Naming it tells you the next action.

Every status is visible on the artifact. §2 of the research PRD bans data-slop, not
honesty about state.

## 5. What gets stored  **LOCKED**

Two tables, keyed by artifact:

- **`artifact_documents`** — one row per ingested artifact: status, error, page count,
  and the extracted text (per-page, so a section can be resolved to its text).
- **`artifact_sections`** — the outline: title, level, page, order. Populated from the
  PDF's own bookmarks when it has them.

**Sections come from the document, not from a heuristic.** If a PDF ships bookmarks, we
read them. If it doesn't, v1 leaves the outline empty rather than guessing — inventing a
structure and presenting it as the document's own is worse than no structure. Generating
one is a solved problem (`tools/pdftoc`, which drafts an outline from a printed contents
page and makes it *editable before it's applied*); porting that is the obvious next
phase, and it must keep the editable step.

**Text lives in Postgres, not the sync spine.** A book is megabytes; the tree frame
carries light fields only. The artifact's node frame carries *status*, the text is
fetched when you open the document. Same split the research extension row uses.

## 6. The surface  **LOCKED**

A **document viewer** in the work pane: outline sidebar, text pane, jump-to-section.
It is a new branch in the existing `activeArtifact.kind` switch — no new route, no rail
item, consistent with how the research bench added its views.

Status renders where the material is listed, so the Research Overview's Material section
tells you not just what's there but whether it's readable.

## 7. Sequencing

- **v1 (this branch)** — the engine, the two tables, PDF text + bookmark outline, status,
  the viewer.
- **v1.1 (done)** — agents can read the corpus. `get_artifact` on an ingested file
  returns the outline plus the first pages of text, and `read_document(artifact_id,
  from_page, to_page)` reads a range. Text comes back with `[pN]` markers so a model can
  cite a page rather than gesture at the document, and both reads are size-bounded — an
  unbounded read is how one book eats a context window. An unreadable document reports
  *why* instead of looking empty, which is the difference between "I can't read this,
  it needs OCR" and a confident answer about nothing.
- **v2** — outline *generation* for PDFs without bookmarks (port `tools/pdftoc`, keeping
  the editable draft step); full-text search across ingested documents.
- **later, and only if wanted** — enrichment over ingested text. Separate step, separate
  decision, reading the output of this one.

## 8. Open

- **Chunking for retrieval.** Sections give natural boundaries; whether ingestion should
  emit chunks or leave that to a consumer is undecided. Don't guess before there's a
  consumer.
- **Re-ingestion.** Currently once per artifact. Re-running on a better extractor needs a
  version marker on `artifact_documents`.
- **Size ceiling.** No limit today. A 2,000-page scan will hurt; add one when it does.
