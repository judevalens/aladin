# Auth System

## Status

Thin email/password auth foundation implemented.

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

## API Routes

Public auth routes:

- `POST /api/auth/register`
- `POST /api/auth/login`
- `POST /api/auth/logout`

Protected auth route:

- `GET /api/auth/me`

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

Application services should use `CurrentUserFromContext` instead of hardcoded user ids when behavior is user-owned.

## Current User Usage

The source service is now user-scoped:

- `SourceService.List`
- `SourceService.Create`
- `SourceService.Delete`

These read the current user from context and pass `user.ID` to the repository. This is the first user-owned slice because private-source/OAuth work needs correct ownership.

Some older workspace services still use the default dev user in wiring. That is intentional for this pass. They should be moved slice-by-slice when the ownership semantics are clear.

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

For OAuth connect later:

- OAuth state must be bound to the currently authenticated user/session.
- Callback must reject if the current user differs from the state owner.
- `returnTo` must be path-only or allowlisted to avoid open redirects.
- Provider credentials should reference `user_id`.
- Refresh tokens should be nullable because not every provider always returns one.
- Credential rows should store `key_version` for future encryption rotation.
- Existing owner-scoped streams should be paused or disabled on disconnect.

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
- Add OAuth state and credential storage model.
- Add session cleanup for expired/revoked sessions.
- Replace the Wasm fetch shim if Ktor exposes credential configuration in the target.
