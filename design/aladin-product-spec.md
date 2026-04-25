# Aladin Product Spec

## 1. Product Summary

Aladin is a terminal for signals and thoughts.

It ingests live data from generic sources, turns that data into structured signals, and connects those signals back to a user's evolving ideas over time. Instead of letting information disappear into feeds or notes go stale in archives, Aladin keeps thought alive by continuously relating new incoming evidence to prior artifacts.

Aladin is designed for people whose work depends on maintaining continuity of interpretation across ongoing streams of information.

## 2. Core Product Thesis

Most tools handle either:
- incoming information
- stored notes and thoughts

Aladin handles the interaction between them.

The core promise is:

**New signals do not just arrive. They update what you were already thinking.**

## 3. Product Vision

Aladin is a living knowledge and idea workspace where:
- sources continuously bring in new information
- the system transforms incoming artifacts into signals
- users capture and evolve their own ideas over time
- the graph/context layer improves how the system retrieves and reasons
- outputs compound into a durable system of thought

Aladin should feel like an always-on workspace for:
- monitoring
- synthesis
- reactivation of prior thought
- progressive refinement of ideas

## 4. Primary Product Concepts

### Source

A configured live input stream that brings new information into a workspace.

Examples:
- Reddit subreddit
- Bluesky search or account
- news feed
- stock ticker feed
- internal chat stream

### Artifact

A raw or normalized ingested unit from a source.

Examples:
- one post
- one article
- one message
- one document
- one transcript

### Signal

A processed, meaningful event or surfaced unit derived from one or more artifacts.

Signals are what the system decides are worth surfacing. They may:
- support an idea
- contradict an idea
- update an idea
- reveal a trend
- expose a new relationship
- indicate a change worth attention

A user may consider some signals to be "insights," but `signal` is the core product term.

### Idea

A living user artifact that evolves over time.

An idea may contain:
- markdown content
- linked evidence
- linked entities/topics
- custom prompts for future analysis
- a thesis, question, hypothesis, or line of thought

An idea is not static. New signals can affect it over time.

### Capture

A lightweight input that seeds future ideas or evidence.

Capture modes in current scope:
- light markdown note
- quick link + description
- voice note

### Graph / Context Layer

A structured context layer that links artifacts, signals, ideas, entities, and themes. Its main purpose is to improve retrieval, context assembly, and future reasoning quality.

## 5. Primary User Jobs

Aladin helps a user:
- capture ideas before they disappear
- monitor live sources without losing context
- connect incoming signals to existing lines of thought
- preserve and evolve hypotheses over time
- surface what changed and why it matters
- ask the system to further analyze relevant incoming signals
- build a durable body of structured thought instead of isolated notes and chats

## 6. Initial Use Cases

Aladin should support these patterns, even if product language stays generic:
- hobbyist trader or analyst tracking company/sector theses
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

### Signals

System-generated meaningful surfaced units derived from new source data.

### Ideas

User-created living artifacts that evolve over time and may include custom prompts for future analysis.

### Graph / Context

A graph-backed context layer that improves retrieval, linking, and future LLM pipelines.

### Specialist UI Surfaces

Use JS-based specialist surfaces where browser tooling is strongest, mounted into the Kotlin/Wasm shell.

Examples:
- markdown editor
- graph explorer
- mini React surfaces

## 8. Core User Loop

1. User configures sources and creates captures/ideas.
2. Sources ingest new artifacts continuously.
3. Backend processes artifacts and generates signals.
4. Signals are linked to relevant ideas and context.
5. User sees what changed and what prior thought it affects.
6. User updates ideas manually or lets the system analyze relevant signals through custom prompts.
7. Updated ideas and outputs become part of the ongoing context for future signals.

## 9. Product Behavior Principles

### 1. Signals over noise

Users should not have to sift raw streams unless they want to. The system should surface meaningful changes.

### 2. Thoughts stay alive

Ideas should remain revisitable and evolvable over time.

### 3. New information should reactivate old context

Incoming data should be evaluated against what the user already cares about.

### 4. The system should support partial thought

Capture can be rough, incomplete, and fast. Structure can emerge later.

### 5. The graph is infrastructure, not decoration

The graph should improve context and reasoning, not merely visualize data.

### 6. Kotlin owns product logic

Frontend JS surfaces exist for rendering and specialist interactivity, not for core business logic.

## 10. Information Architecture

The likely primary product surfaces are:
- **Capture**: quick note, link capture, voice note
- **Sources**: configure and manage live inputs
- **Signals**: surfaced meaningful incoming changes
- **Ideas**: living user artifacts, theses, notes, hypotheses, questions
- **Knowledge / Graph**: structured context and exploration layer

Optional later surface:
- **Outputs**: generated summaries, reports, analyses, decks, etc.

## 11. Idea Model

An idea is a first-class living artifact.

An idea may include:
- title
- content/body
- status
- linked entities/topics
- linked artifacts/signals
- optional custom analysis prompt
- timestamps/history
- future derived metadata

The key capability is that an idea can be used as an analytical lens for future incoming signals.

## 12. Signal Model

A signal is a surfaced meaningful event.

Signals may include:
- supporting evidence
- contradictory evidence
- linked artifacts
- linked ideas
- linked entities/topics
- confidence
- freshness
- type/category

Examples of signal types:
- trend
- contradiction
- update
- support
- emerging theme
- anomaly
- opportunity
- risk

The exact generation sophistication is intentionally deferred.

## 13. Out of Scope for Now

These are not core scope right now:
- final advanced signal-generation methodology
- full GraphRAG implementation details
- collaborative multi-user workflows
- deep permissions/teams model
- broad connector expansion
- polished desktop-native app path
- over-optimized graph ontology

These can be refined later after UX and backend architecture are stabilized.

## 14. Product Positioning

Aladin is best described as:

**A terminal for signals and thoughts.**

Alternative longer positioning:

**Aladin is a living workspace where incoming signals continuously interact with your evolving ideas, helping you keep context, detect change, and build durable understanding over time.**

## 15. Success Criteria for V1 Direction

The product direction is validated if a user can:
- connect a live source
- capture or create an idea
- receive new signals from that source
- see which signals matter to an existing idea
- ask for deeper analysis on that relationship
- keep evolving the idea over time

That is the core loop Aladin must prove.

## Next Step

The next step should be the technical spec:
- what exists already in `backend_v2`
- what maps to this product model
- what must be added or renamed next
