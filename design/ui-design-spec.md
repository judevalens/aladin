# Aladin UI Design Spec

## Direction

Aladin should feel like a minimal editorial workspace with terminal-grade precision: calm, dense where it needs to be, and exact. The interface should use a white canvas, one restrained off-white surface family, charcoal ink, thin structural dividers, compact command controls, and sparse near-black active cues while avoiding default Material softness, colorful accents, and generic SaaS card treatment.

The current shell direction is a minimal three-pane workspace: a compact app rail, an expandable browser tree, and a broad document/workspace pane. The UI should feel like an operating surface for research artifacts, not a generic note app or Material dashboard.

## Visual System

- Color is charcoal, near-black, white, one restrained off-white, and neutral gray only.
- Product code should use `AladinColor` tokens, not Material-style surface roles.
- Core tokens are `Canvas`, `Panel`, `PanelMuted`, `RowHover`, `RowSelected`, `ControlHover`, `ControlPressed`, `Divider`, `Border`, `Ink`, `InkSecondary`, `InkMuted`, `InkDisabled`, `ActiveMarker`, `CommandSurface`, and `CodeText`.
- `Ink` is charcoal/near-black for primary text. `InkSurface` is reserved for selected rail items, brand surfaces, and compact high-contrast controls.
- Terminal precision tokens are `ActiveMarker`, `CommandSurface`, and `CodeText`; use them for row markers, command/search affordances, metadata, and small technical labels.
- Dark contrast tokens are `InkSurface`, `InkSurfaceHover`, and `OnInkSurface`; use them sparingly for active rail items and compact dark controls.
- Shell navigation stays light-theme-first, but selected navigation uses compact near-black contrast with light foreground.
- Content/browser selection should usually use soft gray selected rows, dark text, and a thin dark active marker. Avoid large or tall dark slabs in the browser.
- Secondary hover/pressed states use grayscale fills, never tinted color.
- Dividers are very thin and structural.
- Surfaces should ladder cleanly: white canvas, one restrained off-white panel family, then grayscale state tones. Avoid multiple creamy almost-whites.
- Radius should stay small: 4-6 dp for most controls and panels.
- Pane 2 may use a subtly muted off-white surface to distinguish browser/navigation from the main workspace without adding heavy contrast.

Current contrast defaults:

- `Ink` is charcoal/near-black for primary text.
- `InkSurface` is near-black for selected rail fills and brand surfaces.
- `InkSurfaceHover` is slightly darker than `InkSurface` for selected hover/press feedback.
- `OnInkSurface` is near-white, not pure white, for selected foreground text/icons.
- `CodeText` is used for compact metadata and command-like affordances.

## Typography

- Use Material typography as the implementation base for now, but style choices should feel editorial: heavier titles, restrained labels, and clear hierarchy.
- Metadata, section labels, command markers, and technical context should use a monospace style where available.
- Avoid decorative type treatment until a custom font is selected.
- Prefer contrast through weight and spacing over color.

## Components

- Navigation items use quiet soft selected states with dark icons and a thin active marker. Reserve near-black fills for compact command buttons or moments that need stronger commitment.
- Browser rows use the shared `aladinClickable` interaction layer.
- Selected browser rows use a soft selected surface plus a thin dark marker. Rail selection may remain dark because it anchors the shell.
- Dense browser rows should avoid boxed folder glyphs. Prefer hierarchy from indentation, chevrons, icon weight, and text contrast.
- Pane-level browser filters are deferred; do not add top filter controls until the filtering model is redesigned.
- Panels should feel like document/workspace surfaces, not cards.
- Buttons should look custom and monochrome, not Material-filled or ripple-driven.

## Shell Layout

- Keep the three-pane desktop shell: app rail, browser pane, workspace pane.
- The workspace pane is an artifact/work pane. It can contain an artifact tab rail, a context rail, and the active artifact surface.
- The app rail is icon-only and should read like a compact command rail, not a separate dark navigation product. It uses the muted pane surface, soft selected states, dark active icons, and a thin active marker.
- The brand mark should be quiet and utility-like: bordered or command-surface treatment, monospace `A`, and visually distinct from destination selection.
- The top toolbar should remain quiet and low-contrast, but it can breathe when there is no bottom divider: right-aligned command search plus separate lightweight creation actions.
- Search should read as a command input, not a generic form field. Avoid boxed grouped action clusters unless the actions become a real segmented control.
- Major pane dividers should be thin and structural, not decorative.
- Workspace content should feel like a document/research surface. Prefer section dividers and editorial spacing over large empty bordered cards when content is sparse.
- Page save/load/upload status belongs in the artifact context rail, after breadcrumb/path text and before utility icons. Do not add a third status bar inside the page body.
- Idle page status should be subtle metadata, such as `Saved` plus a formatted update time. Active or failed states may replace that text in-place and show compact retry affordances.

## Browser Tree

