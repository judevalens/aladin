package dawn.system.anchor.app

import dawn.system.anchor.features.login.loginModule
import dawn.system.anchor.features.shell.shellModule
import dawn.system.anchor.services.auth.AuthService
import dawn.system.anchor.services.auth.KtorAuthService
import dawn.system.anchor.services.auth.SessionManager
import dawn.system.anchor.services.data.ArtifactResourceStore
import dawn.system.anchor.services.data.KtorArtifactResourceStore
import dawn.system.anchor.services.data.NodeStore
import dawn.system.anchor.services.data.SqlDelightNodeStore
import dawn.system.anchor.services.data.SqlDelightSyncStateStore
import dawn.system.anchor.services.data.SyncStateStore
import dawn.system.anchor.services.data.DocumentStore
import dawn.system.anchor.services.data.KtorDocumentStore
import dawn.system.anchor.services.data.KtorWorkspaceWriter
import dawn.system.anchor.services.data.WorkspaceWriter
import dawn.system.anchor.services.database.createDatabase
import dawn.system.anchor.services.network.ApiConfig
import dawn.system.anchor.services.network.InMemoryTokenProvider
import dawn.system.anchor.services.network.MutableTokenProvider
import dawn.system.anchor.services.network.TokenProvider
import dawn.system.anchor.services.network.anchorJson
import dawn.system.anchor.services.network.createHttpClient
import dawn.system.anchor.services.sync.KtorSyncApi
import dawn.system.anchor.services.sync.SyncApi
import dawn.system.anchor.services.sync.SyncEngine
import dawn.system.anchor.services.sync.SyncLive
import dawn.system.anchor.services.sync.SyncPuller
import dawn.system.anchor.services.sync.SyncRunner
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import org.koin.core.module.Module
import org.koin.core.qualifier.named
import org.koin.dsl.module

/** Backend connection + HTTP client. */
val networkModule: Module = module {
    single { ApiConfig() }
    single<MutableTokenProvider> { InMemoryTokenProvider() }
    single<TokenProvider> { get<MutableTokenProvider>() }
    single { createHttpClient(config = get(), tokenProvider = get()) }
}

/** Auth against `backend_v2`. The platform module supplies the token storage. */
val authModule: Module = module {
    single<AuthService> { KtorAuthService(get()) }
    single { SessionManager(auth = get(), storage = get(), tokenProvider = get()) }
}

/**
 * Local store. **All writes go through one dispatcher** — SQLite tolerates concurrent
 * readers but not concurrent writers, and a single-threaded writer is cheaper to reason
 * about than retry-on-busy scattered through the stores.
 */
@OptIn(ExperimentalCoroutinesApi::class)
val dataModule: Module = module {
    single { createDatabase(get()) }
    single<CoroutineDispatcher>(named(DB_WRITER)) { Dispatchers.Default.limitedParallelism(1) }
    // The read model's streams outlive any one screen, so they are owned by a scope that
    // lives as long as the store rather than by whatever happened to subscribe first.
    single(named(STORE_SCOPE)) { CoroutineScope(SupervisorJob() + Dispatchers.Default) }
    single<NodeStore> { SqlDelightNodeStore(get(), get(named(DB_WRITER)), get(named(STORE_SCOPE))) }
    single<SyncStateStore> { SqlDelightSyncStateStore(get(), get(named(DB_WRITER))) }
    // File bodies live outside the sync spine — fetched, then kept on disk.
    single<ArtifactResourceStore> { KtorArtifactResourceStore(get()) }
    // The ingested half of a document — outline and status, fetched per open file.
    single<DocumentStore> { KtorDocumentStore(get()) }
    // Writes proxy to Go and apply the committed result under the same seq guard.
    single<WorkspaceWriter> { KtorWorkspaceWriter(client = get(), nodes = get()) }
}

/** The client half of the sync spine: frames in, store updated, UI observes the store. */
val syncModule: Module = module {
    single<SyncApi> { KtorSyncApi(get()) }
    single { SyncEngine.tree(get()) }
    single { SyncPuller(api = get(), engine = get(), nodes = get(), syncState = get()) }
    single { SyncLive(client = get(), config = get(), engine = get(), json = anchorJson) }
    single { SyncRunner(puller = get(), nodes = get(), syncState = get(), live = get()) }
}

/** The Circuit, assembled from every registered screen's factories. */
val circuitModule: Module = module {
    single { buildCircuit(this) }
}

const val DB_WRITER = "db-writer"
const val STORE_SCOPE = "store-scope"

/**
 * Platform-independent modules; combined with a platform module at startup. Feature
 * modules self-register their Presenter/Ui via `bindScreen`.
 */
fun appModules(): List<Module> = listOf(
    networkModule,
    authModule,
    dataModule,
    syncModule,
    circuitModule,
    // feature modules (one per screen)
    loginModule,
    shellModule,
)
