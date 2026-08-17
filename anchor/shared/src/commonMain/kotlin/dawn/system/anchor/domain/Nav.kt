package dawn.system.anchor.domain

/**
 * What the tree can say about one row.
 *
 * Three answers, because two are not enough: a nullable node conflates **not read yet** with
 * **not there**, and anything that acts on absence then acts on every first frame. That
 * mistake shipped twice as a prune that reverted every drill the moment it happened.
 *
 * [Unknown] is the answer for a row nobody has read — and it may move nothing. Only [Gone] is
 * evidence of deletion.
 */
enum class Presence { Unknown, Gone, There }

/**
 * The five views a research folder presents. Synthetic tabs, not artifacts — there is no
 * server object behind them (see `backend_v2/internal/api/research.go`, which exposes only
 * create/get/patch), so they exist purely as an addressing scheme.
 *
 * Mirrors `aladin_react/src/modules/workspace/domain.ts:43`.
 */
enum class ResearchView(val wire: String) {
    Overview("overview"),
    Manifest("manifest"),
    Runs("runs"),
    Code("code"),
    Inspect("inspect"),
    ;

    companion object {
        fun fromWire(value: String): ResearchView? = entries.firstOrNull { it.wire == value }
    }
}

/**
 * What identifies an open thing.
 *
 * **A sealed type rather than the desktop's bare string.** `tabKey()` there returns the raw
 * `artifactId` for an artifact and `research:<contextId>:<view>` for a research view
 * (`domain.ts:50-52`), which means nothing stops an artifact id beginning with `research:`
 * from colliding with a research key. [asString] still reproduces that exact form, because
 * the browser row ids are built with the identical shape (`domain.ts:200`) and the two must
 * agree.
 *
 * Note that the same research folder at two different views is **two keys** — that is the
 * point of the view being part of the identity.
 */
sealed interface TabKey {

    /**
     * The tree row behind this — an artifact for one, the research folder for the other.
     * What the store is asked about, and therefore what "gone" is decided on.
     */
    val nodeId: String

    /** The string form the desktop uses. Load-bearing; do not "tidy". */
    fun asString(): String

    data class Artifact(val artifactId: String) : TabKey {
        override val nodeId: String get() = artifactId
        override fun asString(): String = artifactId
    }

    data class Research(val contextId: String, val view: ResearchView) : TabKey {
        override val nodeId: String get() = contextId
        override fun asString(): String = "$RESEARCH_PREFIX$contextId:${view.wire}"
    }

    companion object {
        const val RESEARCH_PREFIX: String = "research:"
    }
}

/**
 * What the content pane is showing.
 *
 * **A destination and a document share one slot.** That is the whole trick, and it is the
 * previous shell's disease inverted: there, "which section am I in" and "which artifact is
 * active" were separate values that had to agree, and a repair effect existed solely to
 * reconcile them. Here there is nothing to reconcile — there is one answer.
 *
 * Mirrors the prototype's `sel`, which is a single string disambiguated by membership in
 * `DESTS` (`Aladin iPad Shell v2.dc.html:559`). Typed, because Kotlin can.
 */
sealed interface Surface {
    data class Dest(val destination: Destination) : Surface
    data class Doc(val key: TabKey) : Surface

    /**
     * The columns, full width.
     *
     * A surface but **not a destination**, and reachable two ways that must not be confused:
     * promoting the dropdown to a tab (which puts [OpenTab.Browser] in the open set), and a
     * breadcrumb jump from a document (which does not). So a browser surface can be showing
     * while nothing browser-shaped is open — the prototype's `goBrowserAt` pushes `sel` and
     * never touches `mru` (`Aladin iPad Shell v2.dc.html:668`).
     */
    data object Browser : Surface {
        /** Its name everywhere it is written: the crumb, the Open row, the dropdown header. */
        const val TITLE: String = "Browser"
    }
}

/**
 * One member of the open set.
 *
 * The browser tab sits here beside documents because the design says it behaves like one:
 * it appears in Open, it takes a place in recency, `⌃Tab` cycles through it, and closing it
 * prunes the trail exactly as closing a document does.
 *
 * It is **not** a [TabKey], though, and that distinction is load-bearing. A `TabKey` names a
 * row in the tree — it is what the store is asked about, and therefore what "gone" is decided
 * on. The browser names no row. Modelled as a key it would be asked about, answered for with
 * nothing, read as [Presence.Gone], and closed by [Nav.corrected] on the frame after it
 * opened.
 */
