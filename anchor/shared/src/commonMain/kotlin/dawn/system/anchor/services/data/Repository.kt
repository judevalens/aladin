package dawn.system.anchor.services.data

/**
 * Base for every repository: the one shared capability is applying a batch of server
 * changes into local storage. How a repository then *exposes* its data (collection flows,
 * per-entity flows, filtered queries) is defined by each concrete subclass — the base makes
 * no assumption about it.
 */
interface Repository<T> {
    /**
     * Apply a batch of server [changes] into local storage. Each change is applied only if
     * its [SyncChange.seq] is newer than the seq already stored for that id; deletes leave a
     * tombstone so a later stale upsert can't resurrect the entity. Idempotent — safe to
     * re-apply duplicate or out-of-order batches.
     */
    suspend fun sync(changes: Collection<SyncChange<T>>)
}
