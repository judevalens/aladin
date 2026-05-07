# Aladin Product Spec

## 1. Product Summary

Aladin is a terminal for signals and thoughts.

It ingests live data from generic sources, turns that data into structured signals, and connects those signals back to the user's ongoing areas of work. Instead of letting information disappear into feeds or notes go stale in archives, Aladin keeps thought alive by continuously relating new incoming evidence to prior user-organized sections and artifacts.

Aladin is designed for people whose work depends on maintaining continuity of interpretation across ongoing streams of information.

## 2. Core Product Thesis

Most tools handle either:
- incoming information
- stored notes and thoughts

Aladin handles the interaction between them.

The core promise is:

**New signals do not just arrive. They update what you were already working on.**

The sharpest expression of this promise is a **proactive research delta**: Aladin notices a meaningful change in an external source, compares it against the user's existing research context, and surfaces what is new, missing, or newly connected. A notification should not be a raw alert. It should be synthesized context.

The north-star example is a trending source item that produces a personalized gap or connection against the user's notes. The implementation-facing version of that loop lives in [`sys_design/TRENDING_POST_GAP_ANALYSIS.md`](../sys_design/TRENDING_POST_GAP_ANALYSIS.md).

## 3. Product Vision

Aladin is a living knowledge and thought workspace where:
- sources continuously bring in new information
- the system transforms incoming artifacts into signals
- users organize work into sections
- users capture concrete artifacts inside those sections
- the graph/context layer improves retrieval and reasoning across the whole workspace
- outputs compound into a durable system of thought

Aladin should feel like an always-on workspace for:
- monitoring
- synthesis
- reactivation of prior work
- progressive refinement of sections over time

## 4. Primary Product Concepts

### Section

A section is a user-created organizational container.

Examples:
- Rivian
- AI Supply Chain
- GTM Research
- Student Cohort Issues

A section exists for the user’s own organizational needs. It is how they group work, revisit topics, and maintain focus.

### Artifact

An artifact is a concrete captured or stored item.

Examples:
- markdown note
- link + summary
- voice note
- later: imported document, generated output, clip

Artifacts live inside sections from the user’s point of view.

From the product point of view, an artifact is one concrete unit in the workspace. Internally, Aladin keeps the shared artifact envelope lightweight and lets type-specific content live in modular slices. A page, link, voice note, uploaded file, or future artifact type can therefore have different storage and behavior while still appearing as one artifact in the workspace.

### Signal

A signal is a processed, meaningful event or surfaced unit derived from one or more artifacts.

Signals are what the system decides are worth surfacing. They may:
- support a section’s working thesis
- contradict a section’s assumptions
- update a section
- reveal a trend
- expose a new relationship
- indicate a change worth attention

A user may consider some signals to be “insights,” but `signal` is the core product term.

### Source

A configured live input stream that brings new information into the workspace.

Examples:
- Reddit subreddit
- Bluesky search or account
- news feed
- stock ticker feed
- internal chat stream

### Graph / Context Layer

A structured global workspace context layer that links artifacts, signals, entities, and relationships.

The graph is not scoped to a section. Sections are for user organization; the graph is for what the system understands across the whole workspace.

## 5. Primary User Jobs

Aladin helps a user:
- create sections to organize ongoing work
- capture thoughts before they disappear
- monitor live sources without losing context
- connect incoming signals to existing sections
- preserve and evolve lines of thought over time
- surface what changed and why it matters
- ask the system to further analyze relevant incoming signals
- build a durable body of structured thought instead of isolated notes and chats

## 6. Initial Use Cases

Aladin should support these patterns, even if product language stays generic:
- hobbyist trader or analyst tracking company or sector theses
- founder doing market and competitor research
- educator or operator managing many ongoing conversations and signals
- researcher maintaining evolving domain knowledge

These are not all separate products. They are variations of the same core workflow:
- many incoming inputs
- constrained attention
- evolving interpretation
- need for continuity over time

## 7. Product Scope for Now

The locked scope is:

### Capture
- markdown quick note
- link + description capture
- voice note capture

### Sources

Generic live sources where new data comes in.

Current examples:
- Reddit
- Bluesky

Future examples:
- news
- stock tickers
- internal messaging
- additional social/media sources

For third-party live sources, Aladin should treat raw provider content as processing input unless a provider policy and product need explicitly allow durable retention. The durable product value is the system's derived signal, summary, entities, graph context, and provenance link back to the original source.

### Signals

System-generated meaningful surfaced units derived from new source data.

### Sections

User-created containers for organizing work. Sections are the primary user-facing unit of organization.

### Artifacts

Concrete captured or stored items within sections.

Artifact metadata is shared across types, but artifact content is type-specific. Pages own markdown content, files own resource storage, and future link/voice/document types should keep their specialized state out of the shared artifact envelope.

### Graph / Context

A global graph-backed context layer that improves retrieval, linking, and future LLM pipelines across all sections.

### Specialist UI Surfaces

Use JS-based specialist surfaces where browser tooling is strongest, mounted into the Kotlin/Wasm shell.

Examples:
- markdown editor
- graph explorer
- mini React surfaces

Specialist surfaces should behave like input/rendering adapters. Core product behavior such as document sync, persistence, upload workflow, and artifact state belongs in Kotlin and backend services.

## 8. Core User Loop

1. User creates folders and sections to organize work.
2. User captures artifacts into sections.
3. Sources ingest new artifacts continuously.
4. Backend processes incoming artifacts and generates signals.
5. Signals are linked to relevant sections and surrounding context.
6. User sees what changed and what section it affects.
7. User updates section contents manually or lets the system analyze relevant signals further.
8. Updated sections and outputs become part of the ongoing context for future signals.

