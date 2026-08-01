# PRD — Ingestion

> **Audience:** whoever builds and extends the ingestion engine.
> **Date:** 2026-08-01. Revision 2 — adds Part II: structure and semantics (§9–§13).
> Revision 1 (2026-07-31) defined extraction and the status model; those still stand.
> **Status:** v1 shipped on `feat/ingestion-engine`. Part II is designed, not built.
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

The rule that came out of that, and the amendment rev 2 makes to it:

> **v1:** Ingestion extracts. It does not interpret.
>
> **rev 2:** Interpretation is allowed when it is **derived, disposable, anchored to the
> source, and never the thing you query instead of the text.**

The v1 rule was a good overcorrection and too strict to survive contact with a 400-page
book: without *some* map, a reader can only page through. What actually went wrong before
was not interpretation — it was interpretation that became a **truth store**: a global
graph you queried *instead of* the sources, that could drift from them, and that nothing
could re-derive cheaply.

So Part II adds a concept graph, and every clause of the amended rule is load-bearing:

- **derived** — recomputable from the text at any time
- **disposable** — throw it away and nothing else breaks
- **anchored** — every concept points at the chunks it came from, so any claim is one hop
  from its evidence
- **never authoritative** — the text stays the source of truth; the graph is an index

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


---

# PART II — Structure and semantics

> Designed, not built. Part I made a document *readable*; this makes it *navigable* —
> the difference between having the text and being able to find your way around it.

## 9. The problem  **LOCKED**

**Semantic boundaries in a document that doesn't annotate them.**

Some PDFs carry an outline; §5 reads it. Most don't, and then there is nothing to chunk
on. Fixed-size chunking cuts mid-argument, and a reader — human or agent — with no map
can only go page by page.

Two things were measured on 2026-08-01 rather than assumed:

1. **The text layer has no typography.** `ledongthuc/pdf` declares `Text.FontSize` and
   never assigns it — verified against a real upload *and* against a fixture with 26pt
   headings over 11pt body. Both report `0.0`. So the strongest inferred boundary signal
   is unavailable in the current Go stack. Not a document problem; a library ceiling.
2. **Scans are a dead end.** They land in `unsupported` and stop there.

Those are the same problem wearing two hats: **layout is a visual property, and we are
only looking at text.**

## 10. The pipeline  **LOCKED**

Cheapest signal first; the LLM runs last, on the smallest possible input. That ordering
is what makes this affordable on a book rather than a pamphlet.

| # | stage | answers | cost |
|---|---|---|---|
| 1 | **Layout segmentation** | *where things are* — title / heading / paragraph / table / figure boxes, per page | small local model, ~100ms/page, no per-token cost |
| 2 | **Structure assembly** | *the tree* — headings + reading order → chapter → section → block | deterministic |
| 3 | **Embeddings** | *retrieval, and the boundaries layout couldn't see* | one embed per leaf chunk |
| 4 | **LLM** | *names and concepts* | bounded windows only, never the whole document |

**Why segmentation and not better text extraction.** It recovers the typography we can't
get from the text layer, and it works identically on a scan — the same boxes that say
"heading" tell OCR what to read. One path for born-digital and scanned, instead of a
pipeline and a dead end.

**Why embeddings do double duty.** The vectors needed for semantic retrieval are the same
vectors that reveal topic shifts: embed adjacent windows, and a boundary is where
similarity drops. One cost, two uses — and it is the only signal here that finds a
boundary a document merely *implies*. Segmentation finds the ones it *shows*.

**Why the LLM is last and small.** It is an excellent labeller and a poor scanner. By the
time it runs, stages 1–3 have cut the document into regions that fit, so it names regions
and lifts concepts instead of trying to read a book.

## 11. Chunks are a TREE, not a partition  **LOCKED**

Recursion splits at the strongest boundary available; if the pieces are still too large it
recurses with the next signal down. **Keep the internal nodes.** A chapter is a chunk
*and* contains chunks.

