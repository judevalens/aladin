// Central env parsing.
export const config = {
  port: Number.parseInt(process.env.PORT ?? "3500", 10),
  jsonLimit: process.env.JSON_LIMIT ?? "10mb",

  // M8b: Hocuspocus runs its own HTTP+WS server on a separate port (it
  // bundles ws@7; the converter's Express stack uses ws@8 — keeping them on
  // separate servers avoids the in-process socket version mismatch).
  collabPort: Number.parseInt(process.env.COLLAB_PORT ?? "3501", 10),

  // M8b: Hocuspocus collab.
  // Postgres holding the canonical page_ydoc state. Default targets the
  // host-mapped port for `node server.js`; docker-compose overrides with the
  // in-network host:port.
  databaseUrl:
    process.env.BLOCKNOTE_DATABASE_URL ??
    "postgres://aladin:password@localhost:5433/aladin",
  // Go API endpoint that resolves a bearer token / session to a principal.
  authResolveUrl:
    process.env.BLOCKNOTE_AUTH_RESOLVE_URL ??
    "http://localhost:8000/api/auth/resolve",

  // M8c: shared secret guarding the MCP admin bridge (/admin/*). Defaults to a
  // known local-dev value so `make backend` + MCP work out of the box; set
  // BLOCKNOTE_ADMIN_SHARED_SECRET in Docker/prod. Empty string disables the
  // bridge (fail closed).
  adminSharedSecret:
    process.env.BLOCKNOTE_ADMIN_SHARED_SECRET ?? "local-dev-admin-secret",

  // Hocuspocus store debounce (write-efficiency, not durability — clients +
  // y-indexeddb are the durability layer).
  storeDebounceMs: Number.parseInt(process.env.STORE_DEBOUNCE_MS ?? "2000", 10),
  storeMaxDebounceMs: Number.parseInt(
    process.env.STORE_MAX_DEBOUNCE_MS ?? "10000",
    10,
  ),

  // M8.7 JSON projection debounce (collab.js): coalesce page_documents writes
  // to at most once per PROJECTION_DEBOUNCE_MS AND once per PROJECTION_MAX_COMMITS,
  // with a PROJECTION_SWEEP_MS fallback flush for restart-orphaned timers.
  projectionDebounceMs: Number.parseInt(
    process.env.PROJECTION_DEBOUNCE_MS ?? "500",
    10,
  ),
  projectionMaxCommits: Number.parseInt(
    process.env.PROJECTION_MAX_COMMITS ?? "50",
    10,
  ),
  projectionSweepMs: Number.parseInt(
    process.env.PROJECTION_SWEEP_MS ?? "5000",
    10,
  ),
};
