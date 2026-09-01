# Architecture Refactor Planner shard

This is the source package for the interactive companion to
[the architecture consolidation program](../../../docs/architecture/ARCHITECTURE_CONSOLIDATION_PROGRAM.md).

The document is the canonical rationale, sequence, and policy. The shard is the
execution surface: it shows phases and work items, persists status and planning
notes through `useShardState`, and makes the program usable as an operating board.

The source targets the authoring capabilities currently returned by the production
Aladin `get_authoring_guide`: React/TypeScript, `@aladin/kit`, `anchors.json`, and
the bridge/1 shard-local state API. It intentionally does not opt into the
unreleased Shard v2 resource contract.

Before importing this package into Aladin:

1. Create the canonical page from the program document.
2. Create an app titled `Architecture Refactor Planner`.
3. Call `get_authoring_guide(page_id)` for that new app.
4. Write `anchors.json` and `index.tsx`, then build and verify every declared anchor.
5. Add the page artifact ID as a manifest ref only when the destination confirms
   that workspace-node access is enabled and the ref resolves.
6. Publish only after explicit approval.
