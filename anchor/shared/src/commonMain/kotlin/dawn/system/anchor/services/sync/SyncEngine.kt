package dawn.system.anchor.services.sync

import dawn.system.anchor.services.data.NodeStore

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

    /** Applies one frame's entities; returns how many actually changed something. */
    suspend fun applyFrame(frame: Frame): Int {
        var applied = 0
        for (entity in frame.entities) {
            val handler = handlers[entity.entityKind] ?: continue
            if (handler.apply(entity)) applied++
        }
        return applied
    }

    companion object {
        /**
         * The tree kinds — folder, research and artifact all live in one table, mirroring
         * the desktop registry. `watchlist` joins when Markets needs it.
         */
        fun tree(nodes: NodeStore): SyncEngine {
            val handler = NodeEntityHandler(nodes)
            return SyncEngine(
                mapOf(
                    "folder" to handler,
                    "research" to handler,
                    "artifact" to handler,
                ),
            )
        }
    }
}

/** Applies one entity to whichever store owns its kind. */
fun interface EntityHandler {
    suspend fun apply(entity: FrameEntity): Boolean
}

internal class NodeEntityHandler(private val nodes: NodeStore) : EntityHandler {
    override suspend fun apply(entity: FrameEntity): Boolean = nodes.apply(
        kind = entity.entityKind,
        id = entity.entityId,
        seq = entity.seq,
        op = entity.op,
        data = entity.data,
    )
}
