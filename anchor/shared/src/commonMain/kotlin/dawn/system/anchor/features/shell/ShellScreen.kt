package dawn.system.anchor.features.shell

import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import com.slack.circuit.runtime.CircuitUiState
import com.slack.circuit.runtime.presenter.Presenter
import com.slack.circuit.runtime.screen.Screen
import com.slack.circuit.runtime.ui.ui
import dawn.system.anchor.domain.DetailSection
import dawn.system.anchor.domain.Destination
import dawn.system.anchor.domain.OpenItem
import dawn.system.anchor.domain.OpenItems
import dawn.system.anchor.domain.WorkspaceNode
import dawn.system.anchor.domain.sectionsFor
import dawn.system.anchor.services.auth.SessionManager
import dawn.system.anchor.services.data.NodeStore
import dawn.system.anchor.services.di.CommonParcelize
import dawn.system.anchor.services.di.bindScreen
import dawn.system.anchor.services.sync.SyncRunner
import dawn.system.anchor.services.sync.SyncStatus
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.launch
import org.koin.core.module.Module
import org.koin.dsl.module

/**
 * The three-column shell: rail · collapsible list · detail.
 *
 * The presenter owns *shell* state only — which destination, what is selected in each,
 * which items are open. Everything a row or a detail knows comes from the store, and
 * anything a specific surface needs (a reader's scroll, a canvas's tool) belongs to that
 * surface, not here. That boundary is what stops this becoming the prototype's single
 * mutable blob.
 */
@CommonParcelize
data object ShellScreen : Screen {
    data class State(
        val destination: Destination,
        val listOpen: Boolean,
        val rows: List<WorkspaceNode>,
        val selected: WorkspaceNode?,
        /** The selected row's detail, composed from the shared section vocabulary. */
        val sections: List<DetailSection>,
        val openItems: OpenItems,
        val switcherOpen: Boolean,
        val sync: SyncStatus,
        val signedInAs: String?,
        val eventSink: (Event) -> Unit,
    ) : CircuitUiState

    sealed interface Event {
        data class DestinationSelected(val destination: Destination) : Event
        data class RowSelected(val node: WorkspaceNode) : Event
        data object ToggleList : Event
        data object ToggleSwitcher : Event
        data class ActivateOpenItem(val key: String) : Event
        data class CloseOpenItem(val key: String) : Event
        data class StepOpenItem(val delta: Int) : Event
        data object SignOut : Event
    }
}

class ShellPresenter(
    private val nodes: NodeStore,
    private val session: SessionManager,
    private val sync: SyncRunner,
) : Presenter<ShellScreen.State> {

    @Composable
    override fun present(): ShellScreen.State {
        var destination by remember { mutableStateOf(Destination.default) }
        var listOpen by remember { mutableStateOf(true) }
        var switcherOpen by remember { mutableStateOf(false) }
        var openItems by remember { mutableStateOf(OpenItems()) }
        // Selection is remembered per destination, so switching away and back returns you
        // to what you were looking at rather than resetting the pane.
        val selectionByDestination = remember { mutableStateMapOf<Destination, String>() }
        val scope = rememberCoroutineScope()

        val rows by remember(destination) {
            nodes.liveNodes().map { all -> all.filter { rowsFor(destination, it) } }
        }.collectAsState(initial = emptyList())

        val syncStatus by sync.status.collectAsState()
        val user by session.state.collectAsState()

        val selectedId = selectionByDestination[destination]
        val selected = rows.firstOrNull { it.id == selectedId }

        // A container's detail is largely its contents, so the children ride the same
        // reactive path as the list — a frame landing updates both without a refetch.
        val children by remember(selected?.id) {
            selected?.let { nodes.children(it.id) } ?: flowOf(emptyList())
        }.collectAsState(initial = emptyList())

        val sections = remember(selected, children) {
            selected?.let { sectionsFor(it, children) }.orEmpty()
        }

        return ShellScreen.State(
            destination = destination,
            listOpen = listOpen,
            rows = rows,
            selected = selected,
            sections = sections,
            openItems = openItems,
            switcherOpen = switcherOpen,
            sync = syncStatus,
            signedInAs = (user as? dawn.system.anchor.services.auth.AuthState.LoggedIn)?.user?.email,
        ) { event ->
            when (event) {
                is ShellScreen.Event.DestinationSelected -> {
                    destination = event.destination
                    switcherOpen = false
                }

                is ShellScreen.Event.RowSelected -> {
                    selectionByDestination[destination] = event.node.id
                    // Selecting registers an open item — "open" is a place you return to,
                    // not a tab you had to create deliberately.
                    openItems = openItems.register(
                        OpenItem(
                            key = "${destination.id}:${event.node.id}",
                            destination = destination,
                            nodeId = event.node.id,
                            title = event.node.title,
                            owner = ownerLabel(event.node, rows, destination),
                            meta = event.node.artifactType,
                        ),
                    )
                }

                ShellScreen.Event.ToggleList -> listOpen = !listOpen
                ShellScreen.Event.ToggleSwitcher -> switcherOpen = !switcherOpen

                is ShellScreen.Event.ActivateOpenItem -> {
                    openItems = openItems.activate(event.key)
                    openItems.active?.let { item ->
                        destination = item.destination
                        selectionByDestination[item.destination] = item.nodeId
                    }
                    switcherOpen = false
                }

                is ShellScreen.Event.CloseOpenItem -> openItems = openItems.close(event.key)

                is ShellScreen.Event.StepOpenItem -> {
                    openItems = openItems.step(event.delta)
                    openItems.active?.let { item ->
                        destination = item.destination
                        selectionByDestination[item.destination] = item.nodeId
                    }
                }

                ShellScreen.Event.SignOut -> scope.launch {
                    sync.stopAndClear()
                    session.logout()
                }
            }
        }
    }
}

/**
 * Which tree rows a destination shows. Folders is the browser — folders and artifacts as
 * siblings in one list, exactly as desktop presents them. Sources narrows to ingested
 * files. The rest have no tree-backed list yet and say so rather than inventing one.
 */
private fun rowsFor(destination: Destination, node: WorkspaceNode): Boolean =
    when (destination) {
        Destination.Folders -> node.parentId == null
        Destination.Sources -> node.artifactType == "file"
        else -> false
    }

private fun ownerLabel(
    node: WorkspaceNode,
    rows: List<WorkspaceNode>,
    destination: Destination,
): String = node.parentId
    ?.let { parentId -> rows.firstOrNull { it.id == parentId }?.title }
    ?: destination.title

val shellModule: Module = module {
    bindScreen(
        ShellScreen::class,
        presenter = { _, _ -> ShellPresenter(get(), get(), get()) },
        uiFactory = { ui<ShellScreen.State> { state, modifier -> ShellUi(state, modifier) } },
    )
}
