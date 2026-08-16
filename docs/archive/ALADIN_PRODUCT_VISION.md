# Aladin Product Vision

> **Historical / parked (2026-08-14):** This captures the older graph-grounded broad
> AI workspace thesis. The current product direction is the trading research
> workspace in [`../../CURRENT_PRODUCT.md`](../../CURRENT_PRODUCT.md) and
> [`../../design/TRADING_PRD.md`](../../design/TRADING_PRD.md). Treat this as strategic
> history unless it is explicitly revived.

**Status:** Historical strategy doc  
**Audience:** Me, future me, anyone I bring into this  
**Purpose:** Capture what Aladin is, structurally and strategically, distinct from any individual feature spec.

---

## The Thesis

**Aladin is an opinionated, graph-grounded workspace where the LLM is a substrate, not a chat box.**

The product hook in one sentence:

> AI shaped to your work, not a chat box for everything.

The defensible position:

> Proactive and interaction-based usage of LLM, grounded in a personal knowledge graph that no other product has.

---

## Why This Matters

The chat box is a local maximum. Every product still organized around "type into the box, get a paragraph back" is leaving real value on the table, because chat is fundamentally the wrong shape for sustained work:

- Chat is linear; real work is spatial and interleaved.
- Chat is ephemeral; real work needs persistence and structure.
- Chat demands full attention; real work happens around other things.
- Chat conflates "ask a question" with "edit an artifact"; these are different operations.

Most AI products are starting to escape chat: Cursor's tab-completion, Notion's inline edits, Claude artifacts, Linear's drafts. These are all bets that chat is a dead end for serious work.

Aladin's bet is bigger: not just "chat plus a few accessories," but a workspace where the LLM is structurally distributed across the product, showing up in many shaped surfaces appropriate to the task at hand. Chat is the fallback, not the front door.

---

## The Two Axes

Every AI surface in Aladin sits on two axes. This is the organizing model for the product.

### Proactive

The LLM acts without being asked. It is driven by triggers, schedules, or graph-state changes.

Examples:

- Trending-post gap analysis
- Drift detection on docs
- Smart prompts firing on ingestion
- Right-rail enrichment that just appears
- Ambient suggestions
- Notifications with synthesis

The user is the recipient, not the initiator. By the time they see anything, the system has done the work.

### Interaction-Based

The LLM responds to user action. The user initiates with a click, selection, command, or edit.

Examples:

- Inline "explain this" / "find related"
- Doc regeneration via diff and comment
- Cmd-K verbs
- Voice queries
- Chat fallback for the open-ended case

The user steers; the LLM responds.

### The Interesting Middle

Most of the daily-driver value lives in surfaces that are proactively populated but interactively explored:

- Right rail: proactive enrichment, click to dig in
- Smart prompt outputs: proactive synthesis, interactive refinement
- Notification landing surfaces: proactive arrival, interactive depth
- Drift indicators on docs: proactive flag, interactive resolution

The right shape for ambient AI is to proactively surface possibilities and invite interaction only when the user chooses. Never demand attention. Always make options available.

---

## Why Proactivity Is The Moat

Interaction-based AI is well-trodden. ChatGPT, Claude, and every assistant are interaction-based. The bar is not low, but the space is crowded.

Proactivity is harder and rarer. Most products cannot do it because:

- They do not know you well enough.
- Wrong timing is worse than no timing.
- The signal-to-noise cost is paid continuously, not once.

Aladin can do proactivity because the substrate is both:

- a graph model of what the user cares about structurally
- an attention model of what the user cares about now

Proactivity grounded in those is fundamentally different from "the LLM decided to suggest something."

This is the moat. Not the LLM. Not the UI. The grounded proactivity.

---

## Design Imperatives

The two axes imply different checklists. Do not conflate them.

### Proactive Surfaces Must

- Default to silence; fire only on high confidence.
- Make the reason visible; never surface without "why."
- Batch when possible; three at once beats three across the day.
- Be dismissable with feedback, such as "was this useful?"
- Decay; stale signals must stop firing.

