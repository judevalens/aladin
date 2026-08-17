package dawn.system.anchor.features.shell.state

import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import com.slack.circuit.runtime.CircuitUiState
import dawn.system.anchor.domain.ArtifactKind
import dawn.system.anchor.domain.BrowserFilter
import dawn.system.anchor.domain.BrowserTree
import dawn.system.anchor.domain.Entry
import dawn.system.anchor.domain.FolderPurpose
import dawn.system.anchor.domain.ItemSort
import dawn.system.anchor.domain.ItemState
import dawn.system.anchor.domain.NodeKind
import dawn.system.anchor.domain.TabKey
import dawn.system.anchor.domain.WorkspaceNode
import dawn.system.anchor.domain.artifactKind
import dawn.system.anchor.services.data.NodeStore
import dawn.system.anchor.services.data.each
import dawn.system.anchor.services.data.eachChildren

/** One row in a Miller column. Resolved — the UI never receives a node. */
data class BrowserRow(
    val id: String,
    val title: String,
    val isContainer: Boolean,
    val kind: ArtifactKind?,
    val purpose: FolderPurpose?,
    /** Nothing can report this yet; the dot is drawn only when it is non-null. */
    val state: ItemState?,
    val selected: Boolean,
    /** A folder's live item count. Null on a leaf, which has nothing to count. */
    val count: Int? = null,
)

/** One column: a folder's contents, and which of them is on the path. */
data class BrowserColumn(val title: String, val rows: List<BrowserRow>)

/** The third pane — what one selected item is, and what opening it would do. */
data class BrowserDetail(
    val id: String,
    val title: String,
    val kindLabel: String,
    val isContainer: Boolean,
    val openKey: TabKey?,
)

data class BrowserSlice(
    val columns: List<BrowserColumn>,
    val detail: BrowserDetail?,
) : CircuitUiState

/**
 * The Browser destination: **Miller columns**, as deep as the tree goes.
 *
 * Column 0 lists the roots; column *i* lists the children of `path[i - 1]`. So the browser is
 * `path.size + 1` columns wide and scrolls horizontally rather than nesting — which is the
 * answer to the handoff's open question about depth, and the reason this is not a fixed
 * three-pane layout.
 *
 * Every column's contents come from **one** `eachChildren` subscription rather than a query
 * per column: how deep you have descended should not decide how much is watching the
 * database.
 */
class BrowserStateProducer(private val nodes: NodeStore) {

    @Composable
    operator fun invoke(here: Entry, filter: BrowserFilter): BrowserSlice {
        val roots by remember { nodes.children(null) }.collectAsState(emptyList())

        // The whole tree, for the two questions a column cannot answer about itself: how many
        // matching items a folder holds, and what purpose its leaves inherit. It is a local
        // replica, so this is an index over memory rather than a query per folder — but it IS
        // a whole-tree subscription, which the per-column reads deliberately are not.
        val all by remember { nodes.liveNodes() }.collectAsState(emptyList())
        val tree = remember(all) { BrowserTree(all) }

        val deeper by remember(here.path) { nodes.eachChildren(here.path) }
            .collectAsState(emptyList())

        // Titles for the column headings — the path's own rows, resolved rather than copied,
        // so renaming a folder renames its column heading.
        val pathRows by remember(here.path) { nodes.each(here.path) }.collectAsState(emptyList())

        val byParent = remember(deeper) { deeper.groupBy { it.parentId } }

        // Only FOLDERS produce columns. A selected leaf is the last element of the same path
        // and contributes none, which is what makes one array able to carry both (`:740-741`).
        //
        // An element whose row has not been read yet stops the walk rather than being assumed
        // one way or the other: a missing column for a frame is recoverable, an empty column
        // claiming a leaf has no children is a lie. Revisits are seeded from the replay cache,
        // so this is only ever the genuine first read.
        val folderPath = remember(here.path, pathRows) {
            here.path.takeWhile { id -> pathRows.firstOrNull { it.id == id }?.isContainer == true }
        }

        val columns = remember(roots, byParent, pathRows, folderPath, here, filter, tree) {
            buildList {
                add(
                    column(
                        title = "All folders",
                        contents = roots,
                        // Column i's selection is simply `path[i]` now that the leaf lives in
                        // the same array — no fallback, because there is no second field to
                        // fall back to.
                        selectedId = here.path.firstOrNull(),
                        filter = filter,
                        tree = tree,
                    ),
                )
                folderPath.forEachIndexed { depth, folderId ->
                    add(
                        column(
                            title = pathRows.firstOrNull { it.id == folderId }?.title.orUntitled(),
                            contents = byParent[folderId].orEmpty(),
                            selectedId = here.path.getOrNull(depth + 1),
                            filter = filter,
                            tree = tree,
                        ),
                    )
                }
            }
        }

        // The trailing element, and only when it is a leaf — a folder selection is a column,
        // not a thing to open. Mirrors the prototype's `bItem` (`:744`).
        val selected = nodes.nodeOf(here.path.lastOrNull())?.takeIf { it.kind == NodeKind.Artifact }
        return BrowserSlice(
            columns = columns,
            detail = selected?.let { node ->
                BrowserDetail(
                    id = node.id,
                    title = node.title.orUntitled(),
                    kindLabel = node.kindLabel(),
                    isContainer = false,
                    openKey = TabKey.Artifact(node.id),
                )
            },
        )
    }
}

