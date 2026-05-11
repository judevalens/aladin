# Auth System

## Status

Email/password auth and the MCP-ready integration-token foundation are implemented.

Aladin currently uses backend-owned opaque sessions stored in Postgres and sent to the browser as an `HttpOnly` cookie. This is intentionally simple: it establishes a real user boundary before OAuth/private-source work without committing the app to JWTs or a full identity provider.

## Goals

- Give backend requests a trusted current user.
- Bind source subscriptions and future provider credentials to that user.
- Keep tokens out of JavaScript-readable storage.
- Preserve local-dev simplicity.
- Leave room for OAuth providers such as Google, Threads, and future private-source integrations.
- Resolve every auth method into one future `Principal` model.

## Non-Goals

- No JWT access/refresh-token system.
- No OAuth provider integration yet.
- No roles, organizations, SSO, or multi-tenant admin model yet.
- No password reset or email verification yet.

## JWT Stance

No JWT for now.

Aladin currently has one backend validating requests. Opaque DB-backed sessions and integration tokens are a better fit because they support immediate revocation, audit, scope changes, and simple local development.

The intended auth spine is:

```text
Browser session cookie -> DB session -> Principal
MCP/API bearer token -> DB integration token -> Principal
OAuth provider token -> stored credential, not app auth
```

JWT can be reconsidered later for mobile or service-to-service flows if stateless validation becomes a real requirement.

## Backend Model

### `users`

Migration `00020_email_password_auth.sql` extends `users` with:

- `password_hash`
- `updated_at`
- `last_login_at`

Emails remain unique. Passwords are never stored directly.

### `user_sessions`

`user_sessions` stores browser sessions:

- `id`
- `user_id`
- `token_hash`
- `expires_at`
- `revoked_at`
- `user_agent`
- `created_at`
- `last_seen_at`

The browser receives only the raw opaque session token. The database stores a SHA-256 hash of that token.

### `integration_tokens`

Migration `00023_integration_tokens.sql` adds DB-backed bearer tokens for MCP and future personal API access:

- `user_id`
- `name`
- `token_hash`
- `scopes`
- `status`
- `expires_at`
- `revoked_at`
- `created_at`
- `updated_at`
- `last_used_at`

The raw token is shown only once at creation. The database stores only `sha256(token)`.

## Passwords

Password hashing lives in `backend_v2/internal/service/password.go`.

Current scheme:

- PBKDF2-SHA256
- 210,000 iterations
- 16-byte random salt
- 32-byte derived key
- encoded format: `pbkdf2_sha256$iterations$salt$key`

This is stdlib-only and acceptable for v1. If auth becomes serious user-facing infrastructure, Argon2id via `x/crypto` would be a reasonable later upgrade.

## Sessions

Session creation happens after successful register/login:

1. Generate a random 32-byte token.
2. Store `sha256(token)` in `user_sessions`.
3. Return the token as cookie `aladin_session`.

Cookie settings:

- `HttpOnly`
- `SameSite=Lax`
- `Path=/`
- `Secure` only when the request is HTTPS or behind `X-Forwarded-Proto: https`
- 30-day expiration

Logout revokes the current session by setting `revoked_at`.

## Integration Tokens

Integration tokens are intended for MCP servers and other external actors that should act for a user without receiving a browser session.

Management routes require a normal browser session:

- `GET /api/integration-tokens`
- `POST /api/integration-tokens`
- `POST /api/integration-tokens/{id}/revoke`

Token creation accepts:

```json
{
  "name": "Claude Code",
  "scopes": ["artifacts:read", "artifacts:write"],
  "expiresAt": "2026-06-01T00:00:00Z"
}
```

`expiresAt` is optional. Returned raw tokens use the `aladin_it_` prefix and are not recoverable later.

Supported scopes:

- `artifacts:read`
- `artifacts:write`
- `sources:read`
- `sources:write`
- `insights:read`
- `*`

`AuthService.ResolveBearerToken(ctx, rawToken)` resolves a valid token into:

```text
Principal{UserID, ActorType: "integration_token", ActorID, Email, Scopes}
```

This is the method the MCP auth middleware should call.

`ResolveBearerPrincipal(ctx, auth, authorizationHeader)` is the reusable transport helper for MCP and future bearer-auth entry points. It parses `Authorization: Bearer <token>`, resolves the integration token, and returns a scoped `Principal`.

Pre-MCP local smoke flow:

1. Log in through the app or `POST /api/auth/login` so the browser/client has `aladin_session`.
2. Create a token with `POST /api/integration-tokens` and scopes `["artifacts:read", "artifacts:write"]`.
3. Use the one-time `token` value as the future MCP bearer credential.
4. Revoke with `POST /api/integration-tokens/{id}/revoke` when done.

## API Routes

Public auth routes:

- `POST /api/auth/register`
- `POST /api/auth/login`
- `POST /api/auth/logout`

Protected auth route:

- `GET /api/auth/me`

Protected integration-token routes:

