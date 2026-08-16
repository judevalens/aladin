package dawn.system.anchor.features.shell.state

import androidx.compose.runtime.Composable
import com.slack.circuit.runtime.CircuitUiState
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.remember
import dawn.system.anchor.domain.Destination
import dawn.system.anchor.domain.PathLevel
import dawn.system.anchor.domain.SidebarNav
import dawn.system.anchor.domain.WorkspaceNode
import dawn.system.anchor.features.shell.ShellScreen
import dawn.system.anchor.services.data.NodeState
import dawn.system.anchor.services.data.NodeStore
import dawn.system.anchor.services.data.each
import dawn.system.anchor.services.data.eachChildren
import kotlinx.coroutines.flow.flowOf

/** Everything the sidebar and the column browser draw, for one [SidebarNav]. */
data class NavigationSlice(
    val rows: List<WorkspaceNode>,
    val contents: List<WorkspaceNode>,
    val displayed: WorkspaceNode?,
    val folder: WorkspaceNode?,
    val title: String,
    val count: String,
    val searchPlaceholder: String,
    val backLabel: String?,
    val pathLevels: List<PathLevel>,
    val columns: List<ShellScreen.BrowseColumn>,
    /** `Missing` means the folder you are inside was deleted — see the note below. */
    val folderState: NodeState,
    /** The same, for whatever the detail is showing. */
    val displayedState: NodeState,
) : CircuitUiState

/**
 * Reads the tree for one position in it.
 *
 * Every read here is a **query keyed on what it needs** — the row, the folder's children,
 * the path's rows — never a search through a materialised list. That is what lets the whole
 * slice be derived on every frame without anything caching a node: a rename reaches the
 * sidebar heading, Back's label, the path popover and the browser's column headers together,
 * because none of them is holding a copy.
 *
 * The one subtlety worth keeping in mind is [NodeState]: a stream has not read anything when
 * it is first asked, so "no node" and "deleted" have to stay distinguishable. Deciding
 * anything on the difference belongs to the caller, which is why both states are returned
 * rather than flattened to nulls here.
 */
class NavigationStateProducer(private val nodes: NodeStore) {

    @Composable
    operator fun invoke(nav: SidebarNav, query: String): NavigationSlice {
        // The section's rows: the tree's roots for Folders, one artifact type for Sources.
        val rows by remember(nav.destination) {
            when (nav.destination) {
                Destination.Folders -> nodes.children(null)
                Destination.Sources -> nodes.byArtifactType(FILE_ARTIFACT)
                else -> flowOf(emptyList())
            }
        }.collectAsState(initial = emptyList())

        // Two single-row subscriptions rather than two searches through a list. Each is keyed
        // on the id it reads, which is load-bearing rather than tidy — see [readOf].
        val displayedState = nodes.readOf(nav.currentId)
        val folderState = nodes.readOf(nav.folderId)

        // The drill path's own rows — every label the navigation draws reads from these.
        // Deliberately NOT used to decide whether a folder still exists: a retained stream starts
        // unread, so a path that is merely unread looks identical to one that was deleted, and an
        // effect pruning on that reverted every drill the frame after it happened.
        val pathNodes by remember(nav.drill) { nodes.each(nav.drill) }
            .collectAsState(initial = null)

        val children by remember(nav.folderId) {
            nav.folderId?.let(nodes::children) ?: flowOf(emptyList())
        }.collectAsState(initial = emptyList())

        // Every column's contents in ONE subscription per folder rather than a query per column —
        // the depth of the browser should not decide how much is watching the database.
        val columnRows by remember(nav.drill) { nodes.eachChildren(nav.drill) }
            .collectAsState(initial = emptyList())

        val titleOf: (String) -> String = { id ->
            pathNodes?.firstOrNull { it.id == id }?.title?.ifBlank { UNTITLED } ?: UNTITLED
        }

        val visibleRows = remember(rows, query) { rows.filter { it.matches(query) } }
        val contents = remember(children, query) { children.filter { it.matches(query) } }
        val sectionTitle = nav.destination?.title

        val columns = remember(visibleRows, columnRows, nav, pathNodes) {
            val destination = nav.destination ?: return@remember emptyList()
            val byParent = columnRows.groupBy { it.parentId }
            buildList {
                add(
                    ShellScreen.BrowseColumn(
                        title = destination.title,
                        items = visibleRows,
                        selectedId = nav.drill.firstOrNull() ?: nav.selectedId,
                    ),
                )
                nav.drill.forEachIndexed { index, folderId ->
                    add(
                        ShellScreen.BrowseColumn(
                            title = titleOf(folderId),
                            items = byParent[folderId].orEmpty(),
                            selectedId = nav.drill.getOrNull(index + 1) ?: nav.selectedId,
                        ),
                    )
                }
            }
        }

        return NavigationSlice(
            rows = visibleRows,
            contents = contents,
            displayed = displayedState.node,
            folder = folderState.node,
            title = when (nav.level) {
                SidebarNav.Level.Root -> SidebarNav.ROOT_TITLE
                SidebarNav.Level.Section -> sectionTitle.orEmpty()
                SidebarNav.Level.Drill -> titleOf(nav.drill.last())
            },
            count = when (nav.level) {
                SidebarNav.Level.Root -> ""
                SidebarNav.Level.Section -> "${visibleRows.size} items"
                SidebarNav.Level.Drill -> "${contents.size} items"
            },
            searchPlaceholder = when (nav.level) {
                SidebarNav.Level.Root -> "Search everything"
                SidebarNav.Level.Section -> "Search ${sectionTitle?.lowercase().orEmpty()}"
                SidebarNav.Level.Drill -> "Search this folder"
            },
            backLabel = nav.backLabel(sectionTitle, titleOf),
            pathLevels = nav.levels(
                sectionTitle = sectionTitle,
                titleOf = titleOf,
                purposeOf = { id -> pathNodes?.firstOrNull { it.id == id }?.kind?.wire ?: "folder" },
            ),
            columns = columns,
            folderState = folderState,
            displayedState = displayedState,
        )
    }
}

/**
 * One node's read, for exactly [id].
 *
 * The `key` is not optional, and not tidiness. `collectAsState` is `produceState`, whose
 * backing `remember { mutableStateOf(initial) }` has **no keys** — so when the flow changes
 * the state keeps the *previous* flow's value and `initial` never applies again. Combined
 * with the `?: flowOf(Missing)` branch below, that meant a genuine `Missing` read at section
 * level (where [SidebarNav.folderId] is null) survived into the frame after a drill and was
 * attributed to the folder just entered: [dawn.system.anchor.domain.corrected] saw
 * [dawn.system.anchor.domain.Presence.Gone] and called `back()`, reverting every drill.
 *
 * `key` gives the whole group a new identity, so the reset to [NodeState.Loading] is real.
 */
@Composable
private fun NodeStore.readOf(id: String?): NodeState = key(id) {
    val state by remember { id?.let { node(it) } ?: flowOf(NodeState.Missing) }
        .collectAsState(initial = NodeState.Loading)
    state
}

/** The search field narrows on title — the only text a row reliably shows. */
private fun WorkspaceNode.matches(query: String): Boolean =
    query.isBlank() || title.contains(query.trim(), ignoreCase = true)

private const val FILE_ARTIFACT = "file"
private const val UNTITLED = "Untitled"
