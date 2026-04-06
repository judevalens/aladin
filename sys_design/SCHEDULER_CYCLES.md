# Aladin — Source Scheduler And Sync Cycles

## Problem

The Redis/asynq sync path was using `sources.config` for two different jobs:

- head refresh frontier
- in-flight pagination cursor

That design is flawed because a source can be re-claimed for a fresh poll while an
older pagination chain is still in progress. Once that happens, the source-level
cursor stops being meaningful:

- refresh can generate new pagination
- pagination can still be running from an older cycle
- both flows mutate the same source config keys
- the scheduler has no explicit notion of cycle ownership

This breaks fairness and cursor correctness.

## Core Model

The scheduler owns source arbitration.

- `Source` is the unit of scheduling fairness.
- `SyncCycle` is the unit of traversal state.
- A worker executes one fetch step for one cycle.
- The arbiter decides what a source's next turn should be.

The important separation is:

- per-source arbitration
- per-cycle traversal state
- overlap / supersession policy

`sources.config` should only hold long-lived source frontier state.
Pagination continuation must live on a cycle.

## Current Direction

The implementation is moving toward this shape:

### Source

Source holds long-lived scheduling state:

- `sync_status`
- `sync_interval`
- `last_picked_at`
- `last_refresh_at`

Source selection remains SQL-backed and uses fair ordering:

- oldest `last_picked_at`
- then oldest `last_synced_at`
- then oldest `created_at`

### SyncCycle

`sync_cycles` stores traversal state for a source:

- `id`
- `source_id`
- `kind` (`refresh` or `backfill`)
- `status` (`active`, `running`, `complete`, `closed`)
- `cursor`
- `covered_until`
- `last_picked_at`
- `created_at`
- `completed_at`

The cycle owns:

- pagination continuation
- per-cycle identity
- eventually overlap frontier ownership

## Scheduler Ownership

The scheduler owns the arbiter.

The layering should be:

- scheduler: orchestration and source turns
- arbiter: pure policy decisions
- syncer: execute one fetch step
- repositories: persist source and cycle state

The arbiter currently decides:

- skip if the source already has a running cycle
- create a refresh cycle if refresh is due and no active refresh exists
- otherwise continue the oldest active cycle

This lives in `backend_v2/internal/sync/arbiter.go`.

## Current Implementation Status

Implemented:

- source fairness timestamps: `last_picked_at`, `last_refresh_at`
- deterministic source claim ordering in SQL
- `sync_cycles` table and repository
- scheduler arbiter for choosing refresh vs continuing an active cycle
- `CycleID` carried in sync jobs
- pagination cursor moved from source-only state into cycle state
- sync handler updates cycle cursor on `HasMore`
- sync handler completes cycle on terminal page

Not implemented yet:

- overlap detection between newer and older cycles
- writes/reads of `covered_until`
- cycle terminal reasons like `overlap` vs `exhausted`
- explicit supersession / closure policy
- richer backfill policy beyond "continue oldest active cycle"

## Intended Policy

The desired behavior is:

- a source should not have conflicting refresh and pagination traversals mutating the same cursor state
- a fresh refresh should not be blocked forever by historical backfill
- a refresh may fill the gap above an older active cycle
- the newer cycle should stop when it reaches coverage already owned by the older active cycle
- the older cycle should then continue until completion

In other words:

- older in-flight history owns the deeper continuation
- newer refresh fills only the uncovered head gap
- overlap is the stop signal for the newer cycle

## Missing Semantics

The next implementation step is to make overlap explicit.

Each fetch step should eventually report enough information for the scheduler to
manage coverage:

- next cursor
- covered frontier for the cycle
- overlap signal
- completion reason

At that point `covered_until` becomes meaningful and the arbiter can apply the
real policy:

- choose refresh or backfill for a source turn
- stop newer cycles on overlap with older active coverage
- keep older cycles draining history

## Near-Term Next Steps

1. Define `covered_until` format per source type.
   Prefer exact item identity or `(timestamp, external_id)` style frontier.

2. Extend syncer results with cycle-level traversal signals.
   Distinguish:
   - source frontier updates
   - cycle cursor updates
   - overlap / completion reason

3. Persist cycle coverage after each fetch step.

4. Add scheduler logic for overlap termination.
   Newer cycle closes when it reaches older active coverage for the same source.

5. Add tests for:
   - refresh creating a new cycle while older backfill exists
   - newer cycle stopping on overlap
   - older cycle continuing afterward
   - no concurrent running cycles per source

## Constraint

Do not move scheduling truth into the transport queue.

The queue should remain a transport for executable work.
SQL should remain the source of truth for:

- source fairness
- cycle state
- overlap ownership
- next-turn arbitration