- `GET /api/integration-tokens`
- `POST /api/integration-tokens`
- `POST /api/integration-tokens/{id}/revoke`

Health/dev routes remain public:

- `/api/health`
- `/healthz`
- `/readyz`
- `/api/quote`

All other API routes pass through auth middleware.

## Middleware Flow

The server wraps routes with:

```text
cors(traceRequests(authMiddleware(mux)))
```

For each request:

1. Read `aladin_session`.
2. Hash token and load an active, unexpired session.
3. If valid, inject `service.CurrentUser` into request context.
4. If invalid or missing and route is public, continue unauthenticated.
5. Otherwise return `401 Unauthenticated`.

The context helpers live in `backend_v2/internal/service/auth.go`:

- `WithCurrentUser(ctx, user)`
- `CurrentUserFromContext(ctx)`
- `WithPrincipal(ctx, principal)`
- `PrincipalFromContext(ctx)`
- `RequirePrincipal(ctx)`
- `RequireScope(ctx, scope)`
- `ResolveBearerPrincipal(ctx, auth, authorizationHeader)`

Application services should use `RequirePrincipal` or `RequireScope` instead of hardcoded user ids when behavior is user-owned.

Browser-session principals are treated as full app access for now. Integration-token principals must carry the required scope. Artifact, page, and file operations now enforce `artifacts:read` or `artifacts:write`, which is the scope boundary MCP note tools will rely on.

## Current User Usage

The source service is user-scoped:

- `SourceService.List`
- `SourceService.Create`
- `SourceService.Delete`

These read the current user from context and pass `user.ID` to the repository. Workspace artifact/page/file repositories also resolve user ownership from `Principal` in context.

`defaultUserID` remains only as a dev/system fallback for current realtime key resolution and seeded local data. It is not the request ownership source for artifact/page/file operations.

## Client Flow

The Compose app has a root auth gate in `CircuitApp`.

Startup flow:

1. Call `GET /api/auth/me`.
2. If successful, enter the normal workspace.
3. If unauthenticated, show login/register UI.
4. Login/register stores the server cookie automatically and enters the workspace.

Auth UI lives in:

- `aladin_ui/composeApp/src/wasmJsMain/kotlin/com/jvp/aladin_compose/features/auth/AuthUi.kt`

API models/methods live in:

- `api/Models.kt`
- `api/ApiClient.kt`

## Browser Credentials

Because the backend runs on `localhost:8000` and the Wasm dev server runs on a different origin, API requests must include credentials.

Ktor 3.1's Wasm JS client does not expose `configureRequest` in the currently compiled target, so `HttpClientFactory.kt` installs a small fetch shim:

```text
if request credentials are undefined, set credentials = "include"
```

This keeps cookies flowing for same-machine local development. If Ktor is upgraded and the Wasm target exposes request configuration directly, replace the shim with engine-level `credentials = include`.

## CORS

The backend reflects the request `Origin` and sets:

- `Access-Control-Allow-Credentials: true`
- `Vary: Origin`
- allowed methods: `GET, POST, PATCH, DELETE, OPTIONS`
- allowed headers: `Content-Type, Authorization`

Credentialed CORS cannot use `Access-Control-Allow-Origin: *`, so origin reflection is required for browser cookie auth.

## Error Semantics

- Invalid login returns `401`.
- Missing/invalid session on protected routes returns `401`.
- Invalid register input returns `400`.
- Duplicate email currently returns `400`.

The client still uses throwing API calls. A future `Result<T>` boundary can be introduced once repository/UI failure semantics settle.

## OAuth Implications

This auth layer is the trusted user binding that OAuth needs.

For provider connections:

- Aladin uses Nango Cloud as the default credential backend for supported providers.
- Provider tokens live in Nango; Aladin stores only `provider_connections` refs tied to `user_id`.
- Product code depends on `ProviderConnectionService`, not Nango directly.
- Nango is used for Auth/credential/proxy responsibilities only; Aladin keeps ingestion, scheduling, records, matching, and insights.
- Public provider streams stay global; private streams later reference `owner_user_id` and `provider_connection_id`.
- Disconnecting a provider connection marks the local ref inactive and disables dependent owner-scoped streams.
- Threads is deferred because Nango does not support it today; it can later use a local backend behind the same interface.

Do not let OAuth callbacks attach credentials to arbitrary user ids from request payloads.

## Verification

Current verification commands:

```sh
cd backend_v2
GOCACHE=/Users/judepaulemon/Documents/aladin/backend_v2/.gocache go test ./...

cd aladin_ui
./gradlew :composeApp:compileKotlinWasmJs
```

The explicit `GOCACHE` keeps Go test artifacts inside the workspace when macOS cache permissions block sandboxed runs.

## Open Work

- Add logout affordance in the app shell.
- Move artifact/page/file ownership off the default dev user.
- Add password reset and email verification if this becomes external-user facing.
- Add the MCP server transport and note tools on top of `ResolveBearerToken`.
- Add session cleanup for expired/revoked sessions.
- Replace the Wasm fetch shim if Ktor exposes credential configuration in the target.
