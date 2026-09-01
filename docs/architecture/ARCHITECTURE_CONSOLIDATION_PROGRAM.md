# Aladin Architecture Consolidation and Code Quality Program

**Status:** Draft 0.1
**Started:** 2026-08-31
**Purpose:** Turn Aladin from a fast, broad five-month build into a system that can be changed confidently for years.

## Why this work exists

Aladin has proven that the product can exist. It has a real desktop application,
backend services, synchronization, document and shard surfaces, a companion app,
and a meaningful body of tests. The next constraint is no longer whether features
can be built. It is whether the system can remain understandable while the product
direction becomes sharper.

The project does not need a ground-up rewrite. Its quality problem is more specific:

- some files and modules own too many responsibilities;
- several product eras remain present at the same time;
- dependency direction is documented but not always easy to see locally;
- active systems have accumulated transitional paths;
- a change can require understanding more of the repository than it should;
- implementation has moved faster than consolidation.

The work therefore has two distinct stages:

1. **Architecture consolidation:** decide what belongs, assign ownership, establish
   boundaries, and remove or quarantine obsolete paths.
2. **Targeted rewrites:** rewrite only the implementations that remain difficult
   after their intended boundaries are clear.

Rewriting before consolidation would risk producing cleaner code inside the wrong
architecture.

## Desired outcome

A developer should be able to answer these questions quickly:

- Which module owns this behavior?
- What may this module depend on?
- What invariants does it protect?
- What is its public API?
- Where are its tests?
- Can it be changed without loading the entire system into memory?
- Is this active product, supporting substrate, migration code, or parked history?

The program succeeds when ordinary changes become local, system boundaries become
visible in the directory and dependency structure, and obsolete concepts stop
taxing every future decision.

## Principles

### Preserve behavior before improving structure

Characterization tests come before risky extraction or replacement. Existing
behavior remains observable while implementation changes underneath it.

### Consolidate before rewriting

A rewrite must target a stable responsibility. No module is rewritten merely
because it is large or aesthetically uneven.

### Prefer simple, explicit patterns

Use pure domain functions, narrow interfaces, thin orchestration, explicit data
flow, and feature-owned tests. Avoid generic frameworks until a pattern has
repeated enough to justify one.

### One vertical slice at a time

Each refactor leaves the repository buildable and testable. A slice moves behavior,
tests, and ownership together, then removes the superseded path.

### Delete the old path

A successful rewrite reduces the number of ways the system performs the same job.
Compatibility layers need an owner and a removal condition.

### Line count is a signal, not a goal

Large declarative wiring files may be healthy. Small files can still be tightly
coupled. Optimize for cohesion, dependency direction, and ease of change.

### Product truth governs architecture

The active product is the trading research workspace. Supporting systems remain
when they strengthen that loop. Parked product ideas should not continue expanding
or shaping primary navigation and core abstractions.

## Scope

The program covers:

- product-surface consolidation;
- frontend module boundaries and state ownership;
- backend service, repository, API, and MCP boundaries;
- shard and document-surface runtime ownership;
- synchronization and realtime contracts;
- companion application boundaries;
- test architecture and verification;
- operational and architecture documentation.

It does not authorize a ground-up rewrite, a stack replacement, or new product
scope.

## Program phases

### Phase 0 — Stabilize the current frontier

Finish or checkpoint the active Shard v2 and board work before broad structural
movement.

Deliverables:

- a named checkpoint with a known test baseline;
- a short inventory of active uncommitted work;
- explicit release or rollback boundaries;
- no broad refactor mixed into unfinished protocol or persistence changes.

Exit gate: the current frontier can be resumed or reverted without reconstructing
intent from the working tree.

### Phase 1 — Establish the architecture map

Build a current, evidence-based map of Aladin rather than relying on historical
plans.

For every major module, record:

- responsibility;
- active, supporting, transitional, or parked status;
- public API;
- inbound and outbound dependencies;
- state and persistence ownership;
- major invariants;
- test location;
- known duplication;
- likely removal or rewrite pressure.

Deliverables:

- current system context map;
- frontend and backend dependency maps;
- module ownership registry;
- hotspot register;
- parked-system register;
- dependency rules that can eventually be enforced automatically.

Exit gate: every major behavior has one intended owner, and ambiguous ownership is
recorded as an explicit decision.

### Phase 2 — Consolidate product and module boundaries

Reduce conceptual surface area before changing internal patterns.

Work includes:

- quarantine parked graph-first, Tutor, and broad insight surfaces;
- separate historical documentation from current guidance;
- collapse duplicate utilities and parallel access paths;
- make API → service → repository direction visible and enforceable;
- separate product logic from transport, persistence, protocol, and rendering;
- define ownership between desktop host, web UI, backend, collab sidecar, and companion;
- name transitional paths and assign removal gates.

Exit gate: new work has an obvious home, dependency direction is explicit, and
parked systems no longer influence active architecture accidentally.

### Phase 3 — Targeted module rewrites

Rewrite modules only when extraction cannot produce a coherent result or when the
current pattern fundamentally conflicts with the consolidated architecture.

