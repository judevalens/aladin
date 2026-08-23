package dawn.system.anchor.features.shell.state

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import com.slack.circuit.runtime.CircuitUiState
import dawn.system.anchor.domain.ArtifactKind
import dawn.system.anchor.domain.BrowserFilter
import dawn.system.anchor.domain.BrowserTree
import dawn.system.anchor.domain.FolderPurpose
import dawn.system.anchor.domain.Nav
import dawn.system.anchor.domain.NodeKind
import dawn.system.anchor.services.data.NodeStore

/** Something the user did to the tree. */
sealed interface TreeEvent {
    /** A folder row tap: expand or collapse it, and make it where new things land. */
    data class ToggleFolder(val id: String) : TreeEvent

    /** An artifact row tap: the folder it sits in becomes where new things land. */
    data class FocusFolder(val id: String?) : TreeEvent
}

/** One row of the sidebar tree. Resolved — the UI never receives a node. */
data class TreeRow(
    val id: String,
    val title: String,
    /** Visual depth, capped at [TREE_DEPTH_CAP]; deeper rows render at the cap's inset. */
    val depth: Int,
    val isContainer: Boolean,
    val kind: ArtifactKind?,
    val purpose: FolderPurpose?,
    val expanded: Boolean,
    /** Whether expanding would show anything, after the filter — dims the chevron when not. */
    val hasChildren: Boolean,
    /** A folder's live matching-item count; null on a leaf. */
    val count: Int?,
    /** The document being shown — a research folder lights while its Overview is up. */
    val active: Boolean,
    /** `NodeKind.Research` itself, not an inherited purpose: only these have an Overview. */
    val isResearch: Boolean,
    val parentId: String?,
)

/** Where the plus will put a new item, resolved for the menu's footer. */
data class CreateTarget(val folderId: String?, val title: String)

data class TreeSlice(
    val rows: List<TreeRow>,
    val focusedFolderId: String?,
    val createTarget: CreateTarget,
    val handle: (TreeEvent) -> Unit,
) : CircuitUiState

/** Levels of indent the sidebar can afford; hierarchy beyond it is carried by the chevrons. */
internal const val TREE_DEPTH_CAP = 4

/**
 * The sidebar's folder tree — the everyday browser, always visible, one tap opens.
 *
 * **Local state holds identity.** The producer remembers two sets of ids — which folders are
 * expanded and which one is focused — and derives every row from the store each frame, so a
 * rename reaches the tree the same frame it reaches the breadcrumb. It flattens from the ONE
 * whole-tree subscription the Browser already pays for ([BrowserTree]) rather than
 * re-combining per-folder streams on every toggle, which is what would walk straight into the
 * unkeyed `collectAsState` carry-over the NodeReads header warns about.
 *
 * **Reveal on open, not per frame.** Opening a document expands its ancestors once, as an
 * effect of the open; the user may collapse them afterwards and they stay collapsed. A
 * per-frame derivation would force them open again every frame.
 */
class TreeStateProducer(private val nodes: NodeStore) {

    @Composable
    operator fun invoke(nav: Nav, filter: BrowserFilter): TreeSlice {
        val all by remember { nodes.liveNodes() }.collectAsState(emptyList())
        val tree = remember(all) { BrowserTree(all) }

        var expanded by remember { mutableStateOf(emptySet<String>()) }
        var focusedFolderId by remember { mutableStateOf<String?>(null) }

        val activeId = nav.activeDoc?.nodeId

        // Keyed reads, root-most first, excluding the document itself. The effect re-runs when
        // the chain GROWS as unread rows get read; collapsing an ancestor does not change the
        // key, so it stays collapsed. Re-activating the same tab does not re-reveal.
        val ancestors = nodes.ancestorPathOf(activeId)
        LaunchedEffect(activeId, ancestors) {
            if (activeId != null && ancestors.isNotEmpty()) expanded = expanded + ancestors
        }

        val rows = remember(tree, expanded, filter, activeId) {
            flattenTree(tree, expanded, filter, activeId)
        }

        // Where a new item lands: the focused folder if it still is one, else the open
        // document's folder (a research Overview → the research folder itself), else the
        // Browser's standing folder, else the root.
        val focused = nodes.nodeOf(focusedFolderId)?.takeIf { it.isContainer }
        val activeNode = nodes.nodeOf(activeId)
        val docFolderId = activeNode?.let { if (it.isContainer) it.id else it.parentId }
        val pathNode = nodes.nodeOf(nav.here.path.lastOrNull())
        val pathFolderId = pathNode?.let { if (it.isContainer) it.id else it.parentId }
        val targetId = focused?.id ?: docFolderId ?: pathFolderId
        val targetTitle = when {
            targetId == null -> "Workspace"
            else -> nodes.nodeOf(targetId)?.title?.takeIf { it.isNotBlank() } ?: "Untitled"
        }

        return TreeSlice(
            rows = rows,
            focusedFolderId = focusedFolderId,
            createTarget = CreateTarget(targetId, targetTitle),
        ) { event ->
            when (event) {
                is TreeEvent.ToggleFolder -> {
                    focusedFolderId = event.id
                    expanded = if (event.id in expanded) expanded - event.id else expanded + event.id
                }
                is TreeEvent.FocusFolder -> focusedFolderId = event.id
            }
        }
    }
}

/**
 * The tree as rows: a depth-first walk of the expanded folders, each level filtered and
 * ordered by the same rule the Browser's columns use. Pure, so the order and the depth cap
 * are unit-testable without a composition.
 */
internal fun flattenTree(
    tree: BrowserTree,
    expanded: Set<String>,
    filter: BrowserFilter,
    activeId: String?,
): List<TreeRow> {
    val keepEmpty = !filter.isNarrowing
    val out = ArrayList<TreeRow>()

    fun childRows(parentId: String?) =
        childRowsOf(tree.childrenOf(parentId), activeId, filter, tree, keepEmptyFolders = keepEmpty)

    fun walk(parentId: String?, depth: Int) {
        // A cycle in a partially-synced replica must not hang the sidebar.
        if (depth > MAX_WALK_DEPTH) return
        for (row in childRows(parentId)) {
            val node = tree.node(row.id)
            val isExpanded = row.isContainer && row.id in expanded
            out += TreeRow(
                id = row.id,
                title = row.title,
                depth = minOf(depth, TREE_DEPTH_CAP),
                isContainer = row.isContainer,
                kind = row.kind,
                purpose = row.purpose,
                expanded = isExpanded,
                hasChildren = row.isContainer && childRows(row.id).isNotEmpty(),
                count = row.count,
                active = row.selected,
                isResearch = node?.kind == NodeKind.Research,
                parentId = node?.parentId,
            )
            if (isExpanded) walk(row.id, depth + 1)
        }
    }
    walk(null, 0)
    return out
}

private const val MAX_WALK_DEPTH = 16
