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
 * Cadence mirrors the desktop client, and the order matters: **the live socket is the
 * mechanism**, and the pull is recovery. A frame reaches the store as soon as the server
 * commits it; the periodic pull exists only to close the gap a dropped socket would leave,
 * which is why its interval is long rather than tuned for latency. Treating the pull as the
 * update path would make this a polling client, which the sync spine explicitly is not.
 */
class SyncRunner(
    private val puller: SyncPuller,
    private val nodes: NodeStore,
    private val syncState: SyncStateStore,
    private val live: SyncLive,
) {
    private var scope: CoroutineScope? = null
    private var job: Job? = null

    private val _status = MutableStateFlow<SyncStatus>(SyncStatus.Idle)
    val status: StateFlow<SyncStatus> = _status.asStateFlow()

    fun start() {
        if (job?.isActive == true) return
        val runScope = CoroutineScope(SupervisorJob() + Dispatchers.Default)
        scope = runScope
        // The fast path: frames applied as they are committed.
        runScope.launch { runLive() }

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

    /**
     * Holds the socket open, reconnecting with backoff. A clean close is normal — a server
     * restart, a network change — so it is retried rather than treated as failure.
     *
     * The connect-time pull is what makes a reconnect correct: the socket only carries what
     * happens after it opens, so anything committed during the gap arrives by pull.
     */
    private suspend fun runLive() {
        var backoff = LIVE_RETRY_MIN_MILLIS
        while (true) {
            val outcome = runCatching {
                live.connect(
                    onSubscribed = { puller.pullAndApply() },
                    onApplied = { applied ->
                        if (applied > 0) _status.value = SyncStatus.Synced(appliedLastTick = applied)
                    },
                )
            }
            if (outcome.exceptionOrNull() is CancellationException) throw outcome.exceptionOrNull()!!
            backoff = if (outcome.isSuccess) LIVE_RETRY_MIN_MILLIS else (backoff * 2).coerceAtMost(LIVE_RETRY_MAX_MILLIS)
            delay(backoff)
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
        /**
         * Recovery only. Long on purpose: the socket is what makes updates timely, and a
         * short interval here would be polling wearing a different name.
         */
        const val HEARTBEAT_MILLIS = 60_000L
        const val LIVE_RETRY_MIN_MILLIS = 1_000L
        const val LIVE_RETRY_MAX_MILLIS = 30_000L
    }
}

sealed interface SyncStatus {
    data object Idle : SyncStatus
    data class Synced(val appliedLastTick: Int) : SyncStatus
    data class Offline(val reason: String?) : SyncStatus
}
