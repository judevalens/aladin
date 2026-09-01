# Shard v2 backend and operating constraints

## Decision and current status

The user approved continuing with the existing PostgreSQL KV storage foundation,
without requiring Mongo. V2 keeps JSON documents and conditional revision writes
but stores them in separate `shard_resource_*` tables. This prevents existing
v1 KV endpoints from bypassing v2 validation, quotas, resource permissions or
receipt handling. The old `shard_kv` table, API and replica are unchanged.

The new backend works through an isolated integration test:

```
bridge/2 HTTP command → resource service → PostgreSQL transaction
                     → periodic authorized snapshot → WebSocket push
```

`SHARD_V2_ENABLED=1` enables the backend service. The default is off. Enabling it
does not create releases or expose legacy shards: calls still require a
protected active v2 contract. No production deployment, data migration, or
release activation has been performed.

## Build and publication

With the feature enabled, a `contract.json` opts a build into v2. Compilation
validates schemas, provider capabilities and anchor bindings. Workspace stage
validation verifies granted IDs and declared output schemas against the real
authorized source. A build without a contract retains the v1 path.

A successful build captures JS, CSS, import map, anchors and contract; its ID
hashes that complete captured artifact. PostgreSQL stages immutable code and
contract together. Draft builds activate only draft; published builds stage
only. `publish_app` verifies the exact captured build in Chromium, then switches
the published pointer in one transaction. There is no v2 best-effort publication
when the renderer is unavailable. The preview always uses draft resource data,
even when validating a staged published build. Verification is not a read-only
execution sandbox: authored mount effects can issue permitted draft commands.

Content serving and manifest reads use the active protected artifact. An
optional build ID must match, or serving fails with 409. A broken v2 release
never falls back to mutable dist. Disabling v2 execution retains protected
release lookup and refuses to serve existing v2 shards; it does not revive old v1
files or change active pointers. The kit handshake verifies code/release
identity. Only this protected selection adds the approved `unsafe-eval` policy;
v1 keeps its CSP. All v2 iframe code receives that permission, not just AJV.

The publication commit boundary is the active pointer, not the artifact summary
or best-effort build status. A later summary update failure does not undo a
committed publication; retrying the same build is safe. Vendor dependencies
remain content-addressed files and must be retained/backed up separately.

## Implemented behavior

- One service for binding or direct-resource reads/writes, reusable by UI and
  MCP tools. Transports derive principal, app/agent audience, shard,
  environment and pinned release hash; those fields cannot be supplied through
  authored binding params.
- Protected immutable resource release rows and an atomic active pointer.
  Draft and published releases/data are independent. Staging alone changes no
  grants. Old hashes fail. Incompatible schema/generation changes require a
  separate migration; activation validates existing active records.
- Author-defined collection/singleton schemas, literal/input/singleton-dependent
  params, separate app/agent capabilities, server-side output projection and
  full replacement writes. Incoming query filters narrow the authored binding's
  filter rather than removing it; projected-out fields cannot be queried.
- `shard.documents`: persistent JSONB records, server-generated IDs stabilized by
  request ID, create-if-absent, guarded replace, deletion tombstones and explicit
  errors. Tombstones retain record data for future authorized export/recovery.
  Replacing a missing or tombstoned record fails. Tombstone IDs cannot be
  reused by insert.
- Per-actor command receipts bind the request ID to namespace, release, target
  and payload. Successful and business-rejected storage commands retain their
  outcomes for 24 hours. Retries return the original outcome; different payload
  reuse conflicts. Authorization is checked again before any replay is returned.
- Data, receipt and metadata-only audit/outbox entry commit in one PostgreSQL
  transaction. Failure to persist the receipt rolls back the record. The outbox
  lock is acquired before writes, preserving the existing sync ordering rule.
- Atomic active-byte quota admission under a namespace transaction lock.
  Additional caps bound records (including tombstones), receipts and cursors.
