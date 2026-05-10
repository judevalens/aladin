# Nango Provider Connections

## Status

M2 provider-connection foundation.

Aladin uses Nango free self-hosted for OAuth/provider authorization and credential storage for supported providers. Aladin does not store raw provider tokens. It stores local `provider_connections` references and resolves usable credentials through `ProviderConnectionService`.

## Local Stack

Nango runs separately from the main Aladin Docker stack:

```sh
make nango-up
make nango-logs
make nango-down
```

The compose file is `docker-compose.nango.yml`. It intentionally uses separate Nango Postgres and Redis containers so Nango schema and runtime state do not couple to Aladin's own database.

`make backend` and `make worker-go` both run `nango-ensure` first. That target loads Nango env keys from `backend_v2/.env` and starts the Nango compose stack before starting the API or worker.

Required Nango env:

- `NANGO_ENCRYPTION_KEY`
- `NANGO_SECRET_KEY`

Required Aladin backend env:

- `NANGO_BASE_URL`, default `http://localhost:3003`
- `NANGO_CONNECT_BASE_URL`, default `http://localhost:3009`
- `NANGO_SECRET_KEY`
- `NANGO_GOOGLE_PROVIDER_CONFIG_KEY`

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
