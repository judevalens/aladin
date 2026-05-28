# services/blocknote

Combined Node service for Aladin's BlockNote needs. Single Express process,
layered (`src/handlers` → `src/services`, with `src/middleware` for
cross-cutting concerns). One container, one health check.

Roles by milestone:
- **M8a (now):** markdown ↔ BlockNote blocks conversion (formerly
  `blocknote-converter`).
- **M8b:** Hocuspocus mounted on `/collaboration` for real-time Y.Doc sync.
- **M8c:** MCP admin bridge on `/admin/apply`.

## Endpoints (M8a)

- `POST /md-to-blocks` `{markdown: string}` → `{blocks: Block[]}`
- `POST /blocks-to-md` `{blocks: Block[]}` → `{markdown: string}`
- `POST /blocks-to-md-batch` `{blocks: Block[][]}` → `{markdowns: string[]}`
- `GET  /healthz` → `ok`

## Layout

```
server.js              boots Express + middleware, wires routes
src/config.js          env parsing
src/handlers/          thin: validate → call service → shape response
src/services/          actual logic (converter; collab/auth land later)
src/middleware/        error-boundary (never crash on handler errors), trace
test/                  node --test
```

## Failure handling

Every route handler is wrapped by `src/middleware/error-boundary.js` `wrap()`:
sync throws and async rejections both route to the error handler (500, or the
error's `statusCode`) — a handler bug never crashes the process. Process-level
`uncaughtException` / `unhandledRejection` log and exit so Docker restarts a
clean process (an uncaught exception leaves Node in an indeterminate state).

## Local dev without Docker

```
cd services/blocknote
npm install
node server.js     # listens on :3500 by default
npm test           # node --test
```

## Version pinning

`@blocknote/*` (and, from M8b, `yjs` / `@hocuspocus/*`) must be pinned to the
exact versions in `aladin_react/package.json`. CI gate: `scripts/check-blocknote-versions.sh`.
