# Aladin

**An AI-assisted research workspace for connecting source material, notes, and interactive analysis.**

Aladin brings research materials into a shared workspace: folders organize an investigation, pages capture reasoning, uploaded files hold evidence, and **shards** are small interactive applications embedded alongside the work. It combines a React/TypeScript interface with a Go backend and a Tauri desktop client.

The project explores how research can accumulate over time: connecting mentions of the same entity across sources, retaining the underlying data behind an analysis, and keeping clients synchronized as that information changes. Market data is one supported research domain.

**Status:** actively developed. The repository contains the workspace, ingestion and entity-resolution systems, market-data services, and desktop synchronization. Git-backed artifact publishing and a portable experiment ledger are the next direction and are being implemented in stages.

## What is in the workspace

- **Research materials:** folders, editable pages, uploaded files, and embedded interactive applications.
- **Source ingestion:** background processing, enrichment, and entity linking across collected records.
- **Market analysis:** instrument and bar data, corporate-action adjustments, watchlists, and market-data updates.
- **Collaboration and agents:** collaborative editing, an in-app Copilot, and an MCP server exposing workspace tools.
- **Desktop persistence:** local SQLite storage with live updates and incremental synchronization.

### VIDEO DEMO
To see a video of the project: https://streamable.com/jm9hwa

## Engineering highlights

### Entity resolution across sources

Different spellings can refer to the same entity, while identical names can refer to different concepts. The resolver combines normalized-name lookup, trigram matching, context embeddings, and optional LLM adjudication. Context can separate different senses of the same name; fuzzy matches produce merge proposals for review. Rejected matches provide negative evidence for later decisions.

Embedding or adjudication failures fall back to deterministic matching. Aliases and mentions retain the connection between an entity and its source records. The entity layer remains part of the ingestion system; a general graph-exploration interface is currently parked.

Read the [resolver](backend_v2/internal/entities/resolver.go) and its [behavioral tests](backend_v2/internal/entities/resolver_test.go), including ambiguous names, proposed merges, and model failures.

### Recovery from interrupted ingestion

Saving a record and dispatching its next job are separate operations. A process can stop between them, leaving durable data with no queued work. The pipeline uses deterministic task identifiers and idempotent data operations, with a recovery worker that finds stalled records and dispatches their next stage again.

Uploaded-document extraction uses a separate database-driven sweeper: pending work is derived from stored artifacts, so extraction does not depend on receiving an upload event.

Read the [pipeline recovery worker](backend_v2/internal/pipeline/reaper.go), [capture-boundary tests](backend_v2/internal/repo/record_reliability_test.go), and [document sweeper](backend_v2/internal/ingestion/sweeper.go).

### Live updates with durable catch-up

The backend publishes workspace changes through a PostgreSQL outbox. The desktop applies live events for responsiveness and pulls incremental changes to catch up after interruptions. Per-entity sequence guards handle duplicate or older updates.

Only the pull path advances the durable synchronization cursor. It applies a batch and its cursor in one SQLite transaction, so a failed batch cannot move the client past unapplied data.

Read the [backend outbox](backend_v2/internal/outbox/postgres.go), [workspace sync service](backend_v2/internal/workspacesync/), and desktop [pull](aladin_react/src-tauri/src/sync/pull.rs) and [live-event](aladin_react/src-tauri/src/sync/live.rs) paths. The pull implementation includes tests for replay and failure handling.

### Raw market data with adjustments at read time

Market bars are stored in their original form. Split- and dividend-adjusted series are derived at read time by applying corporate actions, preserving the raw observations and keeping the adjustment logic independently testable.

The tests cover ex-date boundaries, reverse splits, compounded adjustments, and input preservation. These are market-data foundations; the broader experiment and backtesting workflow remains under development.

Read the [adjustment implementation](backend_v2/internal/market/bar_adjust.go) and [tests](backend_v2/internal/market/bar_adjust_test.go).

## Architecture

```mermaid
flowchart LR
    UI[React / TypeScript] <--> API[Go API]
    UI <--> Desktop[Tauri / SQLite]
    Desktop <-->|Live events and incremental pull| API
    API <--> PG[(PostgreSQL / pgvector)]
    Worker[Go workers] <--> PG
    Worker <--> Queue[(Redis / Asynq)]
    Sources[External sources] --> Worker
    Worker --> Graph[(Optional Neo4j projection)]
    UI <--> Collab[Node collaboration service]
    Agents[Agent clients] <--> MCP[Go MCP server]
    MCP <--> PG
```

