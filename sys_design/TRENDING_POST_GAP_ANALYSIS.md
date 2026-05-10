# Aladin — Trending Post Gap Analysis

**Status:** Draft / north-star scenario
**Purpose:** Define one complete capture -> structure -> consume loop end-to-end, as the reference workflow the rest of the product is built to support.

---

## 1. North-Star Scenario

> As someone interested in learning algorithmic trading, I want a notification when a post on `r/algorithmictrading` is trending, with a suggested gap or connection against my previous research and notes.

This is the demo that sells Aladin. It is a complete loop where:

- **Capture** is ambient: continuous polling, no user action.
- **Structure** is silent: entity extraction, graph linking, background enrichment.
- **Consume** is proactive: the system reaches out with a synthesized suggestion before the user asks.

By the time the user sees the notification, the copilot has already done the work. The user's job is to decide whether to dig in.

This is a different product than "chat box that answers questions about your workspace." It is the strongest argument for Aladin's existence because no other product can produce this output without rebuilding the same stack.

---

## 2. The Loop

Six steps. Each is a real piece of work; none is a one-liner.

### 2.1 Ambient Ingestion

Continuous polling of subscribed sources such as Reddit or Bluesky. Raw provider content enters the enrichment pipeline as transient processing input. Durable Aladin storage keeps derived value — summaries, entities, key claims, relevance metadata, and source attribution — with a link back to the original source when the user needs primary context.

### 2.2 Trend Detection

A post crosses a threshold that marks it as trending.

- Not absolute upvote count; that varies wildly by subreddit.
- Prefer engagement velocity vs. per-source baseline: upvotes per hour, comment rate vs. typical post at that age.
- v1 cut: per-subreddit tuned thresholds, such as `N upvotes in M hours`.
- v2: rolling baseline per source, time-of-day aware, weekly-cycle aware.

Trending is a property of the artifact, set at detection time. It can decay; a post is trending for a window, not forever.

### 2.3 Relevance Filter

Trending alone is not enough. The post must be relevant to this user specifically.

- On ingestion, every artifact has entities extracted and linked into the graph.
- When a post is flagged trending, query whether the post's entity set overlaps with entities in the user's existing notes and saved artifacts.
- Overlap above a threshold means relevant.
- No meaningful overlap means ignore; do not notify.

This is the step that makes the workflow personal. Without it, Aladin is a Reddit notification client. With it, Aladin does something only Aladin can do.

**Implication:** entity extraction at ingestion time is non-optional for this workflow. Lazy extraction breaks the relevance filter.

### 2.4 Gap And Connection Analysis

Once a post is trending and relevant, the system performs a structured comparison between the post and the user's relevant subgraph.

Two distinct operations return different shapes:

- **Gap analysis:** what does this post cover that the user's notes do not?
- **Connection analysis:** how does this post slot into the user's existing thinking?

Both are LLM operations grounded in deterministic graph queries:

- Graph: subgraph of the user's notes related to entities in the post.
- LLM: structured comparison between post content and that subgraph.
- Output: a short, specific synthesis; not a generic summary.

This is where the LLM-as-frontend-of-the-KG framing pays off. The LLM is doing a tightly scoped comparison over a known subgraph, not generating freeform answers.

### 2.5 Notification

The output of step 2.4 is the notification content, not just a trigger.

The lazy version:

> Trending post on `r/algorithmictrading`: [title]

The Aladin version:

> Trending post about pairs trading. You have 3 notes on mean reversion but nothing on cointegration testing, which this post covers.

The notification is the synthesis. The user should not have to open the app to get the first unit of value.

Surfaces:

- In-app inbox: first surface.
- Push: second surface.
- Email digest: batched, later.

### 2.6 Consume Surface

If the user taps in, they land on the post artifact with the gap/connection framing visible:

- derived post summary / key claims in the artifact view
- source URL to open the original post when needed
- "Why we sent this" panel with relevance reasoning
- gap or connection synthesis from step 2.4
- inline links to existing user notes that triggered the relevance match
- verbs: deepen, save, dismiss, link to existing note

This is where the in-app copilot kicks in. The user can ask follow-ups, request a deeper synthesis, or capture a new note that gets linked into the same neighborhood.

---

## 3. Product Implications

### 3.1 Copilot Appears Across Capture, Structure, And Consume

