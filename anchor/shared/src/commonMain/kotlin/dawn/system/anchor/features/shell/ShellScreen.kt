package dawn.system.anchor.features.shell

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
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
import dawn.system.anchor.domain.PathLevel
import dawn.system.anchor.domain.Presence
import dawn.system.anchor.domain.corrected
import dawn.system.anchor.domain.SidebarNav
import dawn.system.anchor.domain.SurfaceKind
import dawn.system.anchor.domain.SurfaceSlot
import dawn.system.anchor.domain.indexOfNode
import dawn.system.anchor.domain.slotsFor
import dawn.system.anchor.domain.surfaceKind
import dawn.system.anchor.domain.WorkspaceNode
import dawn.system.anchor.features.note.WebPane
import dawn.system.anchor.features.shell.state.NavigationStateProducer
import dawn.system.anchor.features.shell.state.SurfaceStateProducer
import dawn.system.anchor.features.shell.state.WriteStateProducer
import dawn.system.anchor.features.shell.state.rememberChromeState
import dawn.system.anchor.domain.sectionsFor
import dawn.system.anchor.services.auth.SessionManager
import dawn.system.anchor.services.data.ArtifactResourceStore
import dawn.system.anchor.services.data.DocumentStore
import dawn.system.anchor.services.data.NodeState
import dawn.system.anchor.services.data.NodeStore
import dawn.system.anchor.services.data.eachChildren
import dawn.system.anchor.services.data.each
import dawn.system.anchor.services.data.WorkspaceWriter
import dawn.system.anchor.services.di.CommonParcelize
import dawn.system.anchor.services.network.ApiConfig
import dawn.system.anchor.services.network.TokenProvider
import dawn.system.anchor.services.di.bindScreen
import dawn.system.anchor.services.sync.SyncRunner
import dawn.system.anchor.services.sync.SyncStatus
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.launch
import org.koin.core.module.Module
import org.koin.dsl.module

/**
 * The no-rail shell (PRD rev 2): sidebar · detail, plus a conditional copilot pane.
 *
 * The presenter owns *shell* state only — where the sidebar is in its stack, what is open,
 * whether the copilot pane is showing. Everything a row or a detail knows comes from the
 * store, and anything a specific surface needs (a reader's scroll, a canvas's tool) belongs
 * to that surface. That boundary is what stops this becoming the prototype's single
 * mutable blob.
 */
@CommonParcelize
data object ShellScreen : Screen {
    data class State(
        val nav: SidebarNav,
        val sidebarOpen: Boolean,
        /** What the sidebar list renders right now. */
        val level: SidebarNav.Level,
        /** Root level: the sections menu. */
        val destinations: List<Destination>,
        /** Section level: that destination's rows, narrowed by [query]. */
        val rows: List<WorkspaceNode>,
        /** Drill level: the contents of the folder you are inside, narrowed by [query]. */
        val contents: List<WorkspaceNode>,
        val sidebarTitle: String,
        /** "12 items" — empty at root, where a count would mean nothing. */
        val sidebarCount: String,
        val searchPlaceholder: String,
        val query: String,
        /** Back's destination, for the tooltip. Null only at root. */
        val backLabel: String?,
        val pathOpen: Boolean,
        val pathLevels: List<PathLevel>,
        /**
         * The column browser, root-most first: the section's rows, then one column per
         * folder descended into. Empty at root, where the sidebar *is* the sections list.
         */
        val browseColumns: List<BrowseColumn>,
        val browseOpen: Boolean,
        /** The node the detail pane shows. */
        val displayed: WorkspaceNode?,
        val openItems: OpenItems,
        /**
         * The switcher's rows, resolved from the tree on every frame — so renaming a note
         * renames its tab, and a node that vanished stops being offered.
         */
        val openTabs: List<OpenTab>,
        val switcherOpen: Boolean,
        val heroExpanded: Boolean,
        val copilotOpen: Boolean,
        val sync: SyncStatus,
        val signedInAs: String?,
        /**
         * The detail pager's pages, one per open thing — except that every web-backed
         * artifact shares the first page, because they share one web view.
         */
        val pages: List<Page>,
        /** Which page is showing. -1 when nothing is open. */
        val activePage: Int,
        /** The single web host and everything open inside it. */
        val webHost: WebHostState,
        /** The node a long press is acting on — the rename/delete sheet's subject. */
        val actionsFor: WorkspaceNode?,
        /** An in-flight rename. Non-null exactly while its dialog is up. */
        val rename: RenameDraft?,
        /** A delete waiting on confirmation, because it takes the subtree with it. */
        val pendingDelete: WorkspaceNode?,
        val createOpen: Boolean,
        /** Where a new item would land, spelled out so "here" is never ambiguous. */
        val createTarget: String,
        /** A write the server refused. Shown until dismissed; never swallowed. */
        val writeError: String?,
        val eventSink: (Event) -> Unit,
    ) : CircuitUiState

