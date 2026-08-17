package dawn.system.anchor.features.shell.state

import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import com.slack.circuit.runtime.CircuitUiState
import dawn.system.anchor.domain.ArtifactKind
import dawn.system.anchor.domain.BrowserFilter
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
)

/** One column: a folder's contents, and which of them is on the path. */
data class BrowserColumn(val title: String, val rows: List<BrowserRow>)

/** The third pane — what one selected item is, and what opening it would do. */
data class BrowserDetail(
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

        val columns = remember(roots, byParent, pathRows, folderPath, here, filter) {
            buildList {
                add(
                    column(
                        title = "Folders",
                        contents = roots,
                        // Column i's selection is simply `path[i]` now that the leaf lives in
                        // the same array — no fallback, because there is no second field to
                        // fall back to.
                        selectedId = here.path.firstOrNull(),
                        filter = filter,
                    ),
                )
                folderPath.forEachIndexed { depth, folderId ->
                    add(
                        column(
                            title = pathRows.firstOrNull { it.id == folderId }?.title.orUntitled(),
                            contents = byParent[folderId].orEmpty(),
                            selectedId = here.path.getOrNull(depth + 1),
                            filter = filter,
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
): BrowserColumn {
    val surviving = contents.filter { node ->
        filter.matches(
            kind = node.artifactKind,
            purpose = FolderPurpose.of(node),
            state = null,
        )
    }
    val ordered = when (filter.sort) {
        ItemSort.Name -> surviving.sortedBy { it.title.lowercase() }
        ItemSort.Recent -> surviving
    }
    return BrowserColumn(
        title = title,
        rows = ordered.map { node ->
            BrowserRow(
                id = node.id,
                title = node.title.orUntitled(),
                isContainer = node.kind != NodeKind.Artifact,
                kind = node.artifactKind,
                purpose = FolderPurpose.of(node),
                state = null,
                selected = node.id == selectedId,
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
