# Page Document Sync

## Purpose

Page document sync is the boundary between the markdown editor, Kotlin client state, and backend persistence.

The goal is trustworthy single-user editing now, with a clean path toward offline sync and CRDT-based collaboration later.

## Ownership Boundaries

- **Editor / JS surface** owns the live draft text after initial mount.
- **Kotlin syncer** owns load status, save status, revision tracking, and retry metadata.
- **Kotlin producer/UI** interprets sync state for display and routes editor events.
- **Backend** owns durable page content, revision validation, and stale-write rejection.
- **React/TypeScript editor integration** should stay thin: capture editor events, forward file/upload/document requests, and render editor behavior.

## Current Contract

Page load returns:

```json
{
  "id": "artifact-id",
  "title": "Page title",
  "content": "# Markdown",
  "revision": 42,
  "updatedAt": "2026-05-04T12:00:00Z"
}
```

Page save sends:

```json
{
  "content": "# Updated markdown",
  "revision": 43
}
```

The backend accepts a save only when the incoming revision is greater than the stored revision. Stale saves return `409 Conflict` and must not overwrite newer content.

## Sync Flow

1. Opening a page asks the syncer to load it.
2. The backend returns markdown plus current revision.
3. The editor mounts with the initial markdown snapshot.
4. Editor changes emit document update events over the bridge.
5. The syncer increments the loaded/latest revision and sends a save request.
6. Successful save updates sync metadata such as saved revision and `updatedAt`.
7. Stale save responses are ignored or surfaced as non-destructive conflicts; they never reinitialize editor content.

## Design Rules

- Do not feed saved markdown back into the live editor after local editing starts.
- Do not let producers reconstruct document sync semantics from UI state.
- Do not put backend upload/save logic inside the React editor.
- Keep page content separate from the artifact envelope.
- Treat revision as a stale-write guard, not as full collaboration.

## Future CRDT Path

The current `content + revision` model can evolve into:

- document snapshots
- operation logs
- CRDT clocks
- snapshot compaction
- offline mutation queues

The desired migration path is to preserve the current boundaries while replacing the persistence payload.