    /** One page of the detail pager. */
    sealed interface Page {
        /** The shared web view — whichever page or shard is active inside it. */
        data object Web : Page

        /** An artifact with a surface of its own: a PDF, a voice note, a link. */
        data class Artifact(val surface: ArtifactSurface) : Page

        /**
         * A folder or research folder, rendered as a detail — with its OWN sections, so a
         * page that is not the current selection still draws the right thing.
         */
        data class Detail(val node: WorkspaceNode, val sections: List<DetailSection>) : Page

        /** Open, but not read yet. Holds its place so the pager does not collapse. */
        data class Pending(val nodeId: String) : Page
    }

    /** One switcher row: identity from the open list, everything shown from the tree. */
    data class OpenTab(
        val key: String,
        val nodeId: String,
        val title: String,
        /** The folder that owns it, or the destination's name — the grouping heading. */
        val owner: String,
        val meta: String?,
        val active: Boolean,
    )

    /** One column of the browser: a folder's contents, and which of them is on the path. */
    data class BrowseColumn(
        val title: String,
        val items: List<WorkspaceNode>,
        val selectedId: String?,
    )

    data class RenameDraft(val node: WorkspaceNode, val title: String, val saving: Boolean = false)

    /**
     * One artifact rendered in the detail pane. The kind decides the surface; the payload
     * is whatever that surface needs, resolved here so the UI stays a renderer.
     */
    data class ArtifactSurface(
        val node: WorkspaceNode,
        val kind: Kind,
        val resources: ArtifactResourceStore,
        /** The ingested half of a file artifact: status, page count, outline. */
        val documents: DocumentStore,
    ) {
        enum class Kind { Document, Voice, Link, Other }
    }

    /**
     * What the one web host needs: the session, and every web-backed artifact that should
     * stay open inside it. There is no per-artifact web target any more — that shape was
     * what made each note cost its own web view.
     */
    data class WebHostState(
        val panes: List<WebPane>,
        val activeId: String?,
        val token: String?,
        val userName: String?,
        val api: ApiConfig,
    )

    /**
     * Grouped by the producer that owns each event.
     *
     * The nesting is what lets a producer take *only* its own events, so its handler is
     * exhaustive and the compiler catches an unrouted one. A flat hierarchy needed an
     * `else -> false` catch-all in every handler, which is a silent hole: a new event
     * routed nowhere would compile and simply do nothing.
     */
    sealed interface Event {

        /** Panels, popovers and sheets. Nothing outside the shell can tell whether they are open. */
        sealed interface Chrome : Event {
            data object ToggleSidebar : Chrome
            data object TogglePath : Chrome
            data object ToggleSwitcher : Chrome
            data object ToggleBrowse : Chrome
            data object ToggleCreate : Chrome
            data object ToggleHero : Chrome
            data object OpenCopilot : Chrome
            data object CloseCopilot : Chrome
        }

        /** Create, rename, delete — everything that changes the workspace. */
        sealed interface Write : Event {
            data class ActionsRequested(val node: WorkspaceNode) : Write
            data object ActionsDismissed : Write
            data object RenameStarted : Write
            data class RenameEdited(val title: String) : Write
            data object RenameCommitted : Write
            data object RenameCancelled : Write
            data object DeleteRequested : Write
            data object DeleteConfirmed : Write
            data object DeleteCancelled : Write
            data object CreateFolder : Write
            data object CreatePage : Write
            data object WriteErrorDismissed : Write
        }

