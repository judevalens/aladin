# Shard runtime

This sidecar runs immutable, release-scoped GraphQL resolvers and manual lambda
handlers compiled by the Go build pipeline. It is deliberately not a datastore
service. Workers receive a signed scope and can call only the binding operations
declared for the current handler; the Go API revalidates the signature, active
release, handler grant, audience and normal resource authorization on every call.

Required environment:

- `SHARD_RUNTIME_SECRET`: shared with the Go API, at least 32 bytes.
- `SHARD_CAPABILITY_URL`: the Go internal capability endpoint.
- `PORT`: defaults to `8092`.
- `HOST`: defaults to loopback; production Compose sets `0.0.0.0`.

The control API supports prepare, activate and remove plus persisted GraphQL
queries/mutations and manual lambda invocation. Resource subscriptions use the Go
WebSocket service; GraphQL subscription documents are not part of this transport.
It requires bearer authentication except for
`GET /healthz`. `prepare` imports the bundle, builds the schema, validates every
persisted operation and checks all declared exports. `activate` swaps the scope's
worker synchronously and drains the old worker after in-flight requests finish.
Timed-out or periodically recycled workers are terminated and warmed again from
the active immutable release on the next request. Because one release worker can
serve several handlers, its heap uses the smallest declared memory limit.
Resolver timeouts are enforced per invocation and terminate
the worker. The parent also applies the strictest resolver timeout referenced by
each persisted operation, so synchronous authored code cannot block its own timer.
Operation and document counters remain per handler invocation.
The stricter shared heap is deliberately conservative and never widens an authored
memory budget.

Run tests with `npm ci && npm test`.
