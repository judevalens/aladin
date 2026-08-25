package dawn.system.anchor.features.shell

import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.rememberCoroutineScope
import com.slack.circuit.runtime.CircuitUiState
import com.slack.circuit.runtime.presenter.Presenter
import com.slack.circuit.runtime.screen.Screen
import com.slack.circuit.runtime.ui.ui
import dawn.system.anchor.domain.BrowserFilter
import dawn.system.anchor.domain.Destination
import dawn.system.anchor.features.shell.state.BrowserSlice
import dawn.system.anchor.features.shell.state.BrowserStateProducer
import dawn.system.anchor.features.shell.state.ChromeSlice
import dawn.system.anchor.features.shell.state.Crumb
import dawn.system.anchor.features.shell.state.DocumentSlice
import dawn.system.anchor.features.shell.state.DocumentStateProducer
import dawn.system.anchor.features.shell.state.NavSlice
import dawn.system.anchor.features.shell.state.NavStateProducer
import dawn.system.anchor.features.shell.state.OpenRow
import dawn.system.anchor.features.shell.state.NavEvent
import dawn.system.anchor.features.shell.state.WriteSlice
import dawn.system.anchor.features.shell.state.TreeSlice
import dawn.system.anchor.features.shell.state.TreeStateProducer
import dawn.system.anchor.features.shell.state.WriteStateProducer
import dawn.system.anchor.domain.OpenTab
import dawn.system.anchor.domain.TabKey
import dawn.system.anchor.features.shell.state.ChromeEvent
import dawn.system.anchor.features.shell.state.crumbsFor
import dawn.system.anchor.features.shell.state.openRowsFor
import dawn.system.anchor.features.shell.state.rememberChromeState
import dawn.system.anchor.features.shell.ui.ShellUi
import dawn.system.anchor.features.note.EditorSession
import dawn.system.anchor.services.auth.AuthState
import dawn.system.anchor.services.auth.SessionManager
import dawn.system.anchor.services.data.NodeStore
import dawn.system.anchor.services.di.CommonParcelize
import dawn.system.anchor.services.di.bindScreen
import dawn.system.anchor.services.sync.SyncRunner
import dawn.system.anchor.services.sync.SyncStatus
import dawn.system.anchor.services.network.ApiConfig
import dawn.system.anchor.services.network.TokenProvider
import kotlinx.coroutines.launch
import org.koin.core.module.Module
import org.koin.dsl.module

/**
 * The shell: one stable sidebar, a breadcrumb that carries place, and a content surface that
 * swaps.
 *
 * The previous shell pushed and popped a sidebar and hid "where am I" behind a popover. The
 * handoff names both as the failure — *"no sense of place"*, and *"desktop shrunk down"* — so
 * the sidebar never changes meaning, and each surface owns its own shape.
 *
 * **The state is slices, not a blob.** Its predecessor flattened everything into 29 fields,
 * which is why one file rendered every surface. Each slice here carries its own handler, so
 * there is no central event sink to route through and no `else` branch to hide an unrouted
 * event.
 */
@CommonParcelize
data object ShellScreen : Screen {

    data class State(
        val nav: NavSlice,
        val chrome: ChromeSlice,
        /** The open documents, in append order — the tab strip. Resolved rows — the UI never sees a node. */
        val open: List<OpenRow>,
        /** The sidebar's folder tree, and where a new item will land. */
        val tree: TreeSlice,
        /** The whole sense of place. */
        val crumbs: List<Crumb>,
        /** The Miller columns, when Browser is the surface. */
        val browser: BrowserSlice,
        /** Every open document, each of which stays composed. */
        val documents: DocumentSlice,
        /** What the embedded editor needs to join a document; null until signed in. */
        val editor: EditorSession?,
        /** Rename, delete and create — the sheets the browser's organize footer raises. */
        val write: WriteSlice,
        val destinations: List<Destination>,
        val sync: SyncStatus,
        val signedInAs: String?,
        val onSignOut: () -> Unit,
    ) : CircuitUiState
}

class ShellPresenter(
    private val navigation: NavStateProducer,
    private val browser: BrowserStateProducer,
    private val documents: DocumentStateProducer,
    private val nodes: NodeStore,
    private val session: SessionManager,
    private val sync: SyncRunner,
    private val tokens: TokenProvider,
    private val config: ApiConfig,
    private val writes: WriteStateProducer,
    private val tree: TreeStateProducer,
) : Presenter<ShellScreen.State> {

    @Composable
    override fun present(): ShellScreen.State {
        val scope = rememberCoroutineScope()
        val chrome = rememberChromeState()
        val nav = navigation()

        val syncStatus by sync.status.collectAsState()
        val user by session.state.collectAsState()

        // Services become values here, which is the whole reason this layer exists: the UI
        // gets a token and a URL rather than a TokenProvider and an ApiConfig it could call.
        // Re-read on every auth change, so signing in reaches an editor that was never built;
        // a refresh mid-session does not, deliberately — see rememberWebSurfaceHost.
        val editor = (user as? AuthState.LoggedIn)?.let { signedIn ->
            tokens.currentToken()?.let { token ->
                EditorSession(
                    token = token,
                    collabWsUrl = config.collabWsUrl,
                    boardSyncWsUrl = config.boardSyncWsUrl,
                    apiBaseUrl = config.baseUrl,
                    userName = signedIn.user?.email,
                )
            }
        }

        val treeSlice = tree(nav.nav, chrome.filter)

        // New items land where the tree says — its focused folder, else the open document's
        // folder, else the Browser's. Deleting one closes it — the workspace destroying a row
        // and the user closing its tab are the same ending, and `Nav.corrected` would reach the
        // same place a frame later anyway.
        val write = writes(
            parentId = treeSlice.createTarget.folderId,
            onCreated = { nav.handle(NavEvent.OpenDoc(TabKey.Artifact(it.id))) },
            onDeleted = { nav.handle(NavEvent.Close(OpenTab.Doc(TabKey.Artifact(it.id)))) },
            onCreateChosen = { chrome.handle(ChromeEvent.CloseCreate) },
        )

        return ShellScreen.State(
            nav = nav,
            chrome = chrome,
            open = nodes.openRowsFor(nav.nav, chrome.filter),
            tree = treeSlice,
            crumbs = nodes.crumbsFor(nav.nav),
            browser = browser(nav.nav.here, chrome.filter),
            documents = documents(nav.nav),
            editor = editor,
            write = write,
            destinations = Destination.entries,
            sync = syncStatus,
            signedInAs = (user as? AuthState.LoggedIn)?.user?.email,
            onSignOut = {
                scope.launch {
                    sync.stopAndClear()
                    session.logout()
                }
            },
        )
    }
}

val shellModule: Module = module {
    single { NavStateProducer(get()) }
    single { BrowserStateProducer(get()) }
    single { DocumentStateProducer(get(), get()) }
    single { WriteStateProducer(get(), get()) }
    single { TreeStateProducer(get()) }

    bindScreen(
        ShellScreen::class,
        presenter = { _, _ -> ShellPresenter(get(), get(), get(), get(), get(), get(), get(), get(), get(), get()) },
        uiFactory = { ui<ShellScreen.State> { state, modifier -> ShellUi(state, modifier) } },
    )
}
