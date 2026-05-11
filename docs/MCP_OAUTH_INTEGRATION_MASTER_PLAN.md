# MCP + OAuth Integration Master Plan

## Status

Planning document.

Aladin now has the beginning of a real auth boundary: email/password login, opaque browser sessions, and current-user context propagation for source subscriptions. The next strategic step is to make external integrations first-class instead of bolting MCP or OAuth onto the side.

This plan defines the dependency order for adding:

- MCP servers for agent-authored notes
- OAuth/private provider accounts such as Gmail, Threads, Google Drive, and future social APIs
- DB-backed integration tokens
- capability-scoped external actors

The goal is to avoid parallel auth systems. Browser sessions, OAuth credentials, and MCP/API tokens should resolve into the same principal model: an actor acting for a user with a bounded set of capabilities.

JWT is intentionally out of scope for now. Aladin does not need stateless bearer tokens while one backend validates browser sessions and MCP/API tokens. Opaque DB-backed tokens give simpler revocation, audit, scope changes, and local-dev behavior. JWT can be reconsidered later for mobile or service-to-service use if stateless validation becomes a real requirement.

## Product Goal

Aladin should become a durable workspace that external agents can write to and read from.

Near-term example:

> Claude/Codex creates a markdown note in Aladin through MCP, and the note appears in the existing workspace UI as a normal page artifact with agent metadata.

Longer-term:

- Gmail sync runs against a user-owned OAuth credential.
- Threads keyword search runs against a user-owned OAuth credential and quota.
- MCP agents can create, update, and search notes without full app privileges.
- Private-source ingestion, agent writing, and future copilot actions all use the same ownership and permission boundary.

## Core Principle

Do not create one-off auth paths.

Bad v1 shortcut:

```text
MCP_AUTH_TOKEN env var
    -> write as default dev user
```

Preferred v1:

```text
Bearer token
    -> token hash lookup
    -> integration principal
    -> user context + scopes
    -> normal service calls
```

This is only slightly more work, but it avoids rebuilding MCP auth once OAuth/private sources arrive.

The auth spine is:

```text
Browser session cookie -> DB session -> Principal
MCP/API bearer token -> DB integration token -> Principal
OAuth provider token -> stored credential, not Aladin app auth
```

All protected product behavior should depend on the resolved `Principal`, not on the transport that produced it.

## Target Concepts

### User

The human account that owns workspace data, credentials, source subscriptions, and agent-created artifacts.

Current state:

- `users`
- `user_sessions`
- browser login/register flow

### Principal

A runtime identity resolved from a request.

Possible principal kinds:

- `user_session`: browser user session
- `integration_token`: MCP or personal access token
- `oauth_callback`: short-lived OAuth state/callback resolution
- future: internal system worker, app-level copilot, mobile client

Principals should carry:

- `user_id`
- `actor_type`
- `actor_id`
- `scopes`
- optional display label

### Provider Credential

A user-owned credential for an external provider.

Examples:

- Google account for Gmail or Drive
- Threads account
- future X/Bluesky/Reddit private credentials

Provider credentials are not principals by themselves. They are stored secrets used by syncers or provider clients after a user explicitly connects a provider.

### Integration Token

A DB-backed bearer token that lets an external actor call Aladin.

Examples:

- local MCP server token
- Claude/Codex token
- future personal access token
- future automation token

Integration tokens resolve to a user and a scope set.

### Scope

A coarse capability boundary.

Initial scopes:

- `artifacts:read`
- `artifacts:write`
- `sources:read`
- `sources:write`
- `insights:read`

MCP notes v1 should require:

- `artifacts:read`
- `artifacts:write`

## Milestone 1 — Finish Ownership Boundary

### Goal

Services that create or read user-owned data should get ownership from request context, not constructor-level default user ids.

### Status

Code-ready for MCP. `Principal` is implemented, API auth middleware injects browser-session principals, source subscriptions are user-scoped, and workspace artifact/page/file operations resolve ownership from request context. `defaultUserID` remains only as a dev/system fallback for realtime keys and seeded local data.

### Work

- Keep current browser auth as the user session model.
- Define a shared `Principal` shape in the service layer.
- Make browser sessions resolve into `Principal{UserID, ActorType: "user_session"}`.
- Extend auth context helpers beyond `CurrentUser` if needed:
  - `WithPrincipal(ctx, principal)`
  - `PrincipalFromContext(ctx)`