private fun column(
    title: String,
    contents: List<WorkspaceNode>,
    selectedId: String?,
    filter: BrowserFilter,
    tree: BrowserTree,
): BrowserColumn {
    val surviving = contents.filter { node ->
        if (node.isContainer) {
            // A folder is judged by what is INSIDE it, never by itself: filtering for PDFs
            // must not hide every folder, and must not offer one you can only enter to find
            // nothing. The count doing the deciding is the same one the row shows.
            tree.matchingLeaves(node.id, filter) > 0
        } else {
            filter.matches(
                kind = node.artifactKind,
                // Inherited: an artifact carries no purpose of its own, so asking it directly
                // would make every purpose facet match nothing.
                purpose = tree.purposeOf(node),
                state = null,
            )
        }
    }
    // Folders first and always A-Z; only the leaves take the chosen order. A rail whose
    // folders drifted around under a sort would lose the one thing its columns are for.
    //
    // `Recent` is the tree's own order: the sync frame carries no timestamp, so "recent" here
    // means position rather than time. Honest, and worth saying out loud.
    val (folders, leaves) = surviving.partition { it.isContainer }
    val ordered = folders.sortedBy { it.title.lowercase() } + when (filter.sort) {
        ItemSort.Name -> leaves.sortedBy { it.title.lowercase() }
        ItemSort.Recent -> leaves
    }
    return BrowserColumn(
        title = title,
        rows = ordered.map { node ->
            BrowserRow(
                id = node.id,
                title = node.title.orUntitled(),
                isContainer = node.kind != NodeKind.Artifact,
                kind = node.artifactKind,
                purpose = tree.purposeOf(node),
                state = null,
                selected = node.id == selectedId,
                count = if (node.isContainer) tree.matchingLeaves(node.id, filter) else null,
            )
        },
    )
}

/** What to call a node for a human. One place, so no surface invents a second answer. */
private fun WorkspaceNode.kindLabel(): String = when (kind) {
    NodeKind.Research -> "RESEARCH"
    NodeKind.Folder -> "FOLDER"
    NodeKind.Artifact -> when (artifactKind) {
        ArtifactKind.Page -> "NOTE"
        ArtifactKind.Link -> "LINK"
        ArtifactKind.App -> "APP"
        ArtifactKind.Voice -> "VOICE"
        ArtifactKind.File -> "FILE"
        null -> "ARTIFACT"
    }
}

private fun String?.orUntitled(): String = this?.takeIf { it.isNotBlank() } ?: "Untitled"
