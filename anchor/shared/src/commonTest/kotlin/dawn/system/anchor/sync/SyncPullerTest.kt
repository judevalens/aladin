package dawn.system.anchor.sync

import dawn.system.anchor.domain.WorkspaceNode
import dawn.system.anchor.services.data.NodeChange
import dawn.system.anchor.services.data.NodeStore
import dawn.system.anchor.services.data.SyncStateStore
import dawn.system.anchor.services.sync.Frame
import dawn.system.anchor.services.sync.FrameEntity
import dawn.system.anchor.services.sync.SyncApi
import dawn.system.anchor.services.sync.SyncEngine
import dawn.system.anchor.services.sync.SyncPuller
import dawn.system.anchor.services.sync.SyncPullResult
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.JsonObject
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue
import kotlin.test.fail

/**
 * The puller's contract, ported from the Rust client's suite
 * (`src-tauri/src/sync/pull.rs`) — those tests are the specification for this port, so the
 * cases are deliberately the same ones.
 */
class SyncPullerTest {

    // A store that records what happened, so the rules can be asserted without SQLite.
    private class RecordingStore : NodeStore {
        val applied = mutableListOf<Triple<String, Long, String>>()
        var retained: Collection<String>? = null
        var cleared = false
        private val seqs = mutableMapOf<String, Long>()

        override fun liveNodes(): Flow<List<WorkspaceNode>> = flowOf(emptyList())
        override fun node(id: String): SharedFlow<WorkspaceNode?> =
            MutableSharedFlow<WorkspaceNode?>(replay = 1).apply { tryEmit(null) }
        override fun children(parentId: String?): Flow<List<WorkspaceNode>> = flowOf(emptyList())
        override fun byArtifactType(artifactType: String): Flow<List<WorkspaceNode>> = flowOf(emptyList())
        override suspend fun byId(id: String): WorkspaceNode? = null

        override suspend fun applyAll(changes: List<NodeChange>): Int =
            changes.count { apply(it.kind, it.id, it.seq, it.op, it.data) }

        override suspend fun apply(
            kind: String,
            id: String,
            seq: Long,
            op: String,
            data: JsonObject?,
        ): Boolean {
            // The real guard lives in the store; mirror it so puller tests see real skips.
            if (seq <= (seqs[id] ?: 0L)) return false
            seqs[id] = seq
            applied += Triple(id, seq, op)
            return true
        }

        override suspend fun retainOnly(ids: Collection<String>) { retained = ids.toList() }
        override suspend fun clear() { cleared = true }
    }

    private class FakeSyncState : SyncStateStore {
        var value = 0L
        override suspend fun cursor(): Long = value
        override suspend fun setCursor(value: Long) { this.value = value }
        override suspend fun clear() { value = 0L }
    }

    private class ScriptedApi(private val responses: MutableList<SyncPullResult>) : SyncApi {
        val calls = mutableListOf<Long>()
        override suspend fun pull(since: Long): SyncPullResult {
            calls += since
            return if (responses.isEmpty()) {
                SyncPullResult(frames = emptyList(), cursor = since, mode = "delta")
            } else {
                responses.removeAt(0)
            }
        }
    }

    private fun upsert(id: String, seq: Long) = FrameEntity(
        entityKind = "folder",
        entityId = id,
        seq = seq,
        op = "upsert",
        data = JsonObject(emptyMap()),
    )

    private fun puller(api: SyncApi, store: RecordingStore, state: FakeSyncState) =
        SyncPuller(api, SyncEngine.tree(store), store, state)

    @Test
    fun applies_frames_advances_cursor_and_is_idempotent() = runTest {
        val store = RecordingStore()
        val state = FakeSyncState()
        val api = ScriptedApi(
            mutableListOf(
                SyncPullResult(
                    frames = listOf(Frame(listOf(upsert("f1", 1), upsert("f1", 2)))),
                    cursor = 7,
                    mode = "snapshot",
                ),
            ),
        )

        val applied = puller(api, store, state).pullAndApply()

        assertEquals(2, applied)
        assertEquals(7, state.value)
        assertEquals(listOf(0L, 7L), api.calls, "it keeps pulling until the horizon stops moving")

        // Re-pulling from the advanced cursor changes nothing.
        assertEquals(0, puller(api, store, state).pullAndApply())
    }

    @Test
    fun a_snapshot_replaces_removing_omitted_rows() = runTest {
        val store = RecordingStore()
        val state = FakeSyncState()
        val api = ScriptedApi(
            mutableListOf(
                SyncPullResult(
                    frames = listOf(Frame(listOf(upsert("f1", 9), upsert("f3", 9)))),
                    cursor = 20,
                    mode = "snapshot",
                ),
            ),
        )

        puller(api, store, state).pullAndApply()

        assertEquals(listOf("f1", "f3"), store.retained?.toList(), "REPLACE keeps only the snapshot's ids")
    }

    @Test
    fun a_delta_merges_and_never_removes() = runTest {
        val store = RecordingStore()
        val state = FakeSyncState()
        val api = ScriptedApi(
            mutableListOf(
                SyncPullResult(
                    frames = listOf(Frame(listOf(upsert("f1", 3)))),
                    cursor = 5,
                    mode = "delta",
                ),
            ),
        )

        puller(api, store, state).pullAndApply()

        assertEquals(null, store.retained, "a delta must not prune the cache")
    }

    @Test
    fun an_empty_pull_is_a_no_op_but_still_advances_a_moved_horizon() = runTest {
        val store = RecordingStore()
        val state = FakeSyncState()
        val api = ScriptedApi(
            mutableListOf(SyncPullResult(frames = emptyList(), cursor = 4, mode = "delta")),
        )

        assertEquals(0, puller(api, store, state).pullAndApply())
        assertEquals(4, state.value, "events we do not store still move the horizon")
    }

    @Test
    fun a_transport_error_preserves_the_cursor() = runTest {
        val store = RecordingStore()
        val state = FakeSyncState().apply { value = 11 }
        val failing = object : SyncApi {
            override suspend fun pull(since: Long): SyncPullResult = throw IllegalStateException("offline")
        }

        try {
            puller(failing, store, state).pullAndApply()
            fail("expected the transport error to propagate")
        } catch (expected: IllegalStateException) {
            // The point: nothing was applied, so the cursor must not have moved.
        }
        assertEquals(11, state.value)
    }

    @Test
    fun an_unknown_entity_kind_is_skipped_rather_than_failing() = runTest {
        val store = RecordingStore()
        val state = FakeSyncState()
        val api = ScriptedApi(
            mutableListOf(
                SyncPullResult(
                    frames = listOf(
                        Frame(
                            listOf(
                                FrameEntity("wormhole", "w1", 1, "upsert", JsonObject(emptyMap())),
                                upsert("f1", 1),
                            ),
                        ),
                    ),
                    cursor = 3,
                    mode = "delta",
                ),
            ),
        )

        assertEquals(1, puller(api, store, state).pullAndApply(), "the known kind still applies")
        assertTrue(store.applied.none { it.first == "w1" })
    }
}
