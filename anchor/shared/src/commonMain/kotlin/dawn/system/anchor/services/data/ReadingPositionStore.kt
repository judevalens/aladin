package dawn.system.anchor.services.data

import dawn.system.anchor.db.AnchorDatabase
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.jsonPrimitive

/**
 * The reading-position replica: where you are in each document, synced across devices.
 *
 * Same shape as [NodeStore] but deliberately smaller: the reader consults a position only
 * **at open** (apply-at-open — a remote frame must never yank the page mid-read), so this
 * store exposes a suspend read rather than a Flow. Writes are only ever the sync layer
 * applying frames — plus [ReadingPositionWriter] applying its own committed PUT through
 * the same guard, which is what makes the echo frame a no-op.
 */
data class ReadingPositionRow(
    val artifactId: String,
    val page: Int,
    /** The SERVER's unix-ms stamp from the frame — comparable across devices. */
    val updatedAt: Long,
)

/** One frame entity in the store's vocabulary. */
data class ReadingPositionChange(
    val id: String,
    val seq: Long,
    val op: String,
    val data: JsonObject?,
)

interface ReadingPositionStore {
    /** The stored position, or null when none is known. Read at open. */
    suspend fun of(artifactId: String): ReadingPositionRow?

    /** Applies a frame's entities in one transaction under the seq guard. */
    suspend fun applyAll(changes: List<ReadingPositionChange>): Int

    suspend fun clear()
}

internal class SqlDelightReadingPositionStore(
    private val db: AnchorDatabase,
    private val writer: CoroutineDispatcher,
) : ReadingPositionStore {

    private val queries get() = db.readingPositionQueries

    override suspend fun of(artifactId: String): ReadingPositionRow? = withContext(writer) {
        queries.selectLiveById(artifactId).executeAsOneOrNull()?.let { row ->
            ReadingPositionRow(
                artifactId = row.id,
                page = row.page.toInt().coerceAtLeast(1),
                updatedAt = row.updated_at,
            )
        }
    }

    override suspend fun applyAll(changes: List<ReadingPositionChange>): Int = withContext(writer) {
        db.transactionWithResult { changes.count(::applyLocked) }
    }

    /** The guard and the write; caller holds the writer dispatcher and a transaction. */
    private fun applyLocked(change: ReadingPositionChange): Boolean {
        val (id, seq, op, data) = change
        val stored = queries.storedSeq(id).executeAsOneOrNull() ?: 0L
        if (seq <= stored) return false // stale / duplicate / out-of-order

        when (op) {
            OP_DELETE -> queries.softDelete(id = id, seq = seq)
            else -> {
                // An upsert with no data cannot populate columns; skip rather than wipe.
                val light = data ?: return false
                queries.upsert(
                    id = id,
                    page = light.long("page") ?: 1L,
                    seq = seq,
                    updatedAt = light.long("updatedAt") ?: 0L,
                )
            }
        }
        return true
    }

    override suspend fun clear() = withContext(writer) {
        queries.deleteAll()
        Unit
    }

    private companion object {
        const val OP_DELETE = "delete"
    }
}

private fun JsonObject.long(key: String): Long? =
    this[key]?.jsonPrimitive?.content?.toLongOrNull()
