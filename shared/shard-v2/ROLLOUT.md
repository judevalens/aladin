# Shard v2 verification and rollout gates

Status as of 2026-08-31: the MongoDB + GraphQL datastore/runtime implementation is
working on `codex/shard-datastore-runtime`. V2 remains default-off. No production
enablement, existing-shard migration or v1 retirement has been performed.

New verification covers a real MongoDB 8.0 single-node replica set: namespace
isolation, CRUD, schema checks, null/missing filters, sorting, cursor pagination,
CAS, concurrent idempotent replay, tombstones, ordered change signals, freeze
fencing and export/restore. Node tests cover candidate failure, add/change/remove,
release pinning, per-request audience scope, same-hash tenant isolation, worker
environment isolation, budgets, timeout cleanup, manual lambdas and in-flight drain
during an atomic runtime switch. Go build tests prove server code/schema/manifests are inside
the immutable release identity and undeclared or Node builtin imports are rejected.

## Work-package status

| Package | Implemented | Remaining before its full acceptance |
| --- | --- | --- |
| WP01 | Shared schemas, generated Go map, cross-language fixtures, profile limits | Keep fixtures synchronized as contracts evolve |
| WP02 | Authorized registry/service, params, projection, capabilities, conditional writes | Broader provider/security review |
| WP03 | PostgreSQL compatibility plus Mongo default adapter, receipts, quotas, ordered changes and fenced archive restore | Operational retention/recovery tooling, crash/load evidence |
| WP04 | Deterministic client reducer, shared subscriptions, bounded recovery | Sustained memory/throughput soak at default caps |
| WP05 | Kit exports, protected bootstrap, actual web host, one shared socket | Native WKWebView/vendor-scheme pilot and complete lifecycle matrix |
| WP06 | Owned documents, Mongo event-triggered reconciliation and real workspace source | Full credential/fault matrix and broader provider coverage |
| WP07 | Published catalog, five resource MCP tools and optional persisted-operation execution | Staging operational monitoring/visibility |
| WP08 | Immutable code+contract+anchors, exact-build verification, atomic pointer, stage quotas | Per-field index/query-plan staging, reviewed inactive-build cleanup tooling |
| WP09 | Persisted/live/third-resource pilot, MCP JSON-RPC, two real web iframes, reconnect/revocation | Native and deployed staging pilots; sustained performance/fault matrix |
| WP10 | Read-only inventory and migration procedure | Actual saved-row/import audit, reviewed mappings, freeze/drain/cutover runner, authorization and individual migrations |

MongoDB is the preferred owned-data engine; PostgreSQL remains a supported adapter
and the release/control plane. GraphQL is the stable app boundary. TypeScript/Node
allows validated resolver add/change/remove without restarting the Go authority
service. These choices do not remove authorization, bounded views or atomic
publication requirements.

## Final verification result

The complete Go suite passes with the race detector, including the real Mongo
replica-set tests. All 76 frontend test files pass (511 tests), as do the nine
Node resolver-runtime tests; TypeScript, the production frontend build and
`go vet ./...` pass. Schemas, manifest, embedded Go schema map
and kit client bundle regenerate deterministically. The frontend build still
reports PDF dynamic-import and large-chunk warnings in the existing app build;
these are not test failures. Native-host and broad-rollout gaps below remain.

## Evidence collected

- Shared Go/TypeScript fixture validation and schema checksum parity.
- Client tests: sequence gaps, duplicates, identity/epoch routing, bounded queues,
  dependency changes, stale recovery, mutation receipts and revocation purging.
- Host tests: exact source-window checks, forged authority rejection, shared
  socket routing, late acknowledgement cleanup, account changes and send limits.
- PostgreSQL tests: isolation, restart persistence, concurrent CAS, duplicate
  request IDs, receipt failure rollback, quotas, tombstones, projection, numeric/
  null/missing query semantics, cursor scope, release compatibility and retention.
- Mongo tests: physical namespace isolation, concurrent idempotency, CAS,
  declared filters/sorts, null versus missing, cursor scope, change streams,
  expired receipt reuse and transactionally fenced export/restore.
- Archive tests: tombstone/receipt round trip, checksum verification, corrupted
  import rollback and refusal to overwrite accepted writes.
- API tests: real HTTP/WS with PostgreSQL, content-token rejection, default-off
  behavior, current release checks and workspace grant enforcement.
- Chromium pilot: real kit mounts owned collection/singleton and authorized
  workspace records; edits persist across close/reopen; a third declaration uses
  the same provider; server pagination returns distinct pages; published data
  stays separate; verification activates the exact staged build.
