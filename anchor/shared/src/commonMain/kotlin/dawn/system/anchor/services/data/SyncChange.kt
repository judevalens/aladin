package dawn.system.anchor.services.data

/** The kind of change a [SyncChange] carries. */
enum class SyncOp { UPSERT, DELETE }

/**
 * A single decoded change from the server change-feed, ready to apply to local storage —
 * the typed mirror of the backend's per-entity sync frame
 * (`{ entityId, seq, op, data }`, see `backend_v2` / the Tauri client's `FrameEntity`).
 *
 * @param id  the entity id.
 * @param seq per-entity version; a change is applied only if [seq] is newer than the seq
 *   already stored for [id] (the staleness guard).
 * @param op  whether this upserts or deletes the entity.
 * @param value the decoded payload for [SyncOp.UPSERT]; `null` for [SyncOp.DELETE].
 */
data class SyncChange<out T>(
    val id: String,
    val seq: Long,
    val op: SyncOp,
    val value: T?,
)