- The persisted model is recursive through `Item.parentId`, but the UI renders a flat projected tree in a `LazyColumn`.
- The model supports unbounded nesting, but pane 2 should not indent forever or use horizontal scrolling as the primary deep-tree behavior.
- Use scoped drill-in navigation for deep nesting: folders at visual levels 0 and 1 expand inline; folders at visual level 2 open as the new browser scope.
- Pane 2 uses a local scope breadcrumb, not the selected object breadcrumb. It identifies the current browser scope and uses a back chevron to move one scope up.
- The full selected-object breadcrumb belongs in the workspace pane above the open folder/artifact title. It provides absolute orientation and jump navigation for the selected object.
- Use `depth` for visual indentation inside the current scope instead of recursively nesting composables. This preserves virtualization and keeps deep trees predictable.
- Folder-like rows use chevrons plus bare icons. Do not wrap folder icons in boxed glyph backgrounds.
- Artifact rows should stay compact: title plus one muted metadata line. Long summaries belong in the workspace pane, not the browser row.
- Folder rows behave as navigation/expansion controls and should not render selected-fill styling. Artifact rows use soft gray fill, dark foreground, and a thin active marker when they match the active workspace artifact.
- Workspace breadcrumbs remain visible as selected-object path context and jump controls. They should not replace browser expansion state, and they should not use selected-fill styling.
- Browser scope breadcrumbs show the current local browser scope; the browser itself shows that scope and its local descendants.
- Browser filters have been removed from the top of pane 2. Reintroduce filters only after the filtering model is redesigned.
- Browser rows support rename through a compact command-sheet overlay. Double-clicking a folder/artifact row or choosing Rename from the context menu should open the sheet.
- Rename commits on Enter or the explicit Save action. Escape or Cancel closes without saving. Empty or unchanged names should cancel locally.
- Rename overlays should use the app overlay viewport so they render above embedded JS editor surfaces.
- Rename UI should feel like a precise command surface: compact, monochrome, bordered, and token-based. Avoid generic Material dialog or form styling.

## Artifact Workspace

- The artifact/work pane should coordinate open artifacts through a compact tab rail.
- The context rail provides selected-object orientation through breadcrumbs/path text, page metadata, and utility actions.
- The active artifact surface should be type-specific: page editor, link viewer, voice/file viewer, graph surface, or future specialist surface.
- Pages should use the broad document/editor canvas. Compact source-like artifacts such as links, voice notes, and files should render as centered artifact objects when the inspector is closed.
- The artifact inspector is the home for system context: AI summary, key points, related items, entities, and source metadata. Opening it should create a two-column workspace and push compact artifact cards left.
- Link artifacts should read as source cards, not documents: title, domain/source URL, open-original action, summary, raw capture, and user notes belong in the primary card.
- Voice artifacts should read as recording cards: playback, transcript, recording metadata, and user notes belong in the primary card while summaries, key moments, and related context belong in the inspector.
- Voice capture should be modal-first: start recording quickly, show a live confidence waveform, then collect title and description in the review state before saving.
- Page editor chrome should stay minimal. The editor body should be the document surface, not a stack of toolbars.
- Save, upload, and load feedback should feel like document metadata rather than an alert system. Errors should be visible and retryable without adding modal friction.
- JS specialist surfaces should visually integrate with the Kotlin shell. Avoid duplicate chrome inside the embedded surface when the shell already provides tabs, breadcrumbs, status, or utility actions.

## Sources Workspace

- Sources should read as a calm source library for the workspace, not a dashboard or CRUD settings table.
- Prefer language like `live inputs`, `sources`, `feeds`, `health`, and `last refresh` over backend sync jargon.
- The primary Sources surface should have two layers:
  1. an overview band that explains what the area does and summarizes the source set
  2. a scan-friendly card list where each source can be understood without a second pane
- The Sources area should stay visually light. Prefer spacing, alignment, and thin dividers over stacking bordered containers inside bordered containers.
- Do not use a permanent split inspector with its own independent scroll region for this surface.
- Source rows/cards should feel editorial and self-contained: provider mark, clear title, one-line source description, compact status pills, and a quiet open-detail affordance.
- Opening a source should use a modal detail sheet that keeps focus on one object without forcing side-by-side comparison.
- Source detail should stay user-meaningful: what the source tracks, whether it looks healthy, last refresh, and recent activity. Avoid exposing internal-only mechanics like thresholds or backend policy fields.
- Empty Sources states should explain why streams matter to the workspace, not just that the list is empty.
- Add-stream flows should read like provider setup, not a generic modal form. The key user action is defining the upstream stream identity; cadence and matching remain backend-owned.

## Interaction

- Hover state: light gray.
- Pressed state: slightly stronger gray unless the component is already selected.
- Selected state: soft gray surface plus dark active marker for browser rows and rail items. Near-black surfaces are reserved for compact primary/command controls.
- Disabled state: muted text on light gray.
- Avoid Material ripples unless intentionally reintroduced for a specific control.

## Avoid

- Purple/blue accents.
- Creamy beige system colors or multiple near-identical almost-whites.
- Large rounded Material cards.
- Floating shadows as a default depth model.
- Per-component one-off hover colors.