Initial audit candidates—not automatic rewrite commitments—include:

- Copilot orchestration and provider lifecycle;
- artifact persistence and outbox behavior;
- document-surface preview, build, and publication;
- MCP document-surface tools;
- workspace state production and synchronization;
- Copilot markdown rendering;
- board chrome and board-host integration;
- shard resource service and persistence after V2 stabilizes.

Each rewrite proposal states:

- the current failure mode;
- the stable responsibility being preserved;
- the replacement pattern;
- behavior and compatibility constraints;
- migration sequence;
- rollback path;
- tests that prove equivalence;
- code and abstractions that will be deleted.

Exit gate: rewritten modules have smaller public surfaces, clearer invariants,
fewer dependency directions, and no parallel legacy implementation.

### Phase 4 — Code quality hardening

Once responsibilities are stable, improve local implementation quality:

- naming that reflects product language;
- smaller cohesive functions and types;
- explicit errors and failure semantics;
- test fixtures that express behavior rather than implementation;
- consistent concurrency and cancellation rules;
- removal of stale comments and compatibility code;
- performance baselines for hot paths;
- observability at system boundaries;
- automated dependency and architecture checks.

Exit gate: module quality is maintained by normal verification rather than periodic
heroic cleanup.

## Workstreams

### Product and repository shape

Separate current product truth, supporting substrate, experiments, and history.
Establish a clear retirement process for parked systems.

### Frontend

Separate rendering, state derivation, orchestration, host integration, and
persistence. Prefer feature-owned APIs and selectors over broad hooks that
coordinate unrelated systems.

### Backend

Keep transport, application orchestration, domain rules, and persistence distinct.
Define interfaces near their consumers and avoid oversized services that become
dependency hubs.

### Shards and document surfaces

Treat contracts, build pipelines, preview runtimes, publication, host bridges, and
resource providers as distinct responsibilities with explicit trust boundaries.

### Sync and realtime

Document sources of truth, event ordering, replay behavior, reconciliation, and
cross-client ownership. Reduce duplicate implementations only after protocol
parity is proven.

### Companion application

Preserve shared domain rules while keeping platform adapters explicit. Avoid
mirroring desktop implementation details that are not part of a stable contract.

### Verification and operations

Keep sandbox-first verification, destructive-test guards, build reproducibility,
and release recovery as architectural features—not incidental scripts.

## Rewrite decision rubric

A module becomes a rewrite candidate when several of these are true:

- it has multiple unrelated reasons to change;
- its tests require unrelated infrastructure;
- its public API exposes internal sequencing;
- dependency direction is cyclic or ambiguous;
- state ownership is split across several layers;
- compatibility code dominates the active path;
- repeated defects come from the current pattern;
- extraction would preserve the same confusion under more filenames.

A module should not be rewritten only because it is old, large, AI-assisted,
stylistically inconsistent, or unfamiliar.

## Definition of done for a refactor slice

A slice is complete when:

- behavior is covered by tests before or during movement;
- the new owner and dependency direction are documented;
- the old entry point is removed or explicitly deprecated;
- no duplicate production path remains without a removal issue;
- unit and relevant integration tests pass;
- operational behavior and observability remain intact;
- the change reduces cognitive load for the next modification;
- follow-up work is captured as a bounded item rather than a vague TODO.

## Initial backlog

### Immediate

1. Checkpoint and describe the active Shard v2 and board frontier.
2. Generate a repository module inventory from tracked source.
3. Classify modules as active, supporting, transitional, or parked.
4. Produce frontend and backend dependency maps.
5. Create the hotspot register using size, churn, coupling, and test reach.
6. Identify duplicated paths and utilities.
7. Define the first consolidation slice without rewriting behavior.

### Next

8. Quarantine one parked product surface end to end.
9. Consolidate one duplicated access path.
10. Extract one stable subsystem from a hotspot with characterization tests.
11. Add one enforceable dependency rule.
12. Review the result and update the rubric before selecting a rewrite.

### Later

13. Approve the first targeted rewrite proposal.
14. Execute it as a reversible vertical slice.
15. Remove the superseded implementation.
16. Establish recurring architecture-health review based on evidence.

## Decision log

Record architecture decisions with:

- date;
- context;
- decision;
- alternatives considered;
- consequences;
- owner;
- revisit trigger.

The log explains why the system has its current shape, not merely the shape.

## Measures

Track trends rather than chasing arbitrary thresholds:

- number of ambiguous module owners;
- number of active duplicate paths;
- dependency-rule violations;
- files and modules touched by a typical feature change;
- time required to locate the correct owner;
- integration test scope required for local changes;
- stale compatibility layers;
- defects caused by unclear state or persistence ownership;
- successful deletion of obsolete code.

## First planning checkpoint

The first checkpoint answers three questions:

1. What is the smallest stable architecture Aladin needs for the trading research workspace?
2. Which existing systems directly support that architecture?
3. Which systems should be parked, isolated, simplified, or removed?

Only after those answers are written should the program commit to its first major
rewrite.