- Parameterized SQL for filters, boolean groups, scalar equality/in/existence,
  numeric ranges, scalar sorts, and ID tie-breaking. Missing is distinct from
  JSON null; both sort last. String/ID order uses the C collation. Mixed non-null
  scalar sort types are rejected. No data scan runs in the client.
- Random cursor tokens are bound to principal, view, generation, contract and
  query through the resolved view hash. They expire after 15 minutes. Pages are
  **read-current**, using bounded offsets; concurrent writes may affect page
  membership. This is not a snapshot-isolated export API.
- `workspace.nodes`: read-only NodeView records through the existing authorized
  entity services. Dynamic ID lists can only narrow fixed IDs in the protected
  release. Unsupported observation is rejected. Source values are not copied
  into owned storage.
- Generic `refresh-snapshots` subscriptions for both providers. Default interval
  is one second, active only while subscribed. Unchanged snapshots emit no new
  event; changes emit complete replacement views with consecutive string seqs.
  A new subscription always starts at seq 0 with a fresh epoch.
- Refreshes recheck ownership, release and capabilities. A slow consumer retires
  the subscription instead of skipping frames or growing queues. Missed outbox
  notifications recover through periodic authoritative reads; this profile does
  not promise every intermediate change or durable WS replay.

## HTTP and WebSocket mapping

The machine-readable route fixture is `fixtures/api.json`. Both transports reuse
`bridge-request.json` and `bridge-response.json` envelopes.

HTTP: `POST /api/shards/{id}/v2/{draft|published}/request`.
The host sends `X-Shard-Contract` after discovering/pinning the active hash. Hello
can run without a hash; all resource operations require the current one.

```json
{
  "aladin": "bridge/2", "type": "request", "id": 17,
  "method": "resource.read", "params": {"binding": "tasks", "inputs": {}}
}
```

The response is the same envelope consumed by the client bridge adapter. HTTP
status also reflects the error: 400 invalid, 403 denied, 404 absent, 409 stale or
conflict, 429 quota/rate limit, 503 unavailable. Responses are not cached.

The application uses one shared host WebSocket:
`GET /api/shard-resources/ws`. Each message wraps the unchanged bridge request in
`host-request.json`: `{target: {shardId, environment, contractHash}, request}`.
Only the trusted host builds that target. The iframe still sends only bridge/2.
`GET /api/shards/{id}/release?channel=draft|published` supplies protected metadata.
Normal workspace viewing selects published code and data. Draft preview is an
explicit mode, labelled as separate test data; an unpublished shard also opens
there. A failed/unavailable published release never causes an automatic fallback
to draft. Metadata reports `available:false` when no protected release or legacy
bundle exists. Committed publication emits `artifact.published` with its build
and contract identity in the activation transaction. Viewers return to published
on that event; successful staging and subsequent draft builds cannot switch the
data environment. Existing draft records are not copied or deleted.

The earlier per-shard WS route is retained for conformance/compatibility:
`GET /api/shards/{id}/v2/{draft|published}/ws?contractHash=...`.
The UI never opens one physical socket per shard. Shared sockets allow 128
subscriptions; the per-shard route and each host frame allow 32. Service admission
also caps 128 active subscriptions per actor.

Session-cookie/bearer authentication applies. Content-only iframe tokens and
opaque `Origin: null` are rejected. Same-origin hosts are accepted; native/dev
origins (`tauri://localhost`, `http://tauri.localhost`,
`https://tauri.localhost`, `http://localhost:4173`) additionally require explicit
bearer/query credentials. `SHARD_HOST_ORIGINS` adds a comma-separated host allowlist.
Do not put session credentials into shard TSX or content URLs. Browser WebSocket
query credentials require access-log redaction and TLS outside the local sandbox.

The socket accepts subscribe/unsubscribe, acknowledges each subscription, then
pushes events/errors with a complete identity tuple. Each stream has one pending
snapshot; writes have a five-second deadline. Credentials are revalidated every
ten seconds; ownership/release/capabilities are checked at each refresh. The
frontend checks account-token changes every second and before/after HTTP calls.
Revocation detection is bounded by those intervals, not instantaneous invalidation.