- Preserve `CurrentUserFromContext` as a convenience if useful.
- Add constants for well-known actor types and scopes, even if browser sessions initially have full access.
- Add service tests that prove missing principal returns `ErrUnauthenticated`.
- Move ownership slice-by-slice away from default dev user:
  - sources/subscriptions already started
  - artifacts/pages/folders next
  - files/audio resource access after artifacts
  - feed/insights when tenant-specific behavior is tightened

### Acceptance Criteria

- A protected request has one trusted user id in context. Implemented.
- Source APIs no longer rely on default dev user. Implemented.
- Artifact/page/file operations resolve request ownership from `Principal`. Implemented.
- Tests can inject a principal without constructing full HTTP auth. Implemented.

### Implementation Plan

1. Add `Principal` to `backend_v2/internal/service/auth.go` or a small service-level auth/principal file.
2. Add helpers:
   - `WithPrincipal`
   - `PrincipalFromContext`
   - `RequirePrincipal`
   - optionally keep `CurrentUserFromContext` backed by `Principal`.
3. Update API auth middleware to inject a session principal after session lookup.
4. Update `SourceService` to use `RequirePrincipal` rather than directly reading `CurrentUser`.
5. Refactor artifact repository/service ownership so artifact operations receive the principal user id instead of constructor `defaultUserID`.
6. Update wiring so `NewArtifactsPostgres` no longer hardcodes the dev user for authenticated paths.
7. Add/adjust tests for unauthenticated service calls and user-scoped artifact/source behavior.
8. Run:

```sh
cd backend_v2
GOCACHE=/Users/judepaulemon/Documents/aladin/backend_v2/.gocache go test ./...

cd aladin_ui
./gradlew :composeApp:compileKotlinWasmJs
```

## Milestone 2 — Integration Token Model

### Goal

Create the auth foundation MCP needs without using env-only static tokens.

### Status

Code-ready. Implemented in `00023_integration_tokens.sql`, `AuthService`, `PostgresAuthRepository`, and `/api/integration-tokens`.

### Proposed Data Model

```sql
integration_tokens (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    scopes TEXT[] NOT NULL,
    status TEXT NOT NULL,
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    last_used_at TIMESTAMPTZ
)
```

Token rules:

- Raw token is shown once at creation.
- DB stores only a hash.
- Revocation sets `revoked_at` or `status = 'revoked'`.
- Expired/revoked tokens do not resolve.
- `last_used_at` updates best-effort.

### Service API

- `CreateIntegrationToken(ctx, input) -> raw token + metadata`
- `ListIntegrationTokens(ctx)`
- `RevokeIntegrationToken(ctx, id)`
- `ResolveBearerToken(ctx, rawToken) -> Principal`

### Acceptance Criteria

- MCP can authenticate through DB-backed bearer tokens. Implemented through `AuthService.ResolveBearerToken`.
- Token scopes are available in the resolved principal. Implemented.
- Duplicate/invalid tokens fail safely with `401`. Implemented at service resolution boundary.
- No MCP token lives only in `.env`. Implemented.

## Milestone 3 — Capability Enforcement

### Goal

Prevent external actors from receiving full app access by default.

### Status

Code-ready for MCP. The generic helpers exist, artifact/page/file operations enforce `artifacts:read` / `artifacts:write`, and API routes map forbidden scope failures to `403`. The MCP server still needs to call `ResolveBearerPrincipal`, inject the resolved principal, and translate tool-level authorization failures into MCP errors.

### Work

- Add a small authorization helper:
  - `RequireScope(ctx, "artifacts:write")`
  - `HasScope(ctx, "artifacts:read")`
- Browser sessions can initially be treated as full app access.
- Integration tokens are scope-limited.
- MCP tool handlers must check scopes before service calls.

### Acceptance Criteria

- A token with only `artifacts:read` cannot create/update notes. Implemented at artifact/page/file services.
- A token with `artifacts:read/write` can use notes tools. Ready for MCP tools.
- Authorization failures return MCP tool errors, not panics. Pending MCP tool layer.

## Milestone 4 — Nango-Backed Provider Connections

### Goal

Use Nango Cloud as the default provider-connection backend for supported integrations. Aladin keeps users, sessions, source subscriptions, provider streams, records, matching, and insights. Nango owns OAuth authorization, credential storage, token refresh, and proxy/token retrieval.

