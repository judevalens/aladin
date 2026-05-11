# Connected Accounts UI Plan

## Summary

Connected Accounts is the setup surface for private provider credentials. It lives in Sources because provider connections enable future private streams, but it is separate from stream subscriptions. Nango stores supported provider credentials; Aladin stores only local connection references.

## Current Scope

- Google is the only connectable provider for the first smoke test.
- Popular providers are visible in the catalog as disabled/coming later so the UI and backend shape are already extensible.
- Provider OAuth scopes are configured in Nango, not selected in Aladin.
- Gmail ingestion, Threads, and MCP server work are separate milestones.

## Backend Contract

- `GET /api/provider-connections/providers` returns UI-safe provider descriptors: provider id, label, backend, config key, description, category, capabilities, availability, connection state, and coming-soon status.
- `POST /api/provider-connections/{provider}/connect` starts a provider connect session for available providers only.
- `POST /api/provider-connections/sync` reconciles Nango connections into Aladin connection refs after the user completes Nango Connect.
- `GET /api/provider-connections` lists active local refs.
- `POST /api/provider-connections/{connectionId}/disconnect` disconnects the local ref and calls the provider backend where supported.

## UI Behavior

- Sources shows a `Connected accounts` section above live streams.
- Google renders as the primary account row when the provider catalog is available.
- Disabled providers render as compact coming-later pills.
- The connect flow opens an app-level centered modal, starts Nango Connect, opens the returned link, and lets the user check/sync the connection after returning.
- Disconnect is available for connected providers.

## Verification

- Backend tests cover descriptor metadata, unavailable provider rejection, start-connect tags, sync idempotency, and disconnect behavior.
- KMP compile verifies the Sources UI and API models.
- Manual smoke: start Nango, configure Google provider key, log into Aladin, open Sources, connect Google, return, and check connection.
