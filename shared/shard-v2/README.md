# Shard v2 implementation

Implements the contract, client library, protected host bridge, MongoDB and PostgreSQL
document adapters, immutable publication, persisted GraphQL operations, hot-swappable
TypeScript resolvers, manual lambdas and shared MCP access for **Shard v2 — Implementation
Specification** (Aladin artifact `artifact-45ec2ecd-7e92-4ff9-986d-830a1ab297a6`,
design revision 1.0).

V2 is behind `SHARD_V2_ENABLED=1`, off by default. Existing shards remain on
bridge/1. PostgreSQL remains the release/control plane and compatibility datastore.
MongoDB is the default owned-data engine and requires `SHARD_MONGODB_URI` (plus an
optional `SHARD_MONGODB_DATABASE`). Set `SHARD_DATASTORE=postgres` explicitly for
the compatibility adapter. No existing shard is converted automatically.

Copilot and MCP clients discover capabilities through `get_authoring_guide`.
New apps automatically receive the configured data API and starter files; guides
for existing apps preserve their storage model. Runtime version selection is an
operator concern, not an authoring question presented to the user.

- [AUTHORING.md](AUTHORING.md): complete contract, anchors and component example.
- [CLIENT.md](CLIENT.md): kit exports, subscriptions, mutations and pagination.
- [BACKEND.md](BACKEND.md): publication, transports, storage and operating limits.
- [ROLLOUT.md](ROLLOUT.md): verification evidence and outstanding release gates.
- [MIGRATION.md](MIGRATION.md): read-only inventory and safe cutover requirements.

## Code map

- `schemas/`: generated JSON Schema 2020-12 protocol documents.
- `fixtures/validation.json`: shared accepted/rejected examples consumed by Go
  and TypeScript. `fixtures/stream.json`: client reducer continuity trace.
- `backend_v2/internal/shardv2`: dependency-neutral compiler, schema/profile and
  event/query validation. Provider profiles are injected by the caller.
- `backend_v2/internal/service/shard_resources.go`: shared authorization,
  parameter resolution, projection and read/write entry points.
- `backend_v2/internal/repo/shard_resource_*.go`: MongoDB/PostgreSQL document
  storage, guarded mutations, receipts, queries, protected builds and recovery.
- `services/shard-runtime`: release-scoped Node workers that validate GraphQL,
  run compiled TypeScript resolvers/lambdas, enforce budgets and drain old releases.
- `backend_v2/internal/api/shard_resources.go`: HTTP/WS using the shared bridge
  envelopes; `fixtures/api.json` freezes the transport mapping.
- `backend_v2/internal/docsurface`: opt-in contract/anchor checks, embedded kit
  client and real-resource preview; published code is pinned to its contract.
- `aladin_react/src/modules/doc-surface/data`: framework-independent reducer,
  session client, bridge adapter, and a small React adapter. See [CLIENT.md](CLIENT.md).

`generate.py` is a build-time Python-stdlib script. It defines the protocol
schemas, writes their JSON files and the Go embedded schema map, runs `gofmt`,
and computes SHA-256 checksums of schemas and fixtures. It does not generate
application components, execute schemas, or run in a shard. Edit the generator
for protocol changes; edit fixtures directly, then regenerate.

From the repository root:

```sh
python3 shared/shard-v2/generate.py
cd backend_v2
GOCACHE="$PWD/.gocache" go test ./internal/shardv2
```

From `aladin_react`:

```sh
./node_modules/.bin/tsc -b
npm test -- --run src/test/shard-v2-contract.test.ts src/test/shard-v2-provider.test.ts src/test/shard-v2-bridge.test.ts
```

The Go fixture test also verifies manifest checksums and embedded schema parity.
Regeneration must be deterministic. No database or network is needed for these
tests. The broader docsurface suite includes isolated Chromium tests.

## Contract choices made explicit

- Every resource declares `snapshot`; exposure defaults to no capabilities.
  A declared capability must be supported by its registered provider. This is
  compile-time validation, not authorization of a user or an operation.
- Source version defaults to the registered v1 provider version. Owned datasets
  are declared once per contract; external providers cannot select an owned dataset.
- Resource/binding IDs use the spec identifier grammar. Records retain opaque
  string IDs and revisions; singleton ID is `value`. Event sequence numbers are
  decimal strings and are compared exactly with `BigInt` in the client.
- Author schemas have object roots. References are local and acyclic. Reference
  siblings are annotations/definitions only. `format` is an annotation, not a
  data validator. Unsafe integers, non-finite values, unsupported keywords and
  remote references are rejected. JSON depth is at most 64; schema nodes at most
  1024. Protocol JSON is bounded to 1 MiB and record data to 64 KiB.
- Binding params accept literal JSON, `{input: "name"}`, or
  `{binding: "singletonBinding", pointer: "/data/field"}`. An object using a
  reserved key (`input`, `binding`, `literal`) can be escaped as `{literal: ...}`.
  Declared inputs and dependency pointers are checked at compile time; resolved
  values must still be validated against provider params on every backend call.
- Bindings are bounded to 128, active client subscriptions to 32. View limits
  default to 100 and cannot exceed 500. Predicate depth is 8 and leaf count 32.
- Projections preserve nested required fields and reject overlapping paths.
  The backend must perform projection; a client validator does not hide data
  already delivered. Projected records are not full replacement write payloads.
- Descriptor `delivery` explicitly distinguishes default ID-ordered deltas from
  replacement snapshots for filtered/sorted views. `refresh-snapshots` observation
  cannot claim delta delivery.
- The bridge transport uses request/response envelopes and push channels
  `resource.event`, `resource.error`, `resource.status`. Control pushes carry the
  complete subscriptionId/resource/epoch tuple, plus code/message or status.
  The backend adapter must implement this shape; existing bridge/1 does not.
- `sourceUpdatedAt` is optional provider metadata. Missing source freshness stays
  unknown; the client never substitutes a local receipt timestamp for it.

## Validator and CSP decision

TypeScript uses AJV 2020-12; Go uses the existing `google/jsonschema-go` dependency.
AJV runtime compilation requires JavaScript string evaluation. The user approved
relaxing that CSP restriction. `CSPForBridgeVersion` adds `unsafe-eval` only for a
protected bridge/2 release selection, after vendor-origin augmentation. It is
selected by content serving and preview only after resolving a protected v2 release.

This permission applies to **all code in the v2 iframe**, not just AJV. It leaves
v1 CSP, opaque-origin isolation, network policy, and host authorization unchanged.
An author file or iframe parameter must never select the policy.

## Generation and implementation boundary

Run `node shared/shard-v2/bundle-client.mjs` from the repository root after client
changes (requires installed frontend dependencies). It bundles the tested client
into the Go-embedded kit asset; it does not copy frontend public assets. The kit
vendor hash includes this generated asset. Do not edit generated JS by hand.

UI, authored resolvers and MCP call the same authorized service. Owned records use
opaque per-user/shard/environment MongoDB collections when Mongo is configured;
registered external sources execute in the backend. Resolver workers receive
signed capabilities and never receive database credentials or namespace selectors.
Neither authors nor iframe requests choose credentials, backend endpoints,
environment, or release grants. Complex analytics require another registered
backend provider; this release does not offer arbitrary SQL or arbitrary URLs.

The implementation is ready for further sandbox/staging validation, not broad
rollout. The outstanding native-host, load/fault and migration gates are tracked
in ROLLOUT.md. Migration and v1 retirement remain separate outcomes.

Never reuse mutable draft manifest grants for published v2 operations, or enable
v2 serving/CSP based solely on the presence of `contract.json`.
