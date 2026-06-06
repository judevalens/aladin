# Connected Accounts UI Plan

## Summary

Connected Accounts is the setup surface for private provider credentials. It is launched from Sources because provider connections enable future private streams, but it is not a permanent section inside the Sources pane. Nango stores supported provider credentials; Aladin stores only local connection references.

## Current Scope

- Google is the only connectable provider for the first smoke test.
- Popular providers are visible in the catalog as disabled/coming later so the UI and backend shape are already extensible.
- Provider OAuth scopes are configured in Nango, not selected in Aladin.
- Gmail ingestion, Threads, and MCP server work are separate milestones.

## Backend Contract

- `GET /api/provider-connections/providers` returns UI-safe provider descriptors: provider id, label, backend, config key, description, category, capabilities, availability, connection state, and coming-soon status.
- `POST /api/provider-connections/{provider}/connect` starts a provider connect session for available providers only.
- `POST /api/provider-connections/nango/webhook` receives verified Nango auth creation webhooks and persists local connection refs.
- `POST /api/provider-connections/sync` reconciles Nango connections into Aladin connection refs as a manual fallback.
- `GET /api/provider-connections` lists active local refs.
- `POST /api/provider-connections/{connectionId}/disconnect` disconnects the local ref and calls the provider backend where supported.

## UI Behavior

- Sources exposes a standard `Integrations` action near the Add Stream action.
- The `Integrations` action opens a wide app-level management modal, not a small test dialog and not an embedded panel.
- The modal uses the high-contrast management pattern from `design/DESIGN_SPEC.md`: fixed title band, provider card grid on the left, selected provider detail/actions on the right.
- Google renders as the first connectable provider card when the provider catalog is available.
- Disabled/future providers render as muted cards so unavailable state is visible before reading status text.
- Selected provider cards use a strong near-black selected surface with light foreground.
- The detail pane does not repeat the card status badge. It leads with provider identity, primary state explanation, capabilities/scopes, connection model, and actions.
- Capability/scopes are plain tags under an explicit section label; they should not look like clickable buttons.
- The connect flow starts Nango Connect, opens the returned link, and lets the user check/sync the connection after returning.
- Disconnect is available for connected providers.
- The provider grid should have a single scroll container. Avoid wrapping lazy grids in another vertical scroll container.

## Verification

- Backend tests cover descriptor metadata, unavailable provider rejection, start-connect tags, sync idempotency, and disconnect behavior.
- KMP compile verifies the Sources UI and API models.
- Manual smoke: start Nango, configure Google provider key, log into Aladin, open Sources, connect Google, return, and check connection.
