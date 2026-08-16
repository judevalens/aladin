# COMPOSE_ARCHITECTURE.md — Compose Multiplatform + Circuit (the `anchor` companion)

The frontend map for `anchor/`, the iPad companion. `design/UI_ARCHITECTURE.md` is the same
thing for the React desktop app; this is its counterpart for KMP/Compose.

Everything here was learned by getting it wrong first. Where a rule exists, the failure that
produced it is written next to it — a rule without its reason gets "simplified" away.

---

## 1 · The layering

```
domain/            pure rules. Imports NOTHING — no Compose, no SQLDelight, no Ktor.
services/          data · sync · design · network · platform. The outside world.
features/<screen>/ presenter + producers + composables for one screen.
```

`domain/` staying pure is the load-bearing one: it is why "where does Back go from a
subfolder" is a unit test rather than something you check by hand on a device.

---

## 2 · State producers

Circuit's **StateProducer** pattern: a class with a `@Composable` function producing one slice
of state, consumed by a parent presenter.

```kotlin
class NavigationStateProducer(private val nodes: NodeStore) {
    @Composable
    operator fun invoke(nav: SidebarNav, query: String): NavigationSlice { … }
}
```

**Services in the constructor, per-frame state as arguments.** That split is the whole shape:
the constructor takes what DI provides, `invoke` takes what changes each frame.

- **Slices are `data class`es** implementing `CircuitUiState`. Immutable snapshots; the
  producer owns the mutable pieces internally and hands one out per frame.
- **Registered in Koin**, so the presenter composes producers rather than services. The shell
  presenter takes no store, writer or cache at all.
- **Reaching outward is a callback**, not a read into someone else's state — creating a page
  selects it, deleting closes its tabs. Passing `onCreated` / `onDeleted` keeps the direction
  of the dependency visible at the call site.

### When NOT to make one

Do not manufacture a producer for something trivial. If it would be three lines of `remember`,
inline it. Circuit's guidance: keep a presenter unified when its state is tightly
interdependent or genuinely single-responsibility. In the shell, `nav` and `openItems` stayed
in the presenter *because more than one producer writes them* — hiding that behind another
indirection would have made it harder to see, not easier.

### UI stays dumb

Composables render state and emit events. A composable reaching for a store, a repository, or
branching on domain rules has taken logic it should not own. The other failure is the mirror
image: don't dodge a producer by pushing its logic into the view.

---

## 3 · Events, nested by owner

```kotlin
sealed interface Event {
    sealed interface Chrome : Event { … }
    sealed interface Write : Event { … }
    sealed interface Nav : Event { … }
}
```

Each producer handles **only its own** event type, so its `when` is exhaustive with no `else`.

> A flat hierarchy needs `else -> false` in every handler. That is a silent hole: a new event
> routed nowhere compiles and does nothing.

Commands that are not user actions (`dismissOverlays`, `unpinHero`) are lambdas on the slice,
not events — so `Event` keeps meaning "something the user did".

---

## 4 · Reactivity rules

**Local state holds identity. Everything displayable is derived from the store every frame.**

This one rule accounts for most of the bugs found in the shell: a title copied into an open
item, a node captured in a dialog, a path carrying its own labels. Each went stale the moment
the underlying row changed.

- Hold **ids**, resolve nodes. `OpenItem` is `(key, destination, nodeId)` — nothing else.
- The nav path is ids only; labels resolve through the store when drawn.
- The exception is genuinely local text — what the user is *typing* in a rename dialog.

**`NodeState` has three states, not two.**

```kotlin
sealed interface NodeState { Loading; Missing; Present(value) }
```

A nullable node conflates "not read yet" with "not there", so anything acting on absence
fires on every first frame. That shipped twice: a prune that reverted every drill the moment
it happened. Only `Missing` is evidence of deletion.

**Long-lived lambdas must not capture state.** `rememberPagerState { state.pages.size }` and
`LaunchedEffect(pagerState) { … state … }` both cache the first `state` forever. Read through
`rememberUpdatedState`.

---

## 5 · The store's read model

`NodeStore` hands out **retained per-key streams**, not lists to search:

```kotlin
fun node(id: String): StateFlow<NodeState>   // same id → same shared stream
fun children(parentId: String?): Flow<List<WorkspaceNode>>
fun byArtifactType(type: String): Flow<List<WorkspaceNode>>
```

`stateIn(scope, WhileSubscribed)` means opening a fourth item does not re-read the three
already open. Combine per-key flows for a set:

```kotlin
combine(ids.map(::node)) { it.mapNotNull(NodeState::node) }   // order preserved, no map to maintain
```

Scanning a materialised list is what makes copies *tempting*; an O(1) read removes the excuse.

Frames apply in **one transaction** — SQLDelight notifies listeners per table write, so a
2000-row snapshot otherwise re-runs every live query 2000 times.

---

## 6 · Platform views (interop)

- **A hoisted view must always be composed.** Visibility is a parameter, never a condition on
  composing it. `if (visible) NativeWebHost(…)` reads like an optimisation and is a teardown.
- **Never key a long-lived view on changing config.** Keying the web view on a bootstrap that
  embedded the bearer would have closed every editor on the first token refresh.
- **Compose content drawn after an interop view composites OVER it.** An opaque background on
  a sibling box painted a solid rectangle across the editor.
- **`hidden` vs `alpha`:** `hidden = true` makes WebKit and PDFKit discard their backing store,
  so returning costs a full repaint. Alpha-0 stays rendered.
- **Own the resource above the recycler.** `rememberWebHost` creates the `WKWebView`;
  `WebHostSurface` only attaches it, so a pager page can be disposed without destroying it.

---

## 7 · Testing

Producers are tested as real compositions with `circuit-test` (Molecule + Turbine):

```kotlin
presenterTestOf({ navigation(navInsideF2, query = "") }) {
    awaitUntil { it.title == "Greeks deep-dive" }
    nodes.rename("f2", "Greeks, revised")
    awaitUntil { it.title == "Greeks, revised" }
    cancelAndIgnoreRemainingEvents()
}
```

- `FakeNodeStore` gives per-key `MutableStateFlow`s, so a test can rename or delete a row and
  assert what the producer emits next.
- **Await a condition, not a frame count.** A composition emits intermediate frames; counting
  them tests the wrong thing.
- Unit tests need `isReturnDefaultValues = true` — Compose calls `android.util.Log` and
  `os.Trace`, which the unit-test `android.jar` stubs to *throw*.
- Producers are constructed directly with fakes. Constructor injection means a producer test
  needs no Koin container.

---

## 8 · Design tokens

Same rule as desktop: **never hardcode a colour or radius.** `services/design/` owns the
tokens (`AnchorTheme.colors`, `AnchorShape`, type styles) and maps role → glyph for icons.
Features ask for "the icon for this artifact kind", never for a specific glyph.

Icons come from a real dependency (Feather via `compose-icons`) — never hand-drawn on a
Canvas, never vendored from another package's build output.

---

## Reference

- Circuit presenter patterns: <https://slackhq.github.io/circuit/docs/presenter-patterns/>
- `~/.claude/plans/aladin-ipad-shell-architecture.md` — the layering decision
- `design/UI_ARCHITECTURE.md` — the desktop counterpart