sealed interface OpenTab {

    /** Where this tab points. What the trail stores, and what closing prunes. */
    val surface: Surface

    data class Doc(val key: TabKey) : OpenTab {
        override val surface: Surface get() = Surface.Doc(key)
    }

    data object Browser : OpenTab {
        override val surface: Surface get() = Surface.Browser
    }
}

/**
 * One position in the history trail.
 *
 * [path] and [itemId] are the **Browser surface's** column selection — what its columns show
 * when you land there — and deliberately *not* a document's own context. A document's folder
 * is read from the node, which is why truncating a trail entry never makes a breadcrumb wrong.
 *
 * [path] is a **Miller path**: node ids, root-most first, one per column beyond the root list.
 * Column *i* renders the children of `path[i - 1]`, so the browser goes as deep as the tree
 * does and scrolls horizontally rather than nesting. That answers the handoff's open question
 * about depth.
 *
 * **A selected leaf is the last element of the same array**, not a field beside it. That is
 * not tidying: with a separate `itemId`, picking a leaf in a shallow column left the deeper
 * columns standing, and the prototype's single row handler truncates for leaves and folders
 * alike (`:738`). One array makes the truncation unconditional, and a leaf simply produces no
 * column of its own. Which trailing ids are folders is a question for the store, never for
 * this value — see [Nav.select].
 */
data class Entry(
    val surface: Surface,
    val path: List<String> = emptyList(),
)

/**
 * Where you have been, and what is open. The whole navigation model.
 *
 * Replaces five independently-mutated sources of "where am I" — a sidebar stack, an open
 * set, a side map of positions per open item, a correction effect that wrote back into its
 * own input, and a pager index — with one value. Being a value is the point: "where does
 * Back go from a subfolder after closing the active document" is a unit test here, and was
 * previously unreachable inside a 201-line presenter.
 *
 * ### Why four fields and not two
 *
 * [entries] answers *where have I been*; [open] answers *what is open*. Neither derives from
 * the other, and the proof is one sequence:
 *
 * ```
 * open A, B, C        entries [A,B,C] index 2   open [A,B,C]
 * back twice          entries [A,B,C] index 0   open [A,B,C]
 * open D              entries [A,D]   index 1   open [A,B,C,D]
 * ```
 *
 * Navigating somewhere new after stepping back **truncates** the trail — but B and C are
 * still open documents, still one tap away, still holding their surfaces. History and open
 * tabs are separate in a browser for the same reason.
 *
 * [open] and [mru] are then two orderings of one set. [open] is **append order and stable**,
 * because a list that reorders under a finger moves every other target on the way to the one
 * you wanted (the desktop makes the same call, `workspace-slice.ts:42-56`). [mru] is
 * recency, which the switcher's cycling and the collapsed dock's two pills need. [open] is
 * authoritative for membership; [mru] is only a hint — see [switcherOrder].
 */
