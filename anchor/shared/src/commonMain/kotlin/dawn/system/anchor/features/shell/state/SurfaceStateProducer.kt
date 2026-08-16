package dawn.system.anchor.features.shell.state

import androidx.compose.runtime.Composable
import com.slack.circuit.runtime.CircuitUiState
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import dawn.system.anchor.domain.Destination
import dawn.system.anchor.domain.ArtifactKind
import dawn.system.anchor.domain.artifactKind
import dawn.system.anchor.domain.OpenItems
import dawn.system.anchor.domain.SurfaceSlot
import dawn.system.anchor.domain.WorkspaceNode
import dawn.system.anchor.domain.indexOfNode
import dawn.system.anchor.domain.sectionsFor
import dawn.system.anchor.domain.slotsFor
import dawn.system.anchor.features.note.WebPane
import dawn.system.anchor.features.shell.ShellScreen
import dawn.system.anchor.services.data.ArtifactResourceStore
import dawn.system.anchor.services.data.DocumentStore
import dawn.system.anchor.services.data.NodeStore
import dawn.system.anchor.services.data.each
import dawn.system.anchor.services.data.eachChildren

/** What is open, as pager pages, switcher rows and web panes. */
data class SurfaceSlice(
    val pages: List<ShellScreen.Page>,
    val activePage: Int,
    val tabs: List<ShellScreen.OpenTab>,
    val webPanes: List<WebPane>,
) : CircuitUiState

/**
 * Turns the open list into everything that draws it.
 *
 * **One subscription per open item, combined.** Each row watches only itself, so renaming one
 * open note re-emits that one and nothing else — and `combine` keeps them in the order they
 * were opened, so there is no list to index and no order to restore by hand. A row that has
 * been deleted simply stops appearing.
 *
 * Opening a fourth item rebuilds the *combination* but not the reads: the store's streams are
 * retained per id, so the three already open replay what they hold and only the new one
 * touches the database.
 */
class SurfaceStateProducer(
    private val nodes: NodeStore,
    private val resources: ArtifactResourceStore,
    private val documents: DocumentStore,
) {

    @Composable
    operator fun invoke(openItems: OpenItems, activeNodeId: String?): SurfaceSlice {
        val openIds = remember(openItems) { openItems.items.map { it.nodeId } }
        val openNodes by remember(openIds) { nodes.each(openIds) }
            .collectAsState(initial = emptyList())

        // The switcher groups by owning folder, so those folders are watched the same way.
        val ownerIds = remember(openNodes) { openNodes.mapNotNull { it.parentId }.distinct() }
        val ownerNodes by remember(ownerIds) { nodes.each(ownerIds) }
            .collectAsState(initial = emptyList())

        // Each open folder's contents, one subscription each — a detail page draws its OWN
        // sections, not the current selection's, or two open folders render identically.
        val openFolderIds = remember(openNodes) { openNodes.filter { it.isContainer }.map { it.id } }
        val openFolderRows by remember(openFolderIds) { nodes.eachChildren(openFolderIds) }
            .collectAsState(initial = emptyList())

        // Paired with their ids, so an item whose row has not arrived still gets a slot.
        // Driving this off the resolved rows alone let a loading read empty the pager.
        val open = remember(openItems, openNodes) {
            openItems.items.map { item -> item.nodeId to openNodes.firstOrNull { it.id == item.nodeId } }
        }
        val slots = remember(open) { slotsFor(open) }

        // Every web-backed artifact that is open. They share one slot and one view; which of them
        // shows is settled inside the page, not by the pager.
        val webPanes = remember(openNodes) {
            openNodes.mapNotNull { node ->
                when (node.artifactKind) {
                    ArtifactKind.Page -> WebPane.Kind.Page
                    ArtifactKind.App -> WebPane.Kind.Shard
                    else -> null
                }?.let { kind -> WebPane(node.id, kind, node.title.ifBlank { UNTITLED }) }
            }
        }

        val pages = remember(slots, openFolderRows, resources, documents) {
            val childrenByFolder = openFolderRows.groupBy { it.parentId }
            slots.map { slot ->
                when (slot) {
                    SurfaceSlot.Web -> ShellScreen.Page.Web
                    is SurfaceSlot.Artifact ->
                        ShellScreen.Page.Artifact(surfaceFor(slot.node, resources, documents))
                    is SurfaceSlot.Pending -> ShellScreen.Page.Pending(slot.nodeId)
                    is SurfaceSlot.Detail -> ShellScreen.Page.Detail(
                        node = slot.node,
                        sections = sectionsFor(
                            slot.node,
                            childrenByFolder[slot.node.id].orEmpty(),
                            includeContents = true,
                        ),
                    )
                }
            }
        }

        // Built from the live rows, never from a snapshot. An item whose node is gone — deleted
        // here or on another device — stops being a row rather than lingering as a tab that
        // opens nothing.
        val tabs = remember(openItems, openNodes, ownerNodes) {
            openItems.items.mapNotNull { item ->
                openNodes.firstOrNull { it.id == item.nodeId }?.let { node ->
                    ShellScreen.OpenTab(
                        key = item.key,
                        nodeId = node.id,
                        title = node.title.ifBlank { UNTITLED },
                        owner = ownerLabel(node, ownerNodes, item.destination),
                        meta = node.artifactType,
                        active = item.key == openItems.activeKey,
                    )
                }
            }
        }

        return SurfaceSlice(
            pages = pages,
            activePage = remember(slots, activeNodeId) { slots.indexOfNode(activeNodeId) },
            tabs = tabs,
            webPanes = webPanes,
        )
    }
}

/**
 * Only the kinds that have a surface of their own. Pages and shards are absent by
 * construction: they are the web slot.
 */
private fun surfaceFor(
    node: WorkspaceNode,
    resources: ArtifactResourceStore,
    documents: DocumentStore,
) = ShellScreen.ArtifactSurface(
    node = node,
    kind = when (node.artifactKind) {
        ArtifactKind.File -> ShellScreen.ArtifactSurface.Kind.Document
        ArtifactKind.Voice -> ShellScreen.ArtifactSurface.Kind.Voice
        ArtifactKind.Link -> ShellScreen.ArtifactSurface.Kind.Link
        else -> ShellScreen.ArtifactSurface.Kind.Other
    },
    resources = resources,
    documents = documents,
)

/** The switcher's grouping heading: the owning folder, or the destination it came from. */
private fun ownerLabel(
    node: WorkspaceNode,
    owners: List<WorkspaceNode>,
    destination: Destination,
): String = owners.firstOrNull { it.id == node.parentId }
    ?.title?.takeIf { it.isNotBlank() }
    ?: destination.title

private const val UNTITLED = "Untitled"
