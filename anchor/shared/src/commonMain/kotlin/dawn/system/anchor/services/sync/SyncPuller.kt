package dawn.system.anchor.services.sync

import dawn.system.anchor.services.data.NodeStore
import dawn.system.anchor.services.data.SyncStateStore

/**
 * The cursor-advancing path.
 *
 * Reads the cursor, pulls frames since it, applies each under the per-entity seq guard,
 * advances the cursor to the returned horizon, and loops until the horizon stops moving.
 *
 * Four rules carry the correctness of the whole client, and each is load-bearing:
 *
 * 1. **Pull alone advances the cursor.** The live socket applies frames without touching
 *    it, because live delivery may drop, duplicate or reorder; the cursor must only ever
 *    mean "everything up to here is durably applied".
 * 2. **A snapshot REPLACES.** After applying it, rows it omitted are stale cache garbage
 *    and are removed. A delta merges.
 * 3. **A transport failure leaves the cursor alone** — never advance past frames that were
 *    not applied.
 * 4. **Idempotent.** Re-pulling the same range is a no-op thanks to the guard, which is
 *    what makes the recovery heartbeat free.
 */
class SyncPuller(
    private val api: SyncApi,
    private val engine: SyncEngine,
    private val nodes: NodeStore,
    private val syncState: SyncStateStore,
) {

    /** Pulls and applies everything available. Returns the number of entities applied. */
    suspend fun pullAndApply(): Int {
        var total = 0
        while (true) {
            val cursor = syncState.cursor()
            val result = api.pull(cursor) // a throw here leaves the cursor untouched (rule 3)

            val hasEntities = result.frames.any { it.entities.isNotEmpty() }
            if (!hasEntities) {
                // Nothing to apply, but the horizon may still have moved past events that
                // do not concern us (app events, other kinds).
                if (result.cursor > cursor) syncState.setCursor(result.cursor)
                return total
            }

            for (frame in result.frames) {
                total += engine.applyFrame(frame)
            }

            if (result.isSnapshot) {
                val ids = result.frames.flatMap { frame -> frame.entities.map { it.entityId } }
                nodes.retainOnly(ids)
            }

            syncState.setCursor(result.cursor)

            // Stop when the server stops moving the horizon, or we would loop forever on a
            // server that keeps returning the same page.
            if (result.cursor <= cursor) return total
        }
    }
}
