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

  // M8c: shared secret guarding the MCP admin bridge (/admin/*). Empty here;
  // set in M8c.
  adminSharedSecret: process.env.BLOCKNOTE_ADMIN_SHARED_SECRET ?? "",

  // Hocuspocus store debounce (write-efficiency, not durability — clients +
  // y-indexeddb are the durability layer).
  storeDebounceMs: Number.parseInt(process.env.STORE_DEBOUNCE_MS ?? "2000", 10),
  storeMaxDebounceMs: Number.parseInt(
    process.env.STORE_MAX_DEBOUNCE_MS ?? "10000",
    10,
  ),
};
