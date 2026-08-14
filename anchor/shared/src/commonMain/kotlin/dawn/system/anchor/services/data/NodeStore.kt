package dawn.system.anchor.services.data

import app.cash.sqldelight.coroutines.asFlow
import app.cash.sqldelight.coroutines.mapToList
import dawn.system.anchor.db.AnchorDatabase
import dawn.system.anchor.domain.NodeKind
import dawn.system.anchor.domain.WorkspaceNode
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.jsonPrimitive

/**
 * The workspace tree's local store.
 *
 * Reads are Flows — this is the reactive spine. The UI observes the store and the store
 * alone; nothing refetches over REST when something changes, because a change *is* a
 * frame landing here (the locked pattern).
 *
 * Writes are only ever the sync layer applying frames, and the **seq guard lives here**,
 * with the data, rather than in the engine that routes frames. That is deliberate: the
 * engine then owns nothing but dispatch, and a store can be tested for staleness
 * behaviour without any sync machinery.
 */
interface NodeStore {
    fun liveNodes(): Flow<List<WorkspaceNode>>
    fun children(parentId: String?): Flow<List<WorkspaceNode>>
    suspend fun byId(id: String): WorkspaceNode?

    /**
     * Applies one frame entity under the seq guard. Returns true when it actually changed
     * something — stale, duplicate and out-of-order entities return false and write
     * nothing.
     */
    suspend fun apply(kind: String, id: String, seq: Long, op: String, data: JsonObject?): Boolean

    /** Snapshot REPLACE: drop every row the authoritative snapshot omitted. */
    suspend fun retainOnly(ids: Collection<String>)

    suspend fun clear()
}

internal class SqlDelightNodeStore(
    private val db: AnchorDatabase,
    private val writer: CoroutineDispatcher,
) : NodeStore {

    private val queries get() = db.nodeQueries

    override fun liveNodes(): Flow<List<WorkspaceNode>> =
        queries.selectLive().asFlow().mapToList(writer).map { rows -> rows.map(::toDomain) }

    override fun children(parentId: String?): Flow<List<WorkspaceNode>> =
        queries.selectChildren(parentId).asFlow().mapToList(writer).map { rows -> rows.map(::toDomain) }

    override suspend fun byId(id: String): WorkspaceNode? = withContext(writer) {
        queries.selectById(id).executeAsOneOrNull()
            ?.takeIf { it.is_deleted == 0L }
            ?.let(::toDomain)
    }

    override suspend fun apply(
        kind: String,
        id: String,
        seq: Long,
        op: String,
        data: JsonObject?,
    ): Boolean = withContext(writer) {
        val stored = queries.storedSeq(id).executeAsOneOrNull() ?: 0L
        if (seq <= stored) return@withContext false // stale / duplicate / out-of-order

        when (op) {
            OP_DELETE -> queries.softDelete(id = id, kind = kind, seq = seq, updatedAt = seq)
            else -> {
                // An upsert with no data cannot populate columns; skip rather than wipe.
                val light = data ?: return@withContext false
                queries.upsert(
                    id = id,
                    kind = light.string("kind") ?: kind,
                    parentId = light.string("parentId"),
                    position = light.string("position")?.toLongOrNull() ?: 0L,
                    title = light.string("title").orEmpty(),
                    artifactType = light.string("artifactType"),
                    summary = light.string("summary"),
                    seq = seq,
                    updatedAt = seq,
                )
            }
        }
        true
    }

    override suspend fun retainOnly(ids: Collection<String>) = withContext(writer) {
        queries.retainOnly(ids)
        Unit
    }

    override suspend fun clear() = withContext(writer) {
        queries.deleteAll()
        Unit
    }

    private companion object {
        const val OP_DELETE = "delete"
    }
}

private fun JsonObject.string(key: String): String? =
    this[key]?.jsonPrimitive?.takeIf { it.isString || it.content != "null" }?.content
        ?.takeIf { it != "null" }

private fun toDomain(row: dawn.system.anchor.db.Node): WorkspaceNode = WorkspaceNode(
    id = row.id,
    kind = NodeKind.fromWire(row.kind) ?: NodeKind.Artifact,
    parentId = row.parent_id,
    position = row.position,
    title = row.title,
    artifactType = row.artifact_type,
    summary = row.summary,
)
