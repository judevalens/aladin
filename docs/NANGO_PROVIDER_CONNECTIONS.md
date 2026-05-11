# Nango Provider Connections

## Status

M2 provider-connection foundation.

Aladin uses Nango Cloud by default for OAuth/provider authorization and credential storage for supported providers. Aladin does not store raw provider tokens. It stores local `provider_connections` references and resolves usable credentials through `ProviderConnectionService`.

Google is the only enabled provider for the first smoke test. The backend provider catalog also exposes popular future providers as disabled/coming later so the UI can prove the account-management shape without pretending those providers are ready.

## Default Cloud Setup

Required Aladin backend env:

- `NANGO_BASE_URL`, default `https://api.nango.dev`
- `NANGO_CONNECT_BASE_URL`, default `https://connect.nango.dev`
- `NANGO_SECRET_KEY`
- `NANGO_GOOGLE_PROVIDER_CONFIG_KEY`

`NANGO_SECRET_KEY` is the Nango environment secret key from Nango Cloud. `NANGO_GOOGLE_PROVIDER_CONFIG_KEY` is the integration key configured in Nango, for example `google` or `gmail`.

Google OAuth callback URLs should use the exact callback URL shown by Nango Cloud for that integration. Do not use the self-hosted localhost callback for cloud integrations.

Print shell-safe exports from `backend_v2/.env`:

```sh
make env-nango
eval "$(make env-nango)"
```

## Connect Flow

1. User is authenticated in Aladin through the normal HttpOnly session cookie.
2. Frontend calls `POST /api/provider-connections/google/connect`.
3. Aladin creates a Nango connect session with tags:
   - `end_user_id`
   - `end_user_email`
   - `aladin_user_id`
4. Frontend opens Nango Connect UI with the returned session token/link.
5. After Connect succeeds or closes, frontend calls `POST /api/provider-connections/sync`.
6. Aladin lists Nango connections tagged with the current user id and upserts local `provider_connections` refs.

M2 uses explicit sync/reconciliation instead of Nango webhooks.

## Optional Self-Hosted Fallback

The repo still includes `docker-compose.nango.yml` for local self-hosted experiments:

```sh
make nango-up
make nango-logs
make nango-down
```

Self-hosted Nango is not the default path. If it is used, override:

```env
NANGO_BASE_URL=http://localhost:3003
NANGO_CONNECT_BASE_URL=http://localhost:3009
NANGO_ENCRYPTION_KEY=<base64-encoded 32-byte key>
NANGO_SECRET_KEY=<uuid-v4 secret key>
```

Nango self-hosted uses its own Postgres and Redis containers. Keep them separate from Aladin's database and Redis.

## Boundaries

Aladin owns:

- users and sessions
- provider stream and source subscription records
- ingestion scheduler and worker pipeline
- records, matching, insights, and notifications

Nango owns:

- provider OAuth authorization
- credential storage
- token refresh
- credential/proxy access for supported providers

Source syncers must depend on `ProviderConnectionService`, not Nango directly. If Nango later becomes too heavy or a provider is unsupported, a local backend can be added behind the same service interface.

## Threads

Threads is not part of M2 because Nango does not currently support it. Later options:

- implement Threads as a local provider backend behind `ProviderConnectionService`
- migrate Threads to Nango if Nango adds support or a custom provider becomes practical

Do not wire Threads through the Facebook Nango integration unless Nango explicitly supports Threads endpoints and scopes there.