        /** Where the sidebar is, and what it is showing. */
        sealed interface Nav : Event {
            data class SectionOpened(val destination: Destination) : Nav
            data class RowSelected(val node: WorkspaceNode) : Nav
            data class DrilledInto(val node: WorkspaceNode) : Nav
            data object Back : Nav
            data class QueryChanged(val query: String) : Nav
            data class JumpToLevel(val index: Int) : Nav
            data class BrowsePicked(val depth: Int, val node: WorkspaceNode) : Nav
        }

        /** The open set: what you can get back to in one tap. */
        sealed interface Open : Event {
            data class ActivateOpenItem(val key: String) : Open
            data class CloseOpenItem(val key: String) : Open
            data class StepOpenItem(val delta: Int) : Open
        }

        /** The session itself. */
        sealed interface Session : Event {
            data object SignOut : Session
        }
    }
}

/** Where an open item was, so activating it lands you back there rather than at the top. */
private data class ViewPosition(val nav: SidebarNav)

private data class RenameInput(val nodeId: String, val title: String, val saving: Boolean = false)

class ShellPresenter(
    // The state producers. Each owns the services it reads, so this presenter no longer
    // touches the store, the writer or the resource caches directly — it composes slices and
    // holds the two pieces of state more than one of them writes.
    private val navigation: NavigationStateProducer,
    private val surfaces: SurfaceStateProducer,
    private val writes: WriteStateProducer,
    private val session: SessionManager,
    private val sync: SyncRunner,
    // The note editor is the desktop editor in a web view; it needs the bearer to join
    // the collab session, and the endpoints to load from.
    private val tokens: TokenProvider,
    private val api: ApiConfig,
) : Presenter<ShellScreen.State> {

    @Composable
    override fun present(): ShellScreen.State {
        // The sidebar stack is the whole navigation model; nothing else tracks "where".
        var nav by remember { mutableStateOf(SidebarNav()) }
        var openItems by remember { mutableStateOf(OpenItems()) }
        var query by remember { mutableStateOf("") }
        val positionByOpenItem = remember { mutableStateMapOf<String, ViewPosition>() }
        val scope = rememberCoroutineScope()

        // Panels, popovers and sheets: local to the shell, and nothing outside it can tell
        // whether they are open.
        val chrome = rememberChromeState(isRoot = nav.isRoot)

        val syncStatus by sync.status.collectAsState()
        val user by session.state.collectAsState()

        // The tree, read for where we are. Every value here is a query keyed on what it
        // needs, so nothing caches a node and a rename reaches every label at once.
        val navSlice = navigation(nav, query)
        val displayed = navSlice.displayed
        val folder = navSlice.folder

        // One rule, not three effects racing each other. `Presence` keeps "not read yet"
        // distinct from "gone" — the distinction that reverted every drill when it was
        // missing. Tested exhaustively in NavCorrectionTest.
        LaunchedEffect(nav, openItems, navSlice.folderState, navSlice.displayedState) {
            val corrected = nav.corrected(
                folder = navSlice.folderState.presence(),
                displayed = navSlice.displayedState.presence(),
                isOpen = { id -> openItems.items.any { it.nodeId == id } },
            )
            if (corrected != nav) nav = corrected
        }

        // What is open, as pages, tabs and web panes.
        // The ACTIVE ID, not the resolved node. Which item is active is known the instant it
        // is tapped; its row arrives afterwards. Feeding the pager the slower of the two left
        // it sitting on the previous page until the read landed.
        val surfaceSlice = surfaces(openItems, nav.currentId)

        fun register(node: WorkspaceNode, at: SidebarNav) {
            val destination = at.destination ?: return
            val key = "${destination.id}:${node.id}"
            openItems = openItems.register(
                OpenItem(key = key, destination = destination, nodeId = node.id),
            )
            positionByOpenItem[key] = ViewPosition(at)
        }

        // Create / rename / delete. Reaches back into navigation and the open set through
        // callbacks rather than by reading them, so the direction of the dependency is
        // visible here rather than buried in the producer.
        val writeSlice = writes(
            parentId = nav.folderId,
            onCreated = { node ->
                nav = nav.select(node)
                register(node, nav)
            },
            onDeleted = { target ->
                openItems.items.filter { it.nodeId == target.id }
                    .forEach { openItems = openItems.close(it.key) }
                if (nav.currentId == target.id || nav.folderId == target.id) nav = nav.back()
            },
            onCreateChosen = chrome.dismissOverlays,
        )

        return ShellScreen.State(
            nav = nav,
            sidebarOpen = chrome.sidebarOpen,
            level = nav.level,
            destinations = Destination.entries,
            rows = navSlice.rows,
            contents = navSlice.contents,
            sidebarTitle = navSlice.title,
            sidebarCount = navSlice.count,
            searchPlaceholder = navSlice.searchPlaceholder,
            query = query,
            backLabel = navSlice.backLabel,
            pathOpen = chrome.pathOpen,
            pathLevels = navSlice.pathLevels,
            browseColumns = navSlice.columns,
            browseOpen = chrome.browseOpen,
            displayed = displayed,
            openItems = openItems,
            openTabs = surfaceSlice.tabs,
            switcherOpen = chrome.switcherOpen,
            heroExpanded = chrome.heroExpanded,
            copilotOpen = chrome.copilotOpen,
            sync = syncStatus,
            signedInAs = (user as? dawn.system.anchor.services.auth.AuthState.LoggedIn)?.user?.email,
            pages = surfaceSlice.pages,
            activePage = surfaceSlice.activePage,
            webHost = ShellScreen.WebHostState(
                panes = surfaceSlice.webPanes,
                activeId = displayed?.id?.takeIf { id -> surfaceSlice.webPanes.any { it.id == id } },
                token = tokens.currentToken(),
                userName = (user as? dawn.system.anchor.services.auth.AuthState.LoggedIn)?.user?.email,
                api = api,
            ),
            actionsFor = writeSlice.actionsFor,
            // The draft text is genuinely local — it is what the user is typing. The node it
            // renames is not, so it is resolved rather than carried.
            rename = writeSlice.rename,
            pendingDelete = writeSlice.pendingDelete,
            createOpen = chrome.createOpen,
            createTarget = folder?.title?.ifBlank { "Untitled" }
                ?: nav.destination?.title
                ?: SidebarNav.ROOT_TITLE,
            writeError = writeSlice.writeError,
        ) { event ->
            // Routed by type. Navigation and the open set stay here because more than one
            // producer writes them; everything else belongs to the producer that owns it.
            when (event) {
                is ShellScreen.Event.Chrome -> chrome.handle(event)
                is ShellScreen.Event.Write -> writeSlice.handle(event)
                is ShellScreen.Event.Nav.SectionOpened -> {
                    nav = nav.openSection(event.destination)
                    query = ""
                    chrome.unpinHero()
                    chrome.dismissOverlays()
                }

                is ShellScreen.Event.Nav.RowSelected -> {
                    nav = nav.select(event.node)
                    query = ""
                    register(event.node, nav)
                }

                is ShellScreen.Event.Nav.DrilledInto -> {
                    nav = nav.drillInto(event.node.id)
                    query = ""
                    register(event.node, nav)
                }

                ShellScreen.Event.Nav.Back -> {
                    nav = nav.back()
                    query = ""
                    if (nav.isRoot) chrome.unpinHero()
                }

                is ShellScreen.Event.Nav.QueryChanged -> query = event.query

                is ShellScreen.Event.Nav.JumpToLevel -> {
                    nav = nav.jumpTo(event.index)
                    query = ""
                    chrome.dismissOverlays()
                }

                is ShellScreen.Event.Nav.BrowsePicked -> {
                    // Truncate the path back to this column, then take the step. A folder
                    // opens the next column and the browser stays up; an artifact is a
                    // destination, so the browser has done its job and gets out of the way.
                    nav = nav.jumpTo(event.depth + 1).select(event.node)
                    query = ""
                    register(event.node, nav)
                    if (!event.node.isContainer) chrome.dismissOverlays()
                }

                is ShellScreen.Event.Open.ActivateOpenItem -> {
                    openItems = openItems.activate(event.key)
                    positionByOpenItem[event.key]?.let { nav = it.nav }
                    chrome.dismissOverlays()
                    query = ""
                }

                is ShellScreen.Event.Open.CloseOpenItem -> {
                    // Closing removes the page; the pager disposes its surface as a
                    // consequence. Nothing to unmount by hand any more.
                    openItems = openItems.close(event.key)
                    positionByOpenItem.remove(event.key)
                    // Follow whatever took its place. Closing the *last* item leaves nothing
                    // to follow, which the effect above handles by dropping the selection —
                    // the node still exists, so nothing else would stop the header rendering
                    // the title of the thing you just closed.
                    openItems.active?.let { item ->
                        positionByOpenItem[item.key]?.let { nav = it.nav }
                    }
                }

                is ShellScreen.Event.Open.StepOpenItem -> {
                    openItems = openItems.step(event.delta)
                    openItems.active?.let { item ->
                        positionByOpenItem[item.key]?.let { nav = it.nav }
                    }
                    query = ""
                }

                ShellScreen.Event.Session.SignOut -> scope.launch {
                    sync.stopAndClear()
                    session.logout()
                }
            }
        }
    }

    private companion object {
        /** `artifact_type` for a BlockNote page — what the note editor opens. */
        const val PAGE_ARTIFACT = "page"
        /** `artifact_type` for an uploaded file — what the reader opens. */
        const val FILE_ARTIFACT = "file"
        const val VOICE_ARTIFACT = "voice"
        const val LINK_ARTIFACT = "link"
        /** `artifact_type` for a shard — an agent-authored app, also web-backed. */
        const val APP_ARTIFACT = "app"
        const val UNTITLED = "Untitled"
    }
}

