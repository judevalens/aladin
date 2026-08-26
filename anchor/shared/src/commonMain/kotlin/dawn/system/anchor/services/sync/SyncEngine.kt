package dawn.system.anchor.services.sync

import dawn.system.anchor.services.data.NodeChange
import dawn.system.anchor.services.data.NodeStore
import dawn.system.anchor.services.data.ReadingPositionChange
import dawn.system.anchor.services.data.ReadingPositionStore

/**
 * Routes a frame's entities to the store that owns each kind.
 *
 * The engine owns **dispatch and nothing else** — no guard, no write decision, no
 * knowledge of what a node is. That keeps two things true: a store can be tested without
 * the engine, and adding a synced kind is a registry entry plus a store, never a change
 * here.
 *
 * An unknown `entityKind` is skipped rather than failing, so a newer server can introduce
 * kinds without breaking an older client.
 */
class SyncEngine(private val handlers: Map<String, EntityHandler>) {

    /**
     * Applies one frame's entities; returns how many actually changed something.
     *
     * Grouped by handler and handed over as a batch, so each store commits once per frame
     * rather than once per row. A snapshot is a frame of thousands, and SQLDelight re-runs
     * every live query on every commit — so the batching is what keeps first sync from
     * re-reading the whole read model once per node.
     */
    suspend fun applyFrame(frame: Frame): Int {
        var applied = 0
        frame.entities
            .groupBy { handlers[it.entityKind] }
            .forEach { (handler, entities) ->
                if (handler != null) applied += handler.applyAll(entities)
            }
        return applied
    }

    companion object {
        /**
         * The registry: the tree kinds (folder, research, artifact — one table, mirroring
         * the desktop registry) plus `reading_position`. `watchlist` joins when Markets
         * needs it. A kind missing here means its frames are SILENTLY dropped — register
         * the handler in the same change that adds the server's SyncSource.
         */
        fun tree(nodes: NodeStore, readingPositions: ReadingPositionStore): SyncEngine {
            val handler = NodeEntityHandler(nodes)
            return SyncEngine(
                mapOf(
                    "folder" to handler,
                    "research" to handler,
                    "artifact" to handler,
                    "reading_position" to ReadingPositionEntityHandler(readingPositions),
                ),
            )
        }
    }
}

/** Applies one entity to whichever store owns its kind. */
/**
 * Applies entities of one kind. Batched rather than one-at-a-time so the store behind it can
 * commit once per frame — see [NodeStore.applyAll].
 */
fun interface EntityHandler {
    /** Returns how many of [entities] actually changed something. */
    suspend fun applyAll(entities: List<FrameEntity>): Int
}

internal class NodeEntityHandler(private val nodes: NodeStore) : EntityHandler {
    override suspend fun applyAll(entities: List<FrameEntity>): Int = nodes.applyAll(
        entities.map { entity ->
            NodeChange(
                kind = entity.entityKind,
                id = entity.entityId,
                seq = entity.seq,
                op = entity.op,
                data = entity.data,
            )
        },
    )
}

internal class ReadingPositionEntityHandler(
    private val store: ReadingPositionStore,
) : EntityHandler {
    override suspend fun applyAll(entities: List<FrameEntity>): Int = store.applyAll(
        entities.map { entity ->
            ReadingPositionChange(
                id = entity.entityId,
                seq = entity.seq,
                op = entity.op,
                data = entity.data,
            )
        },
    )
}