The diagram shows the main data paths. Collaboration, Copilot, and shard execution have separate Node services. The opt-in shard v2 runtime adds versioned contracts and MongoDB or PostgreSQL document storage; see its [implementation guide](shared/shard-v2/README.md).

| Location | Responsibility |
| --- | --- |
| [aladin_react](aladin_react/) | Primary React interface, state management, and frontend tests |
| [aladin_react/src-tauri](aladin_react/src-tauri/) | Rust desktop host, SQLite persistence, and sync |
| [backend_v2](backend_v2/) | Go API, MCP server, workers, persistence, and migrations |
| [services/blocknote](services/blocknote/) | Page conversion, collaborative editing, and board synchronization |
| [services/copilot-agent](services/copilot-agent/) | Agent execution and streamed Copilot responses |
| [services/shard-runtime](services/shard-runtime/) | Opt-in shard v2 execution service |
| [shared/shard-v2](shared/shard-v2/) | Shared contracts, schemas, and fixtures |
| [docs](docs/) | Subsystem documentation and operating guides |

## Local development

The services run separately during development. Start with the API and browser interface, then enable the integrations needed for your workflow.

### Prerequisites

- Go 1.25 or later, as declared in [go.mod](backend_v2/go.mod).
- Node.js 24 and npm for the frontend and Node services.
- Docker with Compose for local databases and queues.
- Rust and the platform build dependencies for Tauri if running the desktop client.

### API and browser interface

From the repository root, start the core database and queue:

```sh
docker compose up -d postgres redis
```

Create or update your local `backend_v2/.env` with the development database connection and API port:

```dotenv
DATABASE_URL=postgres://aladin:password@localhost:5433/aladin
API_ADDR=:8000
REDIS_URL=redis://localhost:6379
```

These values correspond to the local [Compose configuration](docker-compose.yml). The API loads this file and applies database migrations at startup. See [configuration loading](backend_v2/internal/config/config.go) for additional settings.

Start the API:

```sh
cd backend_v2
go run ./cmd/api
```

In a separate terminal, start the frontend:

```sh
cd aladin_react
npm ci
npm run dev
```

Open **http://localhost:4173**. Vite proxies API requests to port 8000. This starts the base services; collaborative editing, enrichment, Copilot, and external integrations require their corresponding services and configuration.

### Additional services

| Capability | Setup |
| --- | --- |
| Collaborative pages and boards | Install dependencies in `services/blocknote`, then use `make blocknote`; see the [service README](services/blocknote/README.md). |
| Background enrichment | Set `OPENAI_API_KEY` in `backend_v2/.env`, then run `go run ./cmd/worker` from `backend_v2`. PostgreSQL and Redis must be running. |
| Graph projection | Start the `neo4j` Compose service and configure `NEO4J_URI`, `NEO4J_USER`, and `NEO4J_PASS` for the worker. |
| Desktop client | Run `npm run tauri:dev` from `aladin_react` with the backend services running. |
| MCP and Copilot | Install dependencies in `services/copilot-agent`, configure the provider credentials and shared secret, then use `make mcp` and `make copilot-agent`. See the [launcher settings](Makefile) and [configuration loader](backend_v2/internal/config/config.go). |
| Provider connections | Follow the [Nango integration guide](docs/NANGO_PROVIDER_CONNECTIONS.md). |
| Shard v2 | Follow the [runtime setup](shared/shard-v2/README.md); it is disabled by default. |

Run `make help` for available commands. The [local operations guide](docs/DEV_OPS_HARNESS.md) covers queue inspection, stream status, and recovery tools.

## Tests and builds

Frontend tests and production build:

```sh
cd aladin_react
npm test
npm run build
```

Go tests with the repository's isolated PostgreSQL test stack, from the repository root:

```sh
make test-go
```

This target starts test PostgreSQL on port 5444 and serializes Go test packages because some integration tests share database state. Tests for optional services may require additional configuration.

## Product direction

The next phase connects research folders to Git repositories, supports revision-aware artifact publishing, and preserves portable experiment records with their inputs and outcomes. External agents retain their own development environments; Aladin organizes and renders the research record.

Product documentation is being updated to reflect this broader research direction. Older trading and graph-oriented designs remain in the repository as design history; check implementation details against the linked code and tests above.