This scenario is almost entirely capture-side and structure-side. By the time the user sees anything, the work is done. Designing the copilot only as a consume-side chat box misses where the real value lives.

Copilot postures:

- **Ambient worker:** monitors, extracts, matches, synthesizes.
- **Notifier:** reaches out with the research delta.
- **Inspector:** explains context inside the artifact rail.
- **Interactive drone/pane:** lets the user ask follow-ups.

### 3.2 Ambient Surface Is A First-Class Copilot Posture

Orb, prompt, card, and pane are in-app surfaces. This scenario adds:

- **Ambient surface:** copilot reaching the user outside the active app workflow.

This is its own design problem: notification copy, action affordances, snooze, dismiss, save for later. Opinionated workflow products win or lose here.

### 3.3 Verbs Map To Graph Operations

The verb catalog stops being abstract:

| Verb | Graph operation | LLM operation |
|---|---|---|
| Trend relevance | entity overlap query | none; deterministic |
| Gap analysis | subgraph by entity match | structured comparison |
| Connection analysis | subgraph by entity match | structured slotting |
| Recall | entity lookup + neighborhood expansion | render as prose |
| Connect | multi-hop path query | render path as narrative |

Each verb has a graph shape and a render shape. The graph shape is auditable. The render shape is where the LLM earns its keep.

### 3.4 The Right Rail's Eventual Job

The Related, Entities, and Context sections in the right rail are not enrichment fields. They are projections of the artifact's graph neighborhood.

The right rail and the proactive-notification system are the same engine viewed at different zoom levels:

- Right rail = always-on projection of the active artifact's neighborhood.
- Notification = on-demand projection triggered by trend events.

Both consume the same graph queries. Build the engine once.

---

## 4. v1 Scope

Goal: ship the full loop for one source and one user, end to end. Crude is fine. Complete matters more than polished.

### In Scope

- per-subreddit velocity-based trend detection with tuned thresholds
- entity extraction at ingestion time
- transient raw provider content during processing, with durable storage limited to derived insight plus provenance
- user-interest model from entities present in the user's existing notes and saved artifacts
- relevance scoring from entity overlap above a threshold
- gap analysis through one structured LLM prompt
- connection analysis through one structured LLM prompt
- in-app inbox notification surface first
- consume surface using the existing artifact view plus a "Why we sent this" panel and synthesis output
- opt-in per source, such as "watch `r/algorithmictrading` for me"

### Out Of Scope

- automatic interest inference from notes beyond entity overlap
- rolling baseline trend detection
- source discovery
- cross-source trend aggregation
- multi-user or shared interest models
- active-listening voice or autonomous agents
- write-actions that draft follow-up notes automatically
- push notifications and email digests before the in-app inbox works

### Build Order

1. Entity extraction reliability: verify entities are good enough on Reddit posts and user notes.
2. Trend detection: start with one subreddit and hand-tuned thresholds.
3. Relevance filter: wire entity overlap query.
4. Gap/connection prompts: design structured output and test on real posts.
5. In-app inbox: show synthesis and link to artifact.
6. Consume surface: show "Why we sent this," synthesis, and matched notes.
7. Push notification: extend only once the in-app version is useful.
8. Email digest: batched, lower priority.

---

## 5. Open Questions

- **Opt-in granularity:** per-subreddit, per-topic, or per-entity? v1 defaults to per-subreddit.
- **Notification frequency cap:** what prevents 20 notifications per day from a high-traffic source?
- **Feedback loop:** how does the user mark useful vs. missed? Even binary feedback is better than none.
- **Sparse note set:** cold-start users may not have enough prior research for relevance matching. This scenario assumes users already have a research base.
- **Decay:** when does a post stop being eligible to notify on? v1 should tie this to the trending window.
- **Synthesis rendering:** one sentence, paragraph, or bullets? v1 should optimize for one compact notification plus a slightly fuller in-app card.

---

## 6. Why This Scenario Comes First

- It exercises ingestion, entity extraction, graph, LLM synthesis, notification, and consume surfaces.
- It produces output no normal notes app or chat app can produce from the same inputs.
- It is shippable in a bounded scope.
- It is a complete loop: capture, structure, consume.
- It plants the flag on Aladin's workflow: capture continuously, structure quietly, consume through proactive synthesis.

If this works, the rest of the product is additional surface area on the same engine. If this does not work, polishing the rest matters less.