That buys multi-resolution retrieval: match coarse for *"what is this about"*, fine for
*"what exactly does it claim"*. Flattening to leaves throws away the structure that was
expensive to recover.

## 12. The concept graph  **LOCKED**

Small, high-level, and deliberately incomplete — the embeddings carry the detail. A graph
trying to be exhaustive is just a worse index.

```
artifact_chunks    (artifact_id, parent_id, ordinal, page_from, page_to, text, embedding)
artifact_concepts  (artifact_id, name, kind, gist)
concept_edges      (from_concept, to_concept, relation)
concept_chunks     (concept → chunk)        ← the anchors; without these it is a summary
```

**Concept dedup is WITHIN one document.** Merging "risk premium" and "compensation for
risk" inside one author's vocabulary is bounded and cheap. Merging them *across the
corpus* is the global entity resolution that sank the last knowledge graph. That line is
the whole difference, and it is not a matter of degree.

**The agent's path becomes:** map → concept → anchored chunks → read. Bounded at every
step, and citable at the end.

## 13. Where it runs  **LOCKED** — a script Go invokes, not a sidecar

**Go shells out to a Python script per document.** Not a persistent service.

Rasterizing needs MuPDF/pdfium and the document-AI libraries are all Python, so *some*
Python is unavoidable. The question was only whether it runs as a long-lived HTTP service
or as a subprocess, and for this workload it's a subprocess:

- Ingestion is a **batch job in a worker**, not a request path. Nothing is waiting on it,
  so process startup is not on anyone's critical path.
- A sidecar is another thing to start, monitor, and forget to start. The `cmd/worker`
  incident on 2026-08-01 — a whole feature silently doing nothing because a process
  wasn't running — is the argument against adding more of them.
- A subprocess has function-call semantics: it runs, it returns, it dies. No port, no
  health check, no lifecycle.

**One invocation per DOCUMENT, never per page.** This is the detail that makes it work:
importing torch and loading a layout model costs seconds, which is fatal amortised over a
page and irrelevant amortised over a book. It also matches the sweeper, which already
claims and processes a document as a unit.

**The contract**, kept deliberately dumb:

```
in:   pdf path (argv)
out:  JSON on stdout   — pages, regions, structure
      diagnostics on stderr
      non-zero exit = failure
```

A crash becomes `status='failed'` with stderr as the error, which is exactly the status
model §4 already defines — a Python traceback surfaces the same way a malformed PDF does.
`exec.CommandContext` carries the timeout, because a hung subprocess must not wedge the
worker.

**The cost, stated honestly.** `backend_v2/Dockerfile` is currently three static
`CGO_ENABLED=0` binaries on plain alpine, and it is shared by api/worker/mcp. Putting
Python and a layout model inside it makes all three images fat for one service's benefit.
Locally this costs nothing; in prod it's a real regression.

**The escape hatch is the seam.** Go calls an interface — `Segment(ctx, path) (Layout,
error)` — with a subprocess implementation behind it. If the image size ever actually
hurts, an HTTP implementation of that same interface is one file, and nothing above it
changes. Choose the simple thing now; keep the swap cheap.

Go keeps persistence, status, and the sync frames either way. The Python is a function:
bytes in, structure out, no state.

## 13b. The layout model — measured, not guessed  **LOCKED**

Chosen: **DocLayout-YOLO** (`juliozhao/DocLayout-YOLO-DocStructBench`), run on CPU.

Measured 2026-08-01 against the real corpus — a 280-page MIT quant-finance thesis
already in `uploads/`:

| | |
|---|---|
| model load | **2.1s**, once per document |
| inference | **~78ms/page** on the M4 Pro GPU (MPS); ~345ms on CPU |
| render (PyMuPDF, 144dpi) | ~41ms/page |
| **a 280-page book** | **~34s** end to end, once, in a background worker |
| classes | `title · plain text · abandon · figure · figure_caption · table · table_caption · table_footnote · isolate_formula · formula_caption` |