### Status

Code-ready. The Nango-backed service boundary, local provider connection refs, and API routes are implemented. Manual Google/Nango credential smoke remains external setup, not a blocker for MCP readiness.

The stable product boundary is `ProviderConnectionService`, not Nango directly. This keeps source syncers insulated if we later replace Nango or add a local provider backend.

### Data Model

```sql
provider_connections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    backend TEXT NOT NULL DEFAULT 'nango',
    provider_config_key TEXT NOT NULL,
    external_connection_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    granted_scopes TEXT[] NOT NULL DEFAULT '{}',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    disconnected_at TIMESTAMPTZ
)
```

Indexes:

```sql
CREATE UNIQUE INDEX uq_provider_connections_active
ON provider_connections (user_id, backend, provider_config_key, external_connection_id)
WHERE status = 'active';
```

Provider streams gain:

```sql
owner_user_id UUID NULL,
provider_connection_id UUID NULL
```

Public streams keep both null. Private streams use both.

### Service Boundary

```go
type ProviderConnectionService interface {
    ListProviders(ctx context.Context) ([]ProviderDescriptor, error)
    StartConnect(ctx context.Context, input StartProviderConnectInput) (ProviderConnectSession, error)
    SyncConnections(ctx context.Context, input SyncProviderConnectionsInput) ([]ProviderConnection, error)
    ListConnections(ctx context.Context) ([]ProviderConnection, error)
    Disconnect(ctx context.Context, connectionID string) error
    GetConnectionCredentials(ctx context.Context, input ProviderCredentialRequest) (ProviderCredentials, error)
}
```

Supported providers use `NangoProviderConnectionBackend`. Unsupported providers can later use local backends under the same interface.

### API Shape

- `GET /api/provider-connections/providers`
- `POST /api/provider-connections/{provider}/connect`
- `POST /api/provider-connections/nango/webhook`
- `POST /api/provider-connections/sync`
- `GET /api/provider-connections`
- `POST /api/provider-connections/{connectionId}/disconnect`

The frontend starts Nango Connect through Aladin. Nango auth-creation webhooks are the primary persistence path for completed integrations; explicit sync remains as a manual repair/reconciliation fallback.

### Acceptance Criteria

- Nango starts locally through Docker. Implemented via local compose/Make setup.
- Aladin can create a Nango connect session for Google. Code-ready, pending real credentials for manual smoke.
- Aladin stores only provider connection refs, never raw provider tokens. Implemented.
- Sync is idempotent and user-scoped. Implemented and tested with fakes.
- Disconnect marks the local connection inactive and disables dependent owner-scoped streams. Implemented and tested.
- Threads is deferred because Nango does not support it today. Documented.

## Milestone 5 — MCP Readiness Hardening

### Goal

Finish the auth and documentation seams needed before adding the MCP server transport or tools.

### Status

Code-ready. No MCP binary, SDK dependency, or tools are introduced in this milestone.

### Work

- Add reusable bearer authorization resolution:
  - `ResolveBearerPrincipal(ctx, auth, authorizationHeader)`
  - accepts `Authorization: Bearer <integration token>`
  - rejects missing or malformed headers as unauthenticated
- Ensure integration token management remains browser-session-only.
- Ensure API-facing artifact/page/file routes map forbidden scope failures to `403`.
- Document the pre-MCP flow:
  - browser login
  - create integration token
  - use the one-time token as future MCP bearer credential
  - revoke when done

### Acceptance Criteria

- Bearer helper resolves valid integration tokens into scoped principals.
- Missing/malformed bearer headers fail safely.
- Read-only integration tokens can read artifacts/pages/files but cannot write.
- Integration tokens cannot create/list/revoke integration tokens.
- `go test ./...` passes.

## Milestone 6 — MCP Notes Server

### Goal

Add an MCP server that lets external agents author and read Aladin notes through existing artifact services.

### Binary

New binary:

```text
backend_v2/cmd/mcp
```

Responsibilities:

- load env/config
- configure JSON logging to `logs/mcp.log`
- connect Postgres
- run migrations
- build app dependencies
- start Streamable HTTP MCP transport
- graceful shutdown

### Package Placement

```text
backend_v2/internal/mcp/
    server.go
    tools.go
    auth.go
```

This is acceptable as an integration/transport package. Product behavior still stays in services.

### Transport

Use the official Go MCP SDK:

