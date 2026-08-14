package dawn.system.anchor.services.data

import dawn.system.anchor.db.AnchorDatabase
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.withContext

/**
 * Durable sync bookkeeping — today just the pull cursor, which is the one piece of client
 * state whose loss costs a full re-snapshot and whose corruption costs silent data gaps.
 */
interface SyncStateStore {
    suspend fun cursor(): Long
    suspend fun setCursor(value: Long)
    suspend fun clear()
}

internal class SqlDelightSyncStateStore(
    private val db: AnchorDatabase,
    private val writer: CoroutineDispatcher,
) : SyncStateStore {

    override suspend fun cursor(): Long = withContext(writer) {
        db.syncStateQueries.getValue(CURSOR_KEY).executeAsOneOrNull()?.toLongOrNull() ?: 0L
    }

    override suspend fun setCursor(value: Long) = withContext(writer) {
        db.syncStateQueries.putValue(CURSOR_KEY, value.toString())
        Unit
    }

    override suspend fun clear() = withContext(writer) {
        db.syncStateQueries.deleteAll()
        Unit
    }

    private companion object {
        const val CURSOR_KEY = "pull_cursor"
    }
}
