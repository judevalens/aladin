# Existing-shard migration preparation

Status: inventory only; no v1 data or app has been migrated. WP10 depends on the
WP09 release gate. The locked specification requires explicit direction before
production deployment or data migration. General implementation approval is not
a reason to convert the user's saved shards automatically.

## Read-only inventory, 2026-08-30

The Aladin connector listed ten visible app artifacts. Root `index.tsx` and
published/draft build markers were read without edits. Full IDs and build hashes
are recorded in `fixtures/migration-inventory.json`. These are observed authoring
files/build markers, not an atomic backup or proof the current source matches the
published bundle. Imported modules and actual saved KV rows have not been audited.

| Shard | Root-file persistence signal | Proposed review |
| --- | --- | --- |
| 3 Great Stock Strategies — Interactive Guide | No root hook found; imports components | Inspect imported components and saved rows |
| AI & Machine Learning in Trading — Interactive Guide | No root hook found | Inspect imports, component state keys and saved rows |
| Test Shard 2 | `stateKey` appears | Review component persistence and source/build differences |
| Test Shard 3 | `useKV("items/")` | Candidate collection; review each saved value and ID mapping |
| Ticker Match — Memory Game | `useShardState("best-moves")` | Candidate singleton; confirm value type before wrapping |
| Statistics for Algo Trading — A Rebuild-From-Scratch Course | No root hook found; small entry module | Inspect import closure and saved rows |
| United States — Interactive Wiki | `stateKey` appears | Review component persistence and saved rows |
| Codex Task Tracker | `tasks/`, `tracker/focus`, `tracker/bootstrapped`, `tracker/filter` | Candidate task collection plus explicit saved-state singletons |
| Shards as Hosted MCP Apps — Design Review | `design-review/v1` | Candidate singleton; inspect saved shape/version |
| Shard v2 — Declarative Bridge & Data Resources | No root hook found | Inspect imports, component state keys and saved rows |

“No root hook found” never means “no user data.” Default component state keys,
imported hooks, old unused keys and draft/published divergence all matter. Do not
use preview emulator state as saved-data evidence. Do not infer schemas or delete
unknown rows from source-level observations.

## Per-shard migration record required before cutover

1. Capture source/import closure, anchors, dependencies and exact build IDs.
   Inventory actual saved KV namespaces, live/deleted rows, sizes and revisions
   independently for draft and published. Mark every key keep/transform/archive.
2. Review a versioned, deterministic mapping from each key/prefix to an owned
   dataset, record ID and object schema. Scalars/arrays need an explicit object
   wrapper. Document unmapped or invalid values rather than dropping them.
3. Back up v1 rows and files to trusted storage; verify hashes and a restore in an
   isolated database. V2 resource archives alone do not back up v1 or vendor files.
4. Use the shard-scoped Mongo freeze fence before export. Every Mongo mutation
   touches the same control record transactionally, so the freeze drains conflicting
   writers and rejects new ones. A v1-to-v2 transformation still needs an explicit,
   shard-specific mapping and v1 write fence.
5. Transform and validate into a new generation in the sandbox; compare every
   row/count/hash and expected user-visible behavior. No dual writes. Review the
   data-bearing migration artifact and rollback boundary before production use.
6. After explicit production authorization, activate code+contract+generation
   atomically through the migration coordinator. Keep v1 frozen for that shard,
   observe authorized read/write paths and preserve recovery artifacts.
7. Before any v2 writes, rollback may restore the previous pointer/generation.
   After v2 writes, do not restore an old backup over new data: export current
   state and use a reviewed forward/reverse mapping or keep the shard frozen.
8. Retire v1 only after every inventory item is verified and its recovery window
   closes. Do not remove the global v1 bridge/storage path during pilot work.

## Implemented recovery boundary

Mongo V2 export/restore is available as an internal repository interface after a
tested namespace freeze. The versioned archive contains durable records (including
tombstones) and audit events. Restore rewrites them into a new, empty generation,
enforces record/byte quotas and commits atomically. Expiring receipts and cursors
are deliberately omitted; retry transport state is not migrated as user data.

This provides a tested building block, not a completed v1 migration tool. Ordinary
release activation deliberately rejects incompatible schemas/generation changes.
An operator must not bypass that guard by manually changing protected pointers.

## Authoring SDK migration

Published releases are immutable bundles, so removing the provisional UI kit does
not change a shard that is already published. A source rebuild is the migration
boundary:

1. Move nonvisual imports such as `useShardState`, `useKV`, `useResource`,
   `executeGraphQL` and `invokeLambda` from `@aladin/kit` to `@aladin/shard`.
2. Replace kit UI imports with locally authored React and semantic HTML. Use the
   host's token-backed Tailwind colors, typography roles, radii and shadows first;
   use authored CSS where a custom visual or interaction needs it.
3. Put `data-anchor` and `data-kind` directly on the element that owns each
   declared region. These attributes remain platform metadata and do not require
   a component wrapper.
4. Build, inspect the preview and snapshot, verify every anchor and interaction,
   then publish through the normal immutable release flow.

The builder rejects new `@aladin/kit` imports with a targeted migration message.
There is no automatic UI rewrite because choosing the replacement structure and
interaction is design work. `@aladin/kit` remains reserved for a future component
system; it is not an alias for the runtime SDK.
