This repository is now centered on the React frontend in `aladin_react` and the Go backend in `backend_v2`.

## Main App Surfaces

- `aladin_react/`
  The primary product UI. This is the frontend to use for shell, workspace, auth, sources, pages, and general product work.
- `backend_v2/`
  The Go API, worker, MCP server, and persistence layer.
- `aladin_ui/`
  Legacy Kotlin/Compose code that may still exist in the repo, but it is no longer the primary frontend direction.

## Common Commands

### React frontend

```sh
cd aladin_react
npm install
npm run dev
```

Other useful commands:

```sh
cd aladin_react
npm test
npm run build
```

### Go backend

```sh
cd backend_v2
go run ./cmd/api
```

### Worker

```sh
cd backend_v2
go run ./cmd/worker
```

## Current Frontend Direction

- React/TypeScript is the active UI surface.
- New frontend work should target `aladin_react` unless there is a specific reason to touch a legacy surface.
- Docs that still mention Kotlin/Wasm or Compose should be treated as historical unless they explicitly say otherwise.