**Tuning, measured on the real document (M4 Pro, 14 cores, 48GB):**

| variant | ms/page | note |
|---|---|---|
| cpu, single | 345 | the naive first run |
| **mps, single** | **78** | **4.4× — the whole win is one device flag** |
| mps, batch 4 / 8 | 113 / 98 | batching is SLOWER on MPS; don't |
| cpu, batch 8 | 433 | worse still |
| mps + `half=True` | 75 | ~4%, same regions — marginal |

Render DPI, holding `imgsz=1024`: **144dpi is the floor that keeps accuracy.** 96dpi saves
8ms but drops 13 regions/page to 10 — a 23% loss. 200dpi costs 42% more for no extra
regions. Render at 144 and let the model resize.

**Pick the device at runtime**, don't hardcode — `mps` where it exists, `cpu` otherwise.

**These numbers were measured NATIVELY on macOS, not in a container, and that is not an
incidental detail: Docker on macOS has no Metal passthrough.** Containers run in a Linux
VM, so MPS is unreachable from any container on this machine — and a containerised CPU run
is *slower* than the 345ms native-CPU figure, being CPU under virtualisation.

| how the worker runs | device | 280-page book |
|---|---|---|
| `make worker-go` — native, the dev loop | **mps** | **~34s** |
| `aladin-prod-worker` container on this Mac | cpu in a VM | >112s |
| Linux host with a real GPU | cuda | untested |

So containerising the ingestion worker **forfeits the 4.4× entirely**.

**DECIDED (2026-08-01): the ingestion worker runs NATIVELY on macOS, in dev and in
whatever passes for prod here.** Aladin is a single-author personal workspace on one
machine; there is no fleet to schedule and no reason to give up a 4.4× to satisfy a
deployment shape nobody needs. That resolves three things at once:

- **§13 stands, unambiguously.** The Python is a subprocess of a native worker. No image
  question, so the "torch bloats an alpine image shared by api/mcp" cost simply doesn't
  arise.