data class Nav(
    val entries: List<Entry> = listOf(Entry(Surface.Dest(Destination.Home))),
    val index: Int = 0,
    val open: List<OpenTab> = emptyList(),
    val mru: List<OpenTab> = emptyList(),
) {

    val here: Entry get() = entries[index]
    val canBack: Boolean get() = index > 0
    val canForward: Boolean get() = index < entries.lastIndex

    /** The open document being shown, or null on a destination or the browser. */
    val activeDoc: TabKey? get() = (here.surface as? Surface.Doc)?.key

    /**
     * Whether the browser has been promoted to a tab.
     *
     * The toggle's rule reads this: with a tab open, the button **activates it** rather than
     * stacking a dropdown over it, because only one browser is ever live (`:794`).
     */
    val hasBrowserTab: Boolean get() = open.contains(OpenTab.Browser)

    // ── the trail ────────────────────────────────────────────────────────────

    /**
     * Appends a position, discarding anything ahead of where you are.
     *
     * Truncation is what keeps [entries] a **single path**. Real navigation is a tree — going
     * back and then somewhere new branches — and a list can hold only one path through it.
     * Without the truncation, `back` and `forward` stop being inverses and you can walk in
     * circles through states that were never adjacent. Browsers accept the same loss.
     *
     * Re-pushing the position you are already on is a no-op, so tapping the current
     * destination twice does not leave a duplicate that makes Back appear to do nothing.
     */
    fun push(entry: Entry): Nav {
        if (entry == here) return this
        val kept = (entries.take(index + 1) + entry).takeLast(TRAIL_CAP)
        return copy(entries = kept, index = kept.lastIndex)
    }

    /** Back and forward. Clamped: a step past either end is a no-op, never a wrap. */
    fun step(delta: Int): Nav {
        val target = index + delta
        return if (target in entries.indices) copy(index = target) else this
    }

    /** A destination tap. Keeps the Browser's column selection, so returning to it is intact. */
    fun goTo(destination: Destination): Nav =
        push(here.copy(surface = Surface.Dest(destination)))

    /**
     * The breadcrumb's `Browser` jump: show the columns **at a path**.
     *
     * This is what makes the crumb meaningful from a document — it lands you on the document
     * in its own folder, columns and all, rather than at the root. It deliberately does **not**
     * open a browser tab: the surface shows without joining the open set, matching `:668`.
     */
    fun goToBrowser(path: List<String>): Nav = push(Entry(Surface.Browser, path))

    // ── the Browser's columns ────────────────────────────────────────────────

    /**
     * Picks the row [id] in column [column], where 0 is the root list.
     *
     * **Truncate, then append** — the defining Miller move, and it applies to leaves exactly as
     * it does to folders (`:738` has one handler for both). Choosing anything in a column you
     * had already descended past discards every column to its right, because those columns
     * described a path you are no longer on. That is not a special case to handle; it is what
     * the columns *mean*, and it is the only "up" the design has.
     *
     * Folder or leaf is not asked here and could not be answered here — this is a value, and
     * kind lives in the store. A folder contributes a column of children; a leaf contributes
     * none and is simply the selection. Both are the same move.
     *
     * **Not a history entry.** Browsing columns is not navigating (`:738` sets state directly
     * rather than pushing) — otherwise every tap in the columns would need a Back press to undo.
     */
    fun select(column: Int, id: String): Nav =
        replaceHere(here.copy(path = here.path.take(column) + id))

    // ── the open set ─────────────────────────────────────────────────────────

    /**
     * Opens a document, or returns to one already open.
     *
     * Registers, activates and records recency in one move. The previous shell had a separate
     * `register()` helper called from four branches precisely because opening was a second
     * step someone had to remember; here there is no way to navigate to a document without
     * opening it.
     *
     * A document already open **keeps its place** in [open] rather than jumping to the end.
     */
    fun openDoc(key: TabKey): Nav = openTab(OpenTab.Doc(key))

    /**
     * Promotes the browser to a tab, or returns to the tab already open.
     *
     * One function for both, because they are the same move: the maximize glyph, the
     * context menu's "Open in a tab", and the toggle finding a tab already open all land here
     * (`:794`, `:819`) and all push history — promoting *is* a navigation, unlike opening the
     * dropdown, which is not.
     */
    fun openBrowserTab(): Nav = openTab(OpenTab.Browser)

    private fun openTab(tab: OpenTab): Nav =
        copy(
            open = if (open.contains(tab)) open else open + tab,
            mru = listOf(tab) + mru.filterNot { it == tab },
        ).push(here.copy(surface = tab.surface))

    /**
     * Closes a document: it stops being open, and every trail entry showing it is pruned so
     * back/forward can never land on it.
     *
     * The fallback is **history-positional, not most-recent**. Pruning can remove entries from
     * the middle, so the index is re-clamped into what survives and that entry is taken
     * wholesale — which is how you land somewhere you actually were. Most-recent-survivor
     * applies only when nothing survives at all. (The desktop's own rule is different again —
     * the last of its insertion-ordered tab list — and its comment defers the question rather
     * than settling it; `workspace-slice.ts:85-87`. The prototype settles it, :576-587.)
     */
    fun close(tab: OpenTab): Nav {
        if (!open.contains(tab)) return this
        val remainingOpen = open.filterNot { it == tab }
        val remainingMru = mru.filterNot { it == tab }
        val kept = entries.filterNot { it.surface == tab.surface }

        if (kept.isEmpty()) {
            val fallback = remainingMru.firstOrNull()?.surface
                ?: Surface.Dest(Destination.Home)
            return Nav(
                entries = listOf(here.copy(surface = fallback)),
                index = 0,
                open = remainingOpen,
                mru = remainingMru,
            )
        }

        val wasShowing = here.surface == tab.surface
        val landing = when {
            wasShowing -> index.coerceAtMost(kept.lastIndex)
            // Still showing something else: find where that is now that entries were removed.
            else -> kept.indexOfLast { it == here }.takeIf { it >= 0 }
                ?: index.coerceAtMost(kept.lastIndex)
        }
        return copy(
            entries = kept,
            index = landing.coerceAtLeast(0),
            open = remainingOpen,
            mru = remainingMru,
        )
    }

    /**
     * The switcher's rows: recency order, reconciled against what is actually open.
     *
     * [open] is authoritative, so a stale [mru] can neither hide a document nor render a
     * phantom — the worst it can do is order slightly wrong. Mirrors `orderByMru`
     * (`domain.ts:59-77`).
     *
     * **Cycling deliberately does not live here.** A `step` on this list would call
     * [openDoc], which promotes — so each step would reorder the very list being stepped
     * through, and holding ⌃Tab would wander rather than advance. The desktop solves it by
     * freezing the order for as long as the overlay is open (`use-tab-switcher.ts:46-49`:
     * *"if the list re-sorted live, cycling would move the rows you are cycling through under
     * your own fingers"*). That freeze is a property of the overlay's session, not of this
     * value: a switcher session takes this order once at open, moves a highlight within it,
     * and calls [openDoc] once on commit.
     */
    fun switcherOrder(): List<OpenTab> {
        val remaining = open.toMutableList()
        val ordered = mru.filter(remaining::remove)
        return ordered + remaining
    }

    // ── correction ───────────────────────────────────────────────────────────

    /**
     * This position, corrected for what the tree says is gone.
     *
     * **Applied on read, never written back.** The previous shell ran its correction in a
     * `LaunchedEffect` that assigned to the very value the effect was keyed on — a loop
     * running against store reads that lag the position they describe by a frame, and the
     * reason a drill could revert itself. Here it is a pure function of a value and a
     * question, so applying it every frame costs nothing and converges by construction. It
     * also *un-corrects* on its own when a row comes back — which a write-back can never do,
     * having destroyed the original.
     *
     * [presenceOf] answers per **id**, and only [Presence.Gone] moves anything. An id nobody
     * has read answers [Presence.Unknown] and is left alone: conflating "not read yet" with
     * "deleted" is what shipped twice.
     *
     * Only [here] is corrected, not every entry. Rendering only ever asks about the position
     * you are standing on, and a stale folder further back in the trail is corrected when you
     * step onto it.
     *
     * The one rule for callers: **events must start from the corrected value**, so the
     * correction settles into storage the moment the user acts rather than drifting.
     */
    fun corrected(presenceOf: (String) -> Presence): Nav {
        fun gone(id: String?): Boolean = id != null && presenceOf(id) == Presence.Gone

        // A document the workspace destroyed stops being open — the same path as closing it,
        // so the trail is pruned and the landing is picked by one rule rather than two.
        //
        // **Only documents are ever asked about.** The browser tab names no row, so there is
        // nothing to ask and nothing that can answer Gone. Asking anyway is the failure this
        // guards: the store answers null for an id it has no row for, that reads as Gone, and
        // the tab closes itself on the frame after it opened.
        val settled = open
            .filterIsInstance<OpenTab.Doc>()
            .filter { gone(it.key.nodeId) }
            .fold(this) { nav, tab -> nav.close(tab) }

        val entry = settled.here
        // A deleted folder takes every column to its right with it — those columns described
        // its descendants, which are unreachable now. One pass, so a whole deleted branch
        // unwinds at once rather than one rung per frame. A selected leaf is the last element,
        // so the same truncation carries it off when its folder goes, with no separate rule.
        val keptPath = entry.path.takeWhile { !gone(it) }
        val fixed = entry.copy(path = keptPath)
        return if (fixed == entry) settled else settled.replaceHere(fixed)
    }

    private fun replaceHere(entry: Entry): Nav =
        copy(entries = entries.toMutableList().also { it[index] = entry })

    companion object {

        /**
         * A long session should not grow an unbounded trail. Invisible to the user — nobody
         * steps back fifty positions — and it bounds the memory.
         */
        const val TRAIL_CAP: Int = 50
    }
}