- MCP pilot: all five registered tools are called through SDK JSON-RPC. Reads
  work with the iframe closed, retries are idempotent and unauthorized agent
  mutations fail. Shared output schemas avoid RawMessage inference errors.
- Authoring discovery: SDK JSON-RPC exercises enabled/disabled guides, automatic
  starter configuration and real builds for both data APIs. Existing apps retain
  their storage guidance; missing contracts use protected release metadata, and
  disabled execution or storage read failures never advertise a fallback API.
  Copilot/MCP static instructions defer to discovery instead of duplicating APIs.
- Workspace channel regression: published viewers stay on published data across
  draft rebuilds and failures. Explicit previews display a separate-data warning;
  committed publication exits preview. Tests cover unpublished/legacy apps,
  stale metadata responses, bridge teardown and iframe credential refresh.
  Publication metadata is emitted atomically with activation, never on staging or
  failed activation. No existing draft records were migrated or deleted.
- Actual web-host pilot: two opaque-origin iframes load protected HTTP content
  using content-only tokens. The real frontend host modules share one WebSocket,
  render an agent write in both frames, recover after socket loss and purge
  both views on account change. This does not exercise native WKWebView or the
  complete desktop application shell.
- Protected serving ignores tampered mutable dist, including when v2 execution
  is disabled. Legacy v1 build, data hub and
  content-token tests remain passing.

Observed local sandbox timings (not production SLOs):

| Scenario | Observed result | Limit of the evidence |
| --- | --- | --- |
| First build + Chromium + four resource views | about 1.5–2.2 s | Includes build; warm local dependency cache |
| 10,000 small records; filter/sort + 500 validated results | median 23.7 ms, p95 24.5 ms, 20 reads | One view, small records, no sustained contention |
| Shared host socket loss → both views restored | about 307 ms | Local network; bounded retry; one disconnect |
| Complete pilot including two web frames | about 5.7 s | Sandbox source rows, not deployed staging |

## Reproduce

Use only `docker-compose.test.yml` / `aladin-test`; never the user's dev database.
Frontend dependencies and Chrome/Chromium must be installed. The browser pilot
bundles the real frontend host modules from `aladin_react/node_modules`.

```sh
# Repository root
python3 shared/shard-v2/generate.py
node shared/shard-v2/bundle-client.mjs

# backend_v2
# If Chrome is not discoverable, set DOCSURFACE_CHROME_PATH.
TEST_MONGODB_URI='mongodb://127.0.0.1:27029/?replicaSet=rs0&directConnection=true' go test -race -p 1 ./...
GOCACHE="$PWD/.gocache" go vet ./...

# aladin_react
npm test
npm run build

# services/shard-runtime
npm test
```

Regeneration should produce identical schemas/manifest and client bundle on a
second run. Run the pilot with `-run TestShardV2Pilot -count=1 -v` and the load check
with `-run TestShardResourceDefaultBoundLoad -count=1 -v` to capture fresh timings.

## No-go conditions and next gates

Broad rollout is **not approved** by these results. Complete these steps first:

1. Run the persisted/live scenarios in native Tauri/WKWebView, including local
   vendor cache, content-token renewal, account switching and detach/remount.
   Exercise the real application shell as well as its tested bridge modules.
2. Establish staging budgets for sustained refresh throughput, provider heap,
   queue pressure, per-shard request counts and reconnect recovery under load.
   Measure maximum record bytes and concurrent default-cap views; investigate
   plans and add bounded indexes/admission where needed. No increasing queue or
   memory trend is acceptable. The single-view benchmark is not enough.
3. Finish browser/API fault injection: server credential expiry/revocation during
   active streams, process kill after commit/before response, publish concurrent
   with commands, repeated disconnects, source disappearance and vendor failures.
   Existing CAS/receipt/reconciliation tests cover parts, not that full matrix.
4. Add reviewed retention/maintenance tooling for inactive builds, dormant
   receipts/cursors and tombstones. Include vendor files and protected release
   metadata in a verified backup, not only the data archive.
5. Run a deployed staging persisted pilot and live workspace pilot with genuine
   authorized sources. Keep unknown freshness visibly unknown. Only then prepare
   an explicit, reviewable production enablement with monitoring and backup.
6. Finish each MIGRATION.md record and its cutover tooling. Obtain explicit
   production direction before enabling, migrating or retiring anything.

The specification explicitly separates migration/v1 retirement from runtime
implementation. Preserve existing v1 routes and data until all shards have been
individually verified and the recovery window has closed.
