package dawn.system.anchor.services.sync

import dawn.system.anchor.services.data.NodeStore
import dawn.system.anchor.services.data.SyncStateStore
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

/**
 * Owns the sync lifecycle: start on sign-in, stop and forget on sign-out.
 *
 * Structured concurrency replaces the desktop engine's threads and mutex — one scope, one
 * job, cancelled as a unit. There is deliberately **no push path**: writes proxy to the
 * server and come back as frames, so this class only ever pulls.
 *
 * Cadence mirrors the desktop client: a pull on start, then a recovery heartbeat that
 * bounds how stale the cursor can get in wall-clock time. The live socket is the fast path
 * and lands next; the heartbeat is what makes its absence merely slow rather than wrong.
 */
class SyncRunner(
    private val puller: SyncPuller,
    private val nodes: NodeStore,
    private val syncState: SyncStateStore,
) {
    private var scope: CoroutineScope? = null
    private var job: Job? = null

    private val _status = MutableStateFlow<SyncStatus>(SyncStatus.Idle)
    val status: StateFlow<SyncStatus> = _status.asStateFlow()

    fun start() {
        if (job?.isActive == true) return
        val runScope = CoroutineScope(SupervisorJob() + Dispatchers.Default)
        scope = runScope
        job = runScope.launch {
            while (true) {
                runCatching { puller.pullAndApply() }
                    .onSuccess { applied ->
                        _status.value = SyncStatus.Synced(appliedLastTick = applied)
                    }
                    .onFailure { error ->
                        if (error is CancellationException) throw error
                        // Offline or a transient server error: the cursor is untouched, so
                        // the next tick simply retries the same range.
                        _status.value = SyncStatus.Offline(error.message)
                    }
                delay(HEARTBEAT_MILLIS)
            }
        }
    }

    /** Runs one tick now — used on foreground and after a write. */
    fun nudge() {
        val runScope = scope ?: return
        runScope.launch { runCatching { puller.pullAndApply() } }
    }

    /** Stops syncing and drops the cache: another account must not inherit this one's tree. */
    suspend fun stopAndClear() {
        job?.cancel()
        job = null
        scope = null
        _status.value = SyncStatus.Idle
        nodes.clear()
        syncState.clear()
    }

    private companion object {
        const val HEARTBEAT_MILLIS = 20_000L
    }
}

sealed interface SyncStatus {
    data object Idle : SyncStatus
    data class Synced(val appliedLastTick: Int) : SyncStatus
    data class Offline(val reason: String?) : SyncStatus
}