/** The store's read, in the vocabulary the navigation rule speaks. */
private fun NodeState.presence(): Presence = when (this) {
    NodeState.Loading -> Presence.Unknown
    NodeState.Missing -> Presence.Gone
    is NodeState.Present -> Presence.There
}

/**
 * Which tree rows a section shows. Folders is the browser — folders and artifacts as
 * siblings in one list, exactly as desktop presents them. Sources narrows to ingested
 * files. The rest have no tree-backed list yet and say so rather than inventing one.
 */
private fun rowsFor(destination: Destination, node: WorkspaceNode): Boolean =
    when (destination) {
        Destination.Folders -> node.parentId == null
        Destination.Sources -> node.artifactType == "file"
        else -> false
    }

/**
 * What to tell the user when a write is refused. The server's own message when there is
 * one, because "couldn't rename" alone leaves nothing to act on.
 */
private fun Throwable.failureText(action: String): String =
    message?.takeIf { it.isNotBlank() } ?: "the $action didn't go through"

/** The search field narrows on title — the only text a row reliably shows. */
private fun WorkspaceNode.matches(query: String): Boolean =
    query.isBlank() || title.contains(query.trim(), ignoreCase = true)

private fun ownerLabel(
    node: WorkspaceNode,
    owners: List<WorkspaceNode>,
    destination: Destination,
): String = owners.firstOrNull { it.id == node.parentId }
    ?.title?.takeIf { it.isNotBlank() }
    ?: destination.title

val shellModule: Module = module {
    // Each producer owns the services it reads; the presenter composes their slices.
    single { NavigationStateProducer(get()) }
    single { SurfaceStateProducer(get(), get(), get()) }
    single { WriteStateProducer(get(), get()) }

    bindScreen(
        ShellScreen::class,
        presenter = { _, _ ->
            ShellPresenter(get(), get(), get(), get(), get(), get(), get())
        },
        uiFactory = { ui<ShellScreen.State> { state, modifier -> ShellUi(state, modifier) } },
    )
}
