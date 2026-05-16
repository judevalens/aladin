# Realtime Repository Wiring

## Summary

Complete realtime by wiring backend workspace events into client repositories. Repositories remain the source of cached app data and emit flows; producers continue consuming repository flows and should not subscribe to WebSocket events directly.

The first slice focuses on workspace metadata correctness: browser tree, artifact metadata, open tabs, and breadcrumbs. Async ingestion and pipeline realtime stay separate because worker events need a cross-process bridge.

## Key Changes

### Client Realtime Boot

- Start one app-level `WebSocketAppEventSource` on app boot with the broad workspace subscription:
  - `stream = "workspace"`
  - `resourceKind = "*"`
  - `resourceId = "*"`
- Start one `AppEventProcessor` with repository listeners.
- Stop the source/processor when the app root leaves composition.
- Keep event dedupe in `AppEventProcessor`; repositories should be idempotent but do not need their own event-id cache in v1.

### Flow-Backed Repositories

- Keep `ArtifactRepository` as the metadata/page repository, but make it consume realtime events:
  - On `artifact.created` / `artifact.updated`, refetch the artifact and emit it through the existing artifact flow.
  - On `artifact.deleted`, remove or mark the artifact unavailable and emit enough state for open tabs/browser tree to refresh.
  - On `page.updated`, refetch artifact metadata only if needed; do not overwrite editor draft content from realtime events.
- Change `FolderRepository` from request-only to flow-backed:
  - Add `observeBrowserTree(): Flow<List<BrowserTreeNode>>`.
  - Add `refreshBrowserTree()` for explicit reloads.
  - On folder/artifact workspace events, refresh the browser tree and emit the new tree.
- `DocumentBrowserProducer` should collect `observeBrowserTree()` instead of owning the canonical tree loaded once from `browserTree()`.

### Producer Behavior

- `WorkPaneProducer` keeps collecting:
  - `artifactRepository.artifact(id)`
  - `artifactRepository.artifacts(openArtifactIds)`
  - `artifactRepository.observeArtifactBreadcrumbs(activeArtifactId)`
- `DocumentBrowserProducer` keeps local UI-only state:
  - expanded folders
  - current drill scope
  - active rename dialog
  - voice capture modal
- Browser rows, tab titles, and breadcrumbs update through repository emissions, not metadata revision counters or manual recomposition stitches.

### Backend Event Scope

- Keep existing API-process realtime publishes:
  - artifact create/update/delete
  - folder create/update
  - page save
- Add missing publish calls only if the current workspace metadata flow needs them.
- Do not wire pipeline/worker events in this pass because the current in-memory broker only lives inside the API process.

### Deferred Worker Realtime Bridge

- Add a later bridge for async events:
  - DB outbox or Redis pub/sub from worker to API process.
  - API process fans out bridge events through the existing WebSocket broker.
- Future event types:
  - `record.enriched`
  - `tenant_match.created`
  - `insight.created`
  - `provider_stream.updated`

## Public Interfaces / Types

- `FolderRepository` gains:
  - `fun observeBrowserTree(): Flow<List<BrowserTreeNode>>`
  - `suspend fun refreshBrowserTree()`
- `ArtifactRepositoryImpl` and `FolderRepositoryImpl` implement `AppEventListener`.
- App bootstrap owns:
  - `WebSocketAppEventSource`
  - `AppEventProcessor`
  - registered repository listeners
- No backend API response shape changes.
- No database migrations.

## Test Plan

- Client repository tests:
  - `artifact.updated` causes artifact refetch and emits updated metadata.
  - `folder.updated` causes browser tree refresh and emits updated rows.
  - `artifact.created` / `artifact.deleted` refresh browser tree.
  - `page.updated` does not reset live editor draft content.
- Client processor tests:
  - duplicate `eventId`s are ignored.
  - only matching listeners handle an event.
- UI/manual checks:
  - Rename artifact updates browser row and open tab title without manual navigation.
  - Rename folder updates browser tree.
  - Create artifact/folder updates browser tree.
  - Page editing still does not reset while typing.
- Backend tests:
  - artifact/folder/page mutation paths publish expected workspace event type and resource key.
- Compile checks:
  - `cd aladin_react && npm run build`
  - `go test ./...` only if backend publish code changes.

## Assumptions

- Repositories own cache and local data flow; producers remain UI-state owners only.
- Whole-tree refresh on folder/artifact events is acceptable for v1; local tree patching is deferred.
- Realtime is workspace metadata first, not ingestion feed realtime.
- The current in-memory broker is acceptable for API-process mutations but not sufficient for worker-originated events.
- Async pipeline realtime requires a separate cross-process bridge and should not be mixed into this first implementation slice.
