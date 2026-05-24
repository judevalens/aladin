---
name: aladin-go-conventions
description: "Project structure, module boundaries, and layering conventions for the Aladin Go backend, using subdomain-first organization inside each domain with service-by-subdomain, repo-by-subdomain, and api-by-subdomain packages. Use this whenever working on the Aladin backend in Go: adding new packages or files, deciding where code belongs, creating API endpoints, services, or repositories, defining types, wiring binaries, or auditing and refactoring existing code toward the conventions. Trigger this for placement requests like \"add an endpoint for X\" or \"where should this query go?\" and refactor requests like \"this package feels off\", \"audit internal/ingest\", or \"fix the layering in a specific file\"."
---

# Aladin Go Backend Conventions

Use this skill to decide where Aladin backend Go code belongs and how to move existing code toward the repo's intended structure.

## Two modes

Choose one mode before giving guidance or changing code:

1. Placement mode: the user is adding or placing new code.
2. Refactor mode: the user wants an audit, plan, or structural fix for existing code.

If the request is ambiguous, ask which mode they want. A request like "fix the ingest package" is not specific enough to assume a broad refactor.

For refactor mode, read [references/refactoring.md](references/refactoring.md) before producing a plan or touching code.

## Repo shape

```text
aladin/
├── cmd/                       # Binaries, one subdirectory per executable
│   ├── api/                   # HTTP API server
│   ├── ingest-worker/         # Reddit/Bluesky polling + ingest pipeline
│   └── insight-worker/        # Insight generation pipeline
├── internal/
│   ├── domain/                # Shared types (entities, signals, insights, etc.)
│   ├── ingest/                # Ingest domain
│   │   ├── service/           # Business logic, organized by subdomain
│   │   ├── repo/              # Data access, organized by subdomain
│   │   └── api/               # HTTP and async entry points, organized by subdomain
│   ├── insights/              # Same pattern: service/, repo/, api/
│   ├── graph/
│   └── signals/
├── pkg/                       # Currently empty. Don't put things here yet.
└── go.mod
```

## Organizing axes

Apply these in order:

1. Domain first: what product area does the code serve? (`ingest`, `insights`, `graph`, `signals`)
2. Layer second: what role does it play inside that domain? (`service`, `repo`, `api`)
3. Subdomain third: what feature slice inside that domain does it belong to?

Do not flatten domains into top-level layer packages, and do not skip layer boundaries inside a domain.

## Rules in priority order

### 1. Put new code in `internal/`

Do not add to `pkg/` unless there is an explicit decision that another module outside this repo needs to import it.

If something feels "library-ish", keep it in `internal/` anyway. Promote it later only if import pressure justifies it.

### 2. Organize `internal/` by domain

Top-level packages under `internal/` should be product domains:

- `internal/ingest`
- `internal/insights`
- `internal/graph`
- `internal/signals`

Avoid top-level graveyard packages like:

- `internal/handlers`
- `internal/services`
- `internal/repos`
- `internal/utils`
- `internal/common`

### 3. Organize each domain into layers and subdomains

Each domain package uses:

```text
internal/<domain>/
├── service/
│   └── <subdomain>/   # Business logic for one feature slice
├── repo/
│   └── <subdomain>/   # Data access for one feature slice
└── api/
    └── <subdomain>/   # HTTP or async entry points for one feature slice
```

Avoid dumping unrelated files directly under `service/`, `repo/`, or `api/` when they clearly belong to a narrower slice. Prefer subdomain packages such as `api/reddit`, `service/reddit`, or `repo/reddit`.

### 4. Keep APIs thin

An API package, whether HTTP or async, does only this:

1. Parse the request or job payload.
2. Validate input shape.
3. Call a service method.
4. Translate the service result back to the wire format.
5. Return.

API entry points must not:

- import `repo` directly
- contain business logic
- coordinate multi-step workflows
- decide retries, transactions, or side effects

Apply the same rule to HTTP handlers and async job handlers.

### 5. Put business logic in services

Services:

- coordinate multi-step operations
- enforce business rules and invariants
- decide read/write order
- own transaction boundaries
- provide the boundary other domains call through
- coordinate logic across subdomains inside the same domain when needed

