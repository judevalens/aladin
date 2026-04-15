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
- pluggable arbitration policy

`sources.config` should only hold long-lived source frontier state.
Pagination continuation may live on a cycle, but only as a best-effort continuation token.
Correctness must come from exact item identity, not from assuming the upstream cursor is stable.

## Implemented

The current code implements this shape:

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
- `kind` (`refresh`)
- `status` (`active`, `running`, `complete`, `closed`)
- `completion_reason` (`seen_overlap` or `exhausted`)
- `cursor`
- `head_boundary`
- `last_picked_at`
- `created_at`
- `completed_at`

The cycle owns:

- best-effort pagination continuation
- per-cycle identity
- scheduling state

## Current Behavior

The scheduler owns the arbiter.

The layering should be:

- scheduler: orchestration and source turns
- arbiter: pure policy decisions, swappable
- syncer: execute one fetch step
- repositories: persist source and cycle state

The current default arbiter decides:

- skip if the source already has a running cycle
- create a refresh cycle if refresh is due and no active refresh exists
- otherwise continue the newest active cycle

This lives in `backend_v2/internal/sync/arbiter.go` as `FreshnessFirstArbiter`.

That current policy is one implementation, not a permanent truth.
The code already supports swapping arbitration policy without changing:

- scheduler loop
- queue transport
- cycle persistence
- syncer execution contract

The following are implemented today:

- source fairness timestamps: `last_picked_at`, `last_refresh_at`
- deterministic source claim ordering in SQL
- active paused cycles remain eligible for scheduler turns
- `sync_cycles` table and repository
- swappable arbiter interface with `FreshnessFirstArbiter` as the default policy
- `CycleID` carried in sync jobs
- pagination cursor moved from source-only state into cycle state as a continuation hint
- fast seen-ID lookup abstracted behind a sync-layer interface
- Redis-backed seen-ID store using exact `(source_id, external_id)` identity
- sync handler updates cycle cursor on `HasMore`
- sync handler marks successfully-enqueued artifacts as seen
- sync handler completes cycle on terminal page
- syncers stop when they hit already-seen external IDs
- sync cycles persist explicit terminal reason: `seen_overlap` or `exhausted`
- integration coverage uses the real orchestrator and real syncers with only the upstream API clients faked
- Reddit flow fixtures can be generated from simple JSON specs for scheduler/sync tests

## Implemented Policy Semantics

The code currently behaves like this:

- a source should not have conflicting refresh and pagination traversals mutating the same cursor state
- preemption should happen only between fetch steps, not during a running fetch

In concrete terms:

- one fetch/page is the atomic execution unit
- `running` work is not interrupted mid-request
- once a page finishes and the cycle returns to `active`, the arbiter may choose a refresh next if the current refresh window rules allow it
- refresh precedence is page-boundary preemption, not hard cancellation

In other words:

- older in-flight history owns the deeper continuation
- newer refresh fills only the uncovered head gap
- seen-item identity is the stop signal for the newer cycle
- while a newer active cycle exists for the current refresh window, it keeps winning turns over older paused cycles
- once the newer cycle completes, older active cycles resume until completion

## Transport Semantics

The asynq queue is transport, not truth.

That distinction matters in two places:

- enqueueing a task to a queue does not require a worker to be consuming that queue yet
- a worker only dequeues tasks from queues listed in its `Queues` config

So backlog behavior is:

- if a task is enqueued to `pipeline:first_pass`
- and no worker subscribes to `pipeline:first_pass`
- the task stays in Redis until a matching worker comes online

Failure behavior is different:

- if a worker does subscribe to a queue
- but its mux has no handler for the task type
- asynq dequeues the task and immediately fails it as `handler not found`

This is important for sync-only testing:

- removing pipeline handler registration is not enough to preserve backlog
- to keep pipeline tasks queued, the worker must also omit pipeline queues from its `Queues` map
- otherwise the worker will consume and fail them

The current sync path still enqueues `pipeline:first_pass` after artifact handoff.
If we want a true sync-only mode later, that should be an explicit mode switch in the sync orchestrator or worker wiring, not an accidental consequence of missing handlers.

## Planned

The next policy work should build on the current interface, not replace it.

The arbiter is already policy-driven, so future work is about adding implementations and semantics.
Examples of possible policies:

- strict freshness-first
- permissive freshness with a longer hard deadline
- oldest-cycle bias
- weighted mixed policy

The likely near-term shape is a two-threshold freshness policy:

- soft deadline: refresh is due, but older active cycles may keep progressing
- hard deadline: once freshness passes a larger permissive deadline, refresh must take the next turn after the current fetch step completes

This preserves:

- source-level fairness
- page-boundary preemption only
- bounded freshness lag
- eventual progress for older active cycles

## Planned Semantics

Stopping is now identity-based rather than frontier-based.

Each fetch step now reports enough information for the scheduler to keep cycles moving:

- next cursor
- source frontier updates
- exact item identities in the returned page

The remaining gaps are:

- explicit closure/supersession semantics
- watchdog-based recovery for stale `running` cycles
- sync-only mode that intentionally bypasses pipeline handoff
- non-Redis seen-store implementation if Redis stops being the right fit

## Next Steps

1. Implement soft/hard freshness deadlines as an additional arbiter policy.
   Refresh should preempt paused cycles only once freshness passes the permissive threshold.

2. Decide whether seen-overlap should persist as `complete` or `closed`.

3. Add watchdog/heartbeat recovery for stale `running` cycles.

4. Expand tests for:
   - refresh creating a new cycle while an older refresh-started cycle is still active
   - preemption happening only at page boundaries
   - soft deadline allowing older-cycle progress
   - hard deadline forcing refresh next turn
   - newer cycle keeping priority until it hits seen IDs or exhausts
   - older cycle resuming afterward
   - no concurrent running cycles per source
   - sync-only mode behavior when pipeline queues are intentionally not subscribed

## Constraint

Do not move scheduling truth into the transport queue.

The queue should remain a transport for executable work.
SQL should remain the source of truth for:

- source fairness
- cycle state
- overlap ownership
- next-turn arbitration
