# Aladin

Aladin is currently a personal trading research workspace: the layer around
backtesting that preserves what I believed, what I tested, what happened, and what I
learned.

The canonical product baseline is [`CURRENT_PRODUCT.md`](./CURRENT_PRODUCT.md).
When older docs still describe Aladin as a generic knowledge graph or broad proactive
AI workspace, treat those docs as historical unless they explicitly say otherwise.

This repository is centered on the React frontend in `aladin_react` and the Go backend in `backend_v2`.

## Main App Surfaces

- `aladin_react/`
  The primary product UI. This is the frontend to use for shell, workspace, auth,
  markets, sources, pages, and general product work.
- `backend_v2/`
  The Go API, worker, MCP server, and persistence layer.

## Current Product Direction

- Active product: trading research workspace.
- Core loop: hypothesis -> strategy version -> backtest runs -> live scan signals
  -> trades -> journal notes.
- Supporting substrate: sources, documents, entities, search, Copilot, realtime sync,
  and shards when they serve the research loop.
- Parked surfaces: generic graph-first product, standalone broad insights, Tutor, and
  workspace-wide graph exploration.

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

## Frontend Direction

- React/TypeScript is the active UI surface.
- New frontend work should target `aladin_react` unless there is a specific reason to touch a legacy surface.
- Parked surfaces should stay hidden from primary navigation unless they are being
  actively revived for the trading research loop.
- Docs that still mention Kotlin/Wasm, Compose, or the old graph-first product should
  be treated as historical unless they explicitly say otherwise.