Services depend on repository interfaces, not concrete repository types.

Define those interfaces in the consumer package, usually `service`, not in `repo`.

```go
// internal/ingest/service/service.go
package service

type RedditRepo interface {
    SaveCursor(ctx context.Context, sub string, cursor string) error
    LoadCursor(ctx context.Context, sub string) (string, error)
}

type Service struct {
    reddit RedditRepo
}
```

```go
// internal/ingest/repo/reddit.go
package repo

type RedditRepo struct { /* ... */ }

func (r *RedditRepo) SaveCursor(...) error { /* ... */ }
func (r *RedditRepo) LoadCursor(...) (string, error) { /* ... */ }
```

### 6. Keep repositories focused on data access

A repository:

- talks to one external system
- translates between domain types and storage formats
- returns concrete structs

A repository must not:

- contain business-logic conditionals
- call services
- import another domain's packages

If swapping Redis or Neo4j for an in-memory implementation would require service changes, business logic has leaked into the repo.

### 7. Make cross-domain calls service-to-service

When one domain needs another, define an interface in the consuming service package and wire the concrete implementation in `cmd/`.

```go
// internal/insights/service/service.go
type GraphReader interface {
    EntitiesForTopic(ctx context.Context, topic string) ([]domain.Entity, error)
}
```

Do not import another domain's `repo` package directly.

### 8. Place shared types in `internal/domain` only when earned

Use this escalation rule:

1. Start with the layer that first needs the type.
2. If a second layer in the same domain needs it, move it to a domain-level file like `internal/ingest/types.go`.
3. If a second domain needs it, move it to `internal/domain`.
4. Do not promote types early.

`internal/domain` should stay types-only and should not import other `internal/*` packages.

### 9. Keep `cmd/` thin

Each binary should:

1. Load config.
2. Construct dependencies.
3. Construct repos, then services, then API adapters.
4. Wire routes or workers.
5. Start the binary.

Do not put business logic in `cmd/`.

Wire subdomain packages explicitly. `cmd/` should construct the concrete repo implementation for a subdomain, pass it into that subdomain's service, then pass the service into that subdomain's API adapter.

## Placement flow

For "where does this go?" requests, walk this order and stop at the first match:

1. Is it a new binary? Put it under `cmd/`.
2. Is it a type already used by two or more domains? Put it under `internal/domain`.
3. Pick the correct domain under `internal/<domain>/`.
4. Pick the subdomain inside that domain.
5. Classify the code:
   - request or payload parsing: `api/<subdomain>/`
   - decision-making or orchestration: `service/<subdomain>/`
   - storage or external API access: `repo/<subdomain>/`
6. If it still seems cross-cutting, prefer a clearly named `internal/<domain-like-name>/` package such as `internal/logging`, not `internal/utils`.
7. If tempted to use `pkg/`, do not.

## Anti-patterns to flag

- An `api/<subdomain>` package importing `repo/<subdomain>` directly
- A repo method containing business-logic conditionals
- A service depending on a concrete repo type
- A repo defining the interface consumed by a service
- Cross-domain imports of `repo` packages
- `internal/utils`, `internal/common`, `internal/shared`, `internal/helpers`
- A type in `internal/domain` used by only one domain
- `cmd/<binary>/main.go` doing real work
- A new top-level directory next to `cmd/`, `internal/`, `pkg/`
- An async API handler with multi-step logic in it

## Refactor mode

When the user asks for an audit, refactor, or structural fix:

1. Read [references/refactoring.md](references/refactoring.md).
2. Scope strictly to what the user pointed at.
3. Audit against the rules in this file.
4. Produce a prioritized plan.
5. Get approval before editing.
6. Execute step by step, keeping the build mostly green.

Take the example refactor walkthrough in the reference file seriously. Use that structure for small API-thickening problems too, not just broad package audits.

## When in doubt

Prefer the choice that:

1. keeps `internal/domain` small
2. keeps APIs thin
3. keeps services free of direct external-system dependencies
4. keeps `cmd/` limited to wiring
