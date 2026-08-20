package dawn.system.anchor.features.shell.state

import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.remember
import dawn.system.anchor.domain.ArtifactKind
import dawn.system.anchor.domain.BrowserFilter
import dawn.system.anchor.domain.Destination
import dawn.system.anchor.domain.ItemState
import dawn.system.anchor.domain.Nav
import dawn.system.anchor.domain.OpenTab
import dawn.system.anchor.domain.Surface
import dawn.system.anchor.domain.TabKey
import dawn.system.anchor.domain.artifactKind
import dawn.system.anchor.services.data.NodeStore
import dawn.system.anchor.services.data.each

private const val UNTITLED = "Untitled"

private fun String?.orUntitled(): String = this?.takeIf { it.isNotBlank() } ?: UNTITLED

/**
 * One row of the Open list.
 *
 * **Everything here is resolved.** The row receives strings and enums, never a
 * `WorkspaceNode` — so it cannot re-derive a label and become the sixth place that answers
 * "what do I call this node". That was five places in the shell this replaces.
 */
data class OpenRow(
    val tab: OpenTab,
    val title: String,
    /**
     * The **full** ancestor chain, joined by ` › `.
     *
     * Not just the parent: a flat recency list has no other way to tell two same-named notes
     * apart, and a document five levels deep is otherwise unidentifiable in it.
     */
    val folder: String?,
    /** Null for the browser tab, which is not an artifact and has no kind to show. */
    val kind: ArtifactKind?,
    /** Always null today: nothing on the sync frame can report unread, recovered or stale. */
    val state: ItemState?,
    val active: Boolean,
)

/**
 * A stable list key for one open tab. Identity, not position — removing a row must move the
 * others rather than re-map their content.
 */
fun OpenTab.rowKey(): String = when (this) {
    OpenTab.Browser -> "browser"
    is OpenTab.Doc -> key.asString()
}

/** The event that shows this tab: opening a document and returning to the browser are one move. */
fun OpenTab.openEvent(): NavEvent = when (this) {
    OpenTab.Browser -> NavEvent.OpenBrowserTab
    is OpenTab.Doc -> NavEvent.OpenDoc(key)
}

/** One breadcrumb segment. [target] is null for the ones that are not tappable. */
data class Crumb(val label: String, val target: NavEvent?)

/**
 * The Open list, in [Nav.open]'s **append order** — deliberately not recency.
 *
 * A list that reorders under a finger moves every other target on the way to the one you
 * wanted. Recency belongs to the switcher and to the collapsed dock's two pills, which the
 * design labels "recent first" for exactly that reason; the sidebar list is the one place
 * stability matters more.
 *
 * The browser tab resolves without a node, because it names none — it is a place, not a row,
 * and it is **exempt from the filter**: narrowing to PDFs must not hide the browser.
 */
@Composable
fun NodeStore.openRowsFor(nav: Nav, filter: BrowserFilter = BrowserFilter()): List<OpenRow> =
    nav.open.mapNotNull { tab ->
        // KEYED, and not optionally. Each row subscribes per id, and an unkeyed read carries
        // the previous row's value into the next when the list changes — the bug that made
        // every drill revert, in a different place.
        key(tab.rowKey()) {
            when (tab) {
                OpenTab.Browser -> OpenRow(
                    tab = tab,
                    title = Surface.Browser.TITLE,
                    folder = null,
                    kind = null,
                    state = null,
                    active = nav.here.surface == Surface.Browser,
                )

                is OpenTab.Doc -> {
                    val node = nodeOf(tab.key.nodeId)
                    val chain = ancestorPathOf(node?.parentId) + listOfNotNull(node?.parentId)
                    val rows by remember(chain) { each(chain) }.collectAsState(emptyList())
                    OpenRow(
                        tab = tab,
                        title = node?.title.orUntitled(),
                        folder = chain
                            .mapNotNull { id -> rows.firstOrNull { it.id == id }?.title }
                            .filter { it.isNotBlank() }
                            .joinToString(" › ")
                            .takeIf { it.isNotBlank() },
                        kind = node?.artifactKind,
                        state = null,
                        active = tab.key == nav.activeDoc,
                    ).takeIf {
                        // The browser tab above is exempt outright; a document is kept only
                        // if it survives. Filtering by "PDFs" must not hide the browser, and
                        // must not leave a note in a list that claims to show only PDFs.
                        filter.matches(kind = node?.artifactKind, purpose = null, state = null)
                    }
                }
            }
        }
    }

/**
 * The breadcrumb — the shell's whole sense of place, and the reason there is no path popover.
 *
 * Ids in, labels resolved: a rename reaches the crumb, the Open row and the Browser columns
 * together because none of them holds a copy.
 *
 * On the Browser the crumbs are the **columns**, and each is tappable: tapping one truncates
 * the path back to it, which is the same move as re-picking that row in its own column. From a
 * document every ancestor is tappable too, and each jumps into the browser at its own level.
 *
 * Note where a document's crumbs read their folders from: **the nodes**, not the entry's path.
 * The entry's path is the Browser surface's column selection, a different thing — which is why
 * truncating a trail entry can never make a breadcrumb wrong.
 *
 * **Past four crumbs the middle collapses to `…`**, so depth never pushes the leaf off the bar.
 * That is the whole reason an arbitrarily deep tree stays legible here.
 */
@Composable
fun NodeStore.crumbsFor(nav: Nav): List<Crumb> {
    val here = nav.here
    val crumbs = when (val surface = here.surface) {
        is Surface.Dest -> listOf(Crumb(surface.destination.title, null))

        // One crumb per column, so the trail you descended is legible without counting
        // columns. Tapping one truncates back to it — the same move as picking that row again
        // in its own column. A selected leaf is the last path element, so it gets a crumb from
        // the same loop rather than a rule of its own.
        Surface.Browser -> {
            val nodes by remember(here.path) { each(here.path) }.collectAsState(emptyList())
            buildList {
                add(Crumb(Surface.Browser.TITLE, NavEvent.GoToBrowser(emptyList())))
                here.path.forEachIndexed { depth, id ->
                    val title = nodes.firstOrNull { it.id == id }?.title.orUntitled()
                    add(Crumb(title, NavEvent.GoToBrowser(here.path.take(depth + 1))))
                }
            }
        }

        is Surface.Doc -> {
            val doc = nodeOf(surface.key.nodeId)
            // The **full** ancestor chain, every crumb tappable and each jumping into the
            // browser at its own level — not just the parent. A document five levels deep is
            // otherwise unreachable from where it sits.
            val chain = ancestorPathOf(doc?.parentId) + listOfNotNull(doc?.parentId)
            val nodes by remember(chain) { each(chain) }.collectAsState(emptyList())
            buildList {
                add(Crumb(Surface.Browser.TITLE, NavEvent.GoToBrowser(emptyList())))
                chain.forEachIndexed { depth, id ->
                    val title = nodes.firstOrNull { it.id == id }?.title.orUntitled()
                    add(Crumb(title, NavEvent.GoToBrowser(chain.take(depth + 1))))
                }
                add(Crumb(doc?.title.orUntitled(), doc?.let { NavEvent.GoToBrowser(chain + it.id) }))
            }
        }
    }
    return crumbs.elided()
}

/**
 * Keeps a breadcrumb to four segments: first, `…`, and the last two.
 *
 * Depth is expressed by the columns; the bar's job is to say where you are, and the leaf is
 * the part of that which must never be pushed off. Mirrors the prototype's `elide` (`:749`).
 */
private fun List<Crumb>.elided(): List<Crumb> =
    if (size <= 4) this else listOf(first(), Crumb("…", null), this[size - 2], last())