```text
github.com/modelcontextprotocol/go-sdk/mcp
```

Streamable HTTP endpoint:

```text
POST /mcp
```

Authentication:

```text
Authorization: Bearer <integration token>
```

The MCP auth middleware resolves the token to a principal and injects it into context.

### Tools

Use product language `note`, but store as page artifacts for v1.

Important: current artifact type semantics are `page`, `link`, `voice`, and `file`. Do not add a new `note` artifact type just for MCP.

Tools:

- `create_note(title, content, folder_id?, summary?, tags?, agent?)`
- `update_note(id, content?, title?, summary?, tags?)`
- `get_note(id)`
- `list_notes(folder_id?, limit?)`
- `search_notes(query, limit?)`

Defer `delete_note` unless it is clearly safe. Agent delete is a sharper capability than create/update.

### Tool Rules

- `create_note` calls `ArtifactService.Create` with `Type: "page"`.
- `update_note` first loads the artifact and rejects non-page artifacts.
- `get_note` rejects non-page artifacts.
- `list_notes` returns page artifacts only.
- `search_notes` searches page artifacts only.
- Tool handlers require `artifacts:read` or `artifacts:write`.

### Agent Metadata

Store agent metadata in artifact metadata:

```json
{
  "agent": {
    "id": "claude-code",
    "name": "Claude Code",
    "source": "mcp"
  }
}
```

No schema change for v1. Promote to a column only if indexing/filtering by agent becomes necessary.

### Acceptance Criteria

- MCP server starts independently from API.
- Invalid bearer token is rejected.
- Token missing `artifacts:write` cannot create/update notes.
- `create_note` produces a normal page artifact visible in the UI.
- Agent metadata is stored in `artifacts.metadata`.
- `get_note`/`update_note` reject link/voice/file artifacts.

## Milestone 7 — Private Source Sync

### Goal

Use the same auth and credential model for private/user-scoped sources.

Initial candidates:

- Gmail
- Threads keyword search

### Public vs Private Streams

Public provider streams:

- globally reusable
- no owner credential required
- examples: Bluesky public search, HN feed

Private/credentialed streams:

- owner-scoped
- use `provider_connections`
- never shared across users
- examples: Gmail under a user's Google connection; Threads later through a local backend or future Nango support

### Acceptance Criteria

- Private source items include `owner_user_id`.
- Private records/items never match other users.
- Provider rate limits are tracked against the owning connection where relevant.
- Disconnecting a connection stops future sync for dependent streams.

## Milestone 8 — Management UI

### Goal

Make auth/integration state visible and controllable from the app.

### UI Work

- Add logout in app shell.
- Add connected accounts screen.
- Add integration tokens screen:
  - create token
  - copy token once
  - list scopes
  - revoke token
- Add provider connect/disconnect affordances.

### Acceptance Criteria

- User can create an MCP token without editing `.env`.
- User can revoke an MCP token.
- User can see connected provider credentials.
- User can disconnect provider credentials safely.

## Recommended Build Order

1. Add `Principal` model and scope helpers. Done.
2. Move artifact/page/file ownership to current user context. Done for MCP readiness.
3. Add DB-backed integration tokens. Done.
4. Add Nango-backed provider connection refs. Done.
5. Harden bearer auth and scope enforcement for MCP readiness. Done.
6. Add MCP server with notes tools backed by integration tokens. Next.
7. Prove Google connect through Nango with real credentials.
8. Add private-source sync using provider connections.
9. Add management UI for tokens and connected accounts.

This order lets MCP ship before full OAuth while still using the same long-term auth architecture.

## Risks

- Moving artifact ownership may expose old default-user data migration questions.
- MCP SDK transport APIs may shift; pin the module version once implemented.
- Search over page content will be basic until full-text or embedding search is introduced.
- Scope enforcement can become noisy if implemented too granularly too early.
- Nango provider dependency adds operational surface area; keep it isolated behind `ProviderConnectionService` so Cloud, self-hosted, or a local backend remain swappable.

## Deferred Decisions

- Whether integration tokens should be user-created only or also system-created.
- Whether MCP should expose resources in addition to tools.
- Whether agent-authored notes need a dedicated UI marker.
- Whether delete should be exposed to MCP agents.
- Whether Threads should be implemented as a local backend or added to Nango when support exists.
- Whether browser sessions eventually use the same `Principal` struct internally.