## 9. Product Behavior Principles

### 1. Signals over noise

Users should not have to sift raw streams unless they want to. The system should surface meaningful changes.

### 2. Sections stay alive

Sections should remain revisitable and evolvable over time.

### 3. New information should reactivate existing work

Incoming data should be evaluated against what the user already organized and cares about.

### 4. The system should support partial thought

Capture can be rough, incomplete, and fast. Structure can emerge later.

### 5. The graph is infrastructure, not decoration

The graph should improve context and reasoning, not merely visualize data.

### 6. Kotlin owns product logic

Frontend JS surfaces exist for rendering and specialist interactivity, not for core business logic.

## 10. UX and Information Architecture

The shell should use a 3-pane layout:

- **Pane 1**: app-wide navigation
- **Pane 2**: hierarchy and child resources for the selected top-level area
- **Pane 3**: active workspace or detail surface

### Primary navigation

Pane 1 should provide stable product navigation:
- Home
- Sections
- Signals
- Sources
- Graph

Optional later:
- Outputs

### Pane 2 behavior

Pane 2 is the hierarchy/index pane for the selected top-level area.

It should support:
- folders
- sections
- child resources within the selected area

Navigation inside pane 2 should work like a container browser:
- opening a folder replaces pane contents with that folder’s children
- breadcrumbs at the top show the current path
- clicking breadcrumbs navigates upward

### Pane 3 behavior

Pane 3 is the active workspace/detail surface.

It should show:
- selected section workspace
- selected signal detail
- selected source detail
- graph exploration
- specialist editor/graph surfaces when needed

Top-level folder navigation should not live in pane 3 by default.

## 11. Home Experience

Home should act as the operational overview of the workspace.

### Pane 2 on Home
Show:
- **Active Sections**
- most recently updated or most relevant sections

### Pane 3 on Home
Default to a **Daily Brief**:
- important recent signals
- which sections were affected
- suggested next actions
- calm summary rather than raw stream overload

The Home surface should communicate:
- what changed
- where it matters
- where the user might want to go next

## 12. Section Experience

Sections are the primary user workspace.

### Pane 2
Show:
- folders and sections
- hierarchical organization through folders
- breadcrumbs for navigation

### Pane 3
Show the selected section workspace:
- artifacts in the section
- signals relevant to the section
- graph-derived related context from the global KG
- later: prompts/lenses, summaries, outputs

This should feel like a living workspace, not a static folder.

## 13. Signals Experience

Signals are the system’s surfaced updates.

### Pane 2
Show:
- signal lists grouped by freshness, source, or section relevance

### Pane 3
Show selected signal detail:
- title
- concise explanation of why it matters
- linked evidence/artifacts
- relevant sections
- later: actions such as attach, save, dismiss, summarize

Signals should default to a **curated update card** style rather than raw evidence first.

## 14. Document / Page Editing

Pages are the first artifact type with a full editing loop.

The user experience should feel like a trustworthy document surface:
- initial content loads before the editor mounts
- typing should never reset or delete the live draft
- autosave should be quiet and visible through lightweight metadata
- stale saves should not overwrite newer document content
- uploads should produce durable resource URLs, not temporary browser blob URLs

The editor owns live draft text after mount. Kotlin owns page sync metadata such as load state, saved state, revision, upload state, and retry behavior. The backend owns durable content and rejects stale writes using a persisted page revision.

This keeps business logic out of the JS editor while still allowing Aladin to use best-in-class browser editing libraries.

## 15. Sources Experience

Sources remain a dedicated operational area.

### Pane 2
Show:
- configured source list

### Pane 3
Show:
- selected source detail
- source configuration
- health/status
- recent activity

Language should emphasize “live inputs” more than technical sync internals.

## 16. Graph Experience

Graph is a secondary analysis surface.

It is:
- workspace-wide
- cross-section
- contextual
- used for exploration and relationship discovery

It is not the default daily workflow surface.

The graph should help users discover:
- entities
- relationships
- cross-section overlaps
- contextual clusters

## 17. Capture UX

Capture should remain globally reachable.

Default capture modes:
- markdown note
- link + description
- voice note

Recommended shell pattern:
- **small always-visible capture control**
- **command palette for power users**

Captured artifacts should be easy to place into a section at creation time.

## 18. Visual Direction

Target:
- **calm intelligence**
- Linear / Notion-like clarity
- serious, editorial, legible, low-noise

Avoid:
- dashboard clutter
- loud terminal styling
- feed-reader visual identity
- excessive graph-first presentation

The interface should privilege:
- hierarchy
- focus
- continuity
- reading
- progressive discovery

## 19. Acceptance Criteria

The redesign is correct if a user can:

1. Create folders and sections to organize work.
2. Add artifacts into a section using note, link, or voice capture.
3. Navigate section hierarchy through the middle pane with breadcrumbs.
4. Open a section and view its artifacts and relevant signals in the right pane.
5. Open and edit a markdown page without the editor resetting while typing.
6. Inspect signals separately in a dedicated Signals area.
7. Understand that graph context is global across the workspace, not local to one section.
8. Use the product without needing to understand graph internals or backend concepts.

## 20. Assumptions and Defaults

- Primary v1 audience: **founder / analyst**
- Primary UX container: **Section**
- Primary concrete content unit: **Artifact**
- Primary system surfaced object: **Signal**
- KG scope: **workspace-wide global context**
- Shell layout: **3 panes**
- Folder navigation: **middle pane + breadcrumbs**
- Home emphasis: **Active Sections + Daily Brief**
- Signal presentation: **curated update card first**
- Capture pattern: **visible control + command palette**
- Graph role: **secondary analysis surface**