## Shared MCP and catalog

`find_shard_resources`, `describe_shard_resource`, `read_shard_resource`,
`query_shard_resource` and `mutate_shard_resource` use the same service with agent
capabilities and the published environment. No iframe needs to be open. Mutations
require a current contract hash, request ID and, for update/delete, base revision.
Discovery projects current published metadata and reauthorizes each result;
records are read directly from the provider. Missing/revoked candidates are skipped;
availability/rate failures are surfaced instead of appearing as an empty catalog.
There is no per-shard MCP server and no MCP Apps rendering protocol here.

## Limits and operating constraints

Default limits: 64 KiB record data, 16 MiB active data per shard/environment,
10,000 stored records including tombstones, 10,000 retained command receipts,
32 MiB retained receipt bytes, 1,024 cursors per shard/environment, 500 records
per view and a 1 MiB wire-frame cap. Responses reserve space for their envelopes.
A record-count or receipt quota may be reached before the active-byte quota.
Request admission is 64 calls/second/actor with a burst of 256 per service instance
(not a cluster-wide rate limiter); its actor map caps at 1,024 entries. Internal
subscription refreshes do not consume that command budget. Host/iframe pending
requests cap at 32; host outbound buffering caps at 4 MiB and 1 MiB per message.

A captured code build is at most 10 MiB. Retained contracts/build files cap at
128 releases and 128 MiB of serialized stored data per shard/environment. Admission
and quota checks share the namespace lock; rejected stages roll back. No automatic
garbage collection deletes active or rollback candidates. Operators must review
inactive retention before admitting further builds at that cap.

The process-wide compiled data-schema LRU retains at most 256 validators and
4 MiB of source schema bytes. This bounds source retention, not measured heap;
full provider-memory and aggregate-refresh soak evidence remains a rollout gate.

Expired receipts/cursors are pruned on namespace activity; dormant namespaces
remain bounded but need an eventual maintenance job. Tombstones are retained;
retention/export/purge policy is not implemented. Receipt quotas reserve space
for a maximum-size response before committing a write.

Current SQL uses the namespace/ID index and parameterized JSONB expressions.
The sandbox benchmark at 10,000 small records, filtering/numeric sorting and
validating 500 returned records, observed about 24 ms p95 over 20 reads. It is not
a production SLO or evidence for maximum-size records/many concurrent views.
Per-field expression indexes and staging query-plan admission are not implemented.
Namespace operations serialize; v2 writes also use the existing per-user outbox
lock. Profile larger/combined workloads before increasing caps or broad rollout.

## Export and restore

The internal `ResourceArchiveStore` streams a newline-delimited JSON archive
under the namespace lock. It retains record IDs, revisions, data, tombstones,
audit fields and command receipts; the footer checks counts and SHA-256. Export
is owner/read-scope protected, restore owner/write-scope protected, and content
tokens are denied. The checksum detects corruption; it is not an authenticity
signature. Keep archives in trusted, access-controlled storage.

Restore requires an empty namespace and matching user/shard/environment,
generation and contract. It validates schemas/quotas and commits everything or
nothing. An archive with retired, undeclared datasets needs an explicitly reviewed
mapping; this strict restore will reject it. Cursors are not restored. Code,
contract release rows, vendor dependencies and the wider sync outbox need their
own backup; this is not a full database disaster-recovery product. Internal callers
must set deadlines since exports hold a namespace lock while streaming.

The archive test covers corrupt-input rollback, tombstones, receipt replay after
restore, exact round-trip checksums and refusal to overwrite accepted writes.
No bridge/MCP endpoint exposes restore or an unsafe rollback shortcut. See
MIGRATION.md for the still-required freeze/drain/cutover coordinator.

## Verification

See ROLLOUT.md for commands, measured evidence and remaining gates. All database
and browser verification uses the isolated test stack, not real dev/production.
The release matrix is not complete until native-host, fault/load and authorized
staging checks pass. Existing-shard conversion and v1 retirement are not implied.
