package dawn.system.anchor.features.shell.state

import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import dawn.system.anchor.domain.ArtifactKind
import dawn.system.anchor.domain.Destination
import dawn.system.anchor.domain.ItemState
import dawn.system.anchor.domain.Nav
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
    val key: TabKey,
    val title: String,
    /** The folder that owns it, as secondary text — two same-named notes stay distinguishable. */
    val folder: String?,
    val kind: ArtifactKind?,
    /** Always null today: nothing on the sync frame can report unread, recovered or stale. */
    val state: ItemState?,
    val active: Boolean,
)

/** One breadcrumb segment. [target] is null for the ones that are not tappable. */
data class Crumb(val label: String, val target: NavEvent?)

/**
 * The Open list, in [Nav.open]'s **append order** — deliberately not recency.
 *
 * A list that reorders under a finger moves every other target on the way to the one you
 * wanted. Recency belongs to the switcher, which freezes it for the life of the overlay for
 * the same reason.
 */
@Composable
fun NodeStore.openRowsFor(nav: Nav): List<OpenRow> =
    nav.open.map { key ->
        val node = nodeOf(key.nodeId)
        val folder = nodeOf(node?.parentId)
        OpenRow(
            key = key,
            title = node?.title.orUntitled(),
            folder = folder?.title?.takeIf { it.isNotBlank() },
            kind = node?.artifactKind,
            state = null,
            active = key == nav.activeDoc,
        )
    }

/**
 * The breadcrumb — the shell's whole sense of place, and the reason there is no path popover.
 *
 * Ids in, labels resolved: a rename reaches the crumb, the Open row and the Browser columns
 * together because none of them holds a copy.
 *
 * On the Browser the crumbs are the **columns**, and each is tappable: tapping one truncates
 * the path back to it, which is the same move as re-picking that folder in its own column.
 * From a document only `Browser` is tappable, and it jumps to that document in its own folder.
 *
 * (The prototype leaves the Browser crumbs inert, because its fixture is one level deep and
 * there is nothing to truncate back to. With real Miller depth they have to work.)
 *
 * Note where a document's crumb reads its folder from: **the node**, not [Nav.Entry.folderId].
 * The entry's ids are the Browser surface's column selection, a different thing — which is why
 * truncating a trail entry can never make a breadcrumb wrong.
 */
@Composable
fun NodeStore.crumbsFor(nav: Nav): List<Crumb> {
    val here = nav.here
    return when (val surface = here.surface) {
        is Surface.Dest ->
            if (surface.destination == Destination.Browser) {
                // One crumb per column, so the trail of folders you descended is legible
                // without counting columns. Tapping one truncates back to it — the same
                // move as picking that folder again in its own column.
                val path by remember(here.path) { each(here.path) }.collectAsState(emptyList())
                val item = nodeOf(here.itemId)
                buildList {
                    add(Crumb(Destination.Browser.title, NavEvent.GoToBrowser(emptyList(), null)))
                    here.path.forEachIndexed { depth, id ->
                        val title = path.firstOrNull { it.id == id }?.title.orUntitled()
                        add(Crumb(title, NavEvent.GoToBrowser(here.path.take(depth + 1), null)))
                    }
                    item?.let { add(Crumb(it.title.orUntitled(), null)) }
                }
            } else {
                listOf(Crumb(surface.destination.title, null))
            }

        is Surface.Doc -> {
            val doc = nodeOf(surface.key.nodeId)
            val folder = nodeOf(doc?.parentId)
            // The full Miller path, so the crumb lands you on the document in its own
            // folder with every column above it — not at the root.
            val ancestors = ancestorPathOf(doc?.parentId)
            buildList {
                add(
                    Crumb(
                        label = Destination.Browser.title,
                        target = doc?.let { NavEvent.GoToBrowser(ancestors + listOfNotNull(it.parentId), it.id) },
                    ),
                )
                folder?.let { add(Crumb(it.title.orUntitled(), null)) }
                add(Crumb(doc?.title.orUntitled(), null))
            }
        }
    }
}
