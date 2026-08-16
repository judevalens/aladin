package dawn.system.anchor.features.shell.state

import androidx.compose.runtime.Composable
import dawn.system.anchor.domain.ArtifactKind
import dawn.system.anchor.domain.Destination
import dawn.system.anchor.domain.ItemState
import dawn.system.anchor.domain.Nav
import dawn.system.anchor.domain.Surface
import dawn.system.anchor.domain.TabKey
import dawn.system.anchor.domain.artifactKind
import dawn.system.anchor.services.data.NodeStore

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
 * **Exactly one crumb is ever tappable** — `Browser`, and only while viewing a document. The
 * prototype wires no others (`:632-637`), despite the README describing ancestors as
 * tappable; the prototype is the deliverable.
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
                val folder = nodeOf(here.folderId)
                val item = nodeOf(here.itemId)
                buildList {
                    add(Crumb(Destination.Browser.title, null))
                    folder?.let { add(Crumb(it.title.orUntitled(), null)) }
                    item?.let { add(Crumb(it.title.orUntitled(), null)) }
                }
            } else {
                listOf(Crumb(surface.destination.title, null))
            }

        is Surface.Doc -> {
            val doc = nodeOf(surface.key.nodeId)
            val folder = nodeOf(doc?.parentId)
            buildList {
                add(
                    Crumb(
                        label = Destination.Browser.title,
                        target = doc?.let { NavEvent.GoToBrowser(it.parentId, it.id) },
                    ),
                )
                folder?.let { add(Crumb(it.title.orUntitled(), null)) }
                add(Crumb(doc?.title.orUntitled(), null))
            }
        }
    }
}