- **MPS is the target, not a bonus.** ~34s for a 280-page book.
- **Run the model on every page** (§14's first open, now closed). At 34s a book, sampling
  pages to learn typographic conventions is complexity bought for nothing.

If Aladin ever runs on a Linux host, containerising is the normal thing to do there and
the GPU works fine — the constraint is macOS-specific, not architectural.

**Why the corpus forced this.** That thesis is an **OCR'd scan** — producer
`Adobe Acrobat 8.13 Paper Capture Plug-in`, every page a 3392×4416 PNG with an invisible
text layer. Its font metrics are OCR's *guesses*: sizes smear continuously across
11.5–12.4pt, and thresholding on size flagged **451 of 1796 lines (25%) as headings**.
Typography-based detection is not mis-tuned on this corpus, it is inapplicable.

The visual model handles precisely what defeated it. The MIT library stamp OCR'd as
`MASSACHUSETTS NS E OF TECHNOLOGY JUN 252008 LIBRARIES` and `AII4H` — the exact noise that
produced false headings — comes back classed **`abandon`**. Having a class for *"this is
page furniture, ignore it"* is worth as much as the heading detection.

**Two things that make it cheaper than expected:**

- **No rasterizing for scans.** The page image is already embedded in the file.
- **No OCR pass.** The words already exist in the text layer; they simply have no
  structure. Once a region is labelled, its text is pulled by bounding box
  (`page.get_textbox(rect)`). The model *only* segments.

**PyMuPDF is the extractor**, replacing `ledongthuc/pdf` in the Python path: it exposes
span-level `size`/`flags`/`bbox`, renders pages, and reads embedded images. One dependency
covers extraction, rasterizing, and text-to-region mapping.

**Known noise, to handle when building:** overlapping/duplicate boxes need
non-max-suppression, and low-confidence regions (<0.3) are mostly junk. Raise the floor and
dedup by IoU.

## 13c. Corpus check — does it hold beyond one document?  **measured 2026-08-01**

Five documents, four born-digital and one OCR'd scan:

| document | pages | kind | ms/page | `title` share | found |
|---|---|---|---|---|---|
| MIT quant thesis | 280 | **OCR'd scan** | 79 | 9% | `3.2.4 Trend-Based Regression`, `5.4 Summary of Empirical Analysis` |
| arXiv preprint | — | born-digital | 74 | 28% | `1 Introduction`, `2 Defining AI Runtime Infrastructure` |
| SSRN paper | 16 | born-digital | 74 | 28% | `2. HYPOTHESES`, `3.1. Momentum Strategies` |
| ACM article | 38 | born-digital | 70 | 5% | `1.2 Terminology`, `3 DEDUCTIVE KNOWLEDGE` |
| journal article | 4 | born-digital | 115 | 8% | `Actors: A Model of Concurrent Computation`, `References` |

**The result that matters: one pipeline covers both formats.** The 280-page scan gave up
real numbered section headings — the thing font-size thresholding got 25% wrong on — at
the same cost per page as a born-digital paper. Timing is 70–115 ms/page throughout, with
no format-dependent blowup.

`abandon` scales with publisher furniture, as intended: 40 regions on the ACM paper (DOI
lines, badges, running heads), 14 on arXiv, 8 on the thesis.

**Two things NOT clean, to handle when building:**

- **Title share varies 5%–28% across the corpus.** Boxes were rendered onto the pages and
  eyeballed (`tools/doclayout/annotate.py`) — the arXiv and SSRN reds looked like genuine
  headings, so the spread is most likely subsection density rather than the model catching
  bold run-in text. That is a visual spot-check on two pages, **not** a measured
  precision/recall, so don't treat title density as a load-bearing segmentation signal
  without counting properly first.
- **False positives are real but cheap to filter.** The ACM paper labelled
  `Latest updates: https://dl.acm.org/doi/…` a title; the journal produced the fragment
  `BookRe`. A URL rule and a minimum-length rule remove both without touching anything
  genuine.

## 13d. What accuracy is actually needed  **LOCKED**

**~85% on layout is the target. Chasing higher is wasted money.**

The consumer is a language model, not a parser, and that changes what an error costs. A
missed heading still gets read. A figure box that swallows two charts still yields usable
concepts. Layout errors **degrade gracefully** here in a way they never would in a
deterministic pipeline, so the marginal value of 95% over 85% is small and the marginal
cost is large.

**But the errors are not equally cheap, and the budget belongs on one side:**

| error | cost | why |
|---|---|---|
| **boundary** — a chunk spans two topics | cheap | concepts get muddier; retrieval still lands, the LLM still reads it |
| **anchor** — a region maps to the wrong page | **expensive** | a citation points at something false, and a confident wrong citation is worse than none |

Boxes may be approximate. **The page they resolve to may not.** Anchoring is a bounding-box
lookup into text that already exists (§13b), so it is deterministic — keep it that way, and
never infer a page number.

The corollary is §14's remaining open: at 85% you *will* meet the 15%, so the structure has
to be **inspectable and correctable**. `tools/pdftoc` already encodes that instinct — draft,
let a human fix it, then apply.

## 14. Open

- **Are inferred boundaries auto-applied or reviewable?** `tools/pdftoc` deliberately makes
  a drafted outline editable before it touches the PDF, and that instinct was right.
  Silent structure you can't correct is the failure mode.
- **When does the concept pass run** — on ingest, or on demand? It is N LLM calls per
  document, so this is a cost decision, not a technical one.
- **Multi-column and tables.** Segmentation gives boxes; reading order across columns is a
  separate problem and tables are not prose.
- **Re-ingestion.** A better layout model invalidates chunk boundaries, which invalidates
  embeddings and anchors. `extractor` on `artifact_documents` is the version marker; the
  cascade needs one too.