### Interaction Surfaces Must

- Be responsive; latency kills interaction.
- Be discoverable; users will not find what they do not see.
- Be reversible; users explore aggressively when they can back out.
- Be predictable; same gesture, same shape of result.

When designing any new surface, ask first: which axis? Then check the relevant imperatives.

---

## What Is True Across Both Axes

The same substrate powers everything:

- **Graph**: structural model of the user's knowledge
- **Attention model**: temporal projection of what is currently active
- **Smart prompts**: saved questions that run continuously
- **Verb catalog**: named operations that map to graph query plus LLM render pairs

Every surface, proactive or interactive, is a different way of consuming or invoking this substrate.

Examples:

- The right rail is a proactive projection.
- The Cmd-K menu is an interactive entry into the verb catalog.
- A notification is a proactive smart-prompt output.
- Doc regeneration is an interactive smart-prompt invocation.

Same engine, different presence.

This is the architectural payoff. Users learn the underlying model once and recognize it everywhere. Surfaces feel like facets of one thing rather than separate features.

---

## What This Is Not

To stay sharp, define the product by what it rejects:

- **Not a chat-first AI assistant.** Chat is a fallback, not the primary surface.
- **Not a generic doc editor with AI features.** Aladin is graph-grounded; generic editors are not.
- **Not an autonomous agent platform.** Smart prompts are scoped, repeatable LLM calls, not deliberating agents. Power-user agent capabilities are a long-term extension, not the headline.
- **Not a knowledge base for everyone.** Aladin is opinionated about how research works. Users who want infinite flexibility should use Notion.
- **Not a search engine over your stuff.** Search is one verb. Aladin's value is in synthesis, projection, and proactive surfacing.
- **Not a Notion / Obsidian / Roam competitor.** Those are authoring tools. Aladin is a thinking tool that happens to produce artifacts.

---

## The Sequencing

Proactive surfaces are the unlock; interaction surfaces are the foundation.

Build in this order:

1. **Substrate first.** Graph, ingestion, enrichment, attention model, smart-prompt framework. Without these, every surface is a custom build.
2. **Interaction surfaces next.** Verb catalog, inline actions, Cmd-K, doc regeneration. Easier to ship, easier to validate, and teaches users the vocabulary.
3. **Proactive surfaces on top.** Notifications, ambient suggestions, drift detection. These only work when the substrate is rich enough to ground them.
4. **Power-user extensions last.** Composed prompts, agents, marketplace. This should emerge from real usage patterns, not be designed in advance.

The trending-post gap analysis loop is the first proactive milestone: a complete capture-to-notify cycle that exercises the whole stack. If that ships and feels good, the rest of the product is variations on the theme.

---

## The Decision Filter

Every feature decision should be checkable against the thesis:

- Does this strengthen proactive presence, interaction-based depth, or both?
- Does this leverage the substrate, or is it parallel infrastructure?
- Does this require the graph, or could ChatGPT do it?
- Does this respect the design imperatives for its axis?

If a feature passes all four, build it.

If it passes none, it is a chat-box feature wearing different clothes, and it does not belong in Aladin.

---

## What This Means For Build-For-Self

The thesis is what it is whether or not anyone else ever uses Aladin. It describes how I want to do research, structurally:

- Capture continuously, without friction.
- Structure quietly, without demanding attention.
- Consume through proactive synthesis when the system has something useful to say.
- Interact deeply when I want to dig in.

If Aladin makes that real, it will be the tool I use every day, regardless of whether it ever has another user. If others end up wanting it, the thesis tells them what they are getting and what they are not. Either way, the thesis is the constant. Implementation churns. The vision holds.

---

## One-Line Summary

> Aladin is what AI-native research feels like when the LLM stops being a chatbot and starts being a substrate: proactive when it has something useful to say, interactive when you want to dig in, grounded in a graph that knows what you care about.
