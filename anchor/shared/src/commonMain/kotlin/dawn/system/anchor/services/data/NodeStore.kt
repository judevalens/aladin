package dawn.system.anchor.services.data

import app.cash.sqldelight.coroutines.asFlow
import app.cash.sqldelight.coroutines.mapToList
import app.cash.sqldelight.coroutines.mapToOneOrNull
import dawn.system.anchor.db.AnchorDatabase
import dawn.system.anchor.domain.NodeKind
import dawn.system.anchor.domain.WorkspaceNode
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.shareIn
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
/** One entity from a frame, in the store's own vocabulary rather than the sync wire's. */
data class NodeChange(
    val kind: String,
    val id: String,
    val seq: Long,
    val op: String,
    val data: JsonObject?,
)

interface NodeStore {
    fun liveNodes(): Flow<List<WorkspaceNode>>

    /**
     * One node, as a **retained** stream.
     *
     * Two properties, both load-bearing:
     *
     *  - It is a *query*, not a search through a materialised list. `id` is the primary key,
     *    so SQLite answers with an index and the flow re-emits only when that row changes —
     *    a heading bound to it does not recompute because something unrelated synced.
     *  - The same id returns the **same shared stream**, holding its current value. So
     *    opening a fourth item does not re-read the three already open: their streams are
     *    still warm and replay what they hold. Rebuilding a combined subscription would
     *    otherwise re-query every open row every time the set changed by one.
     *
     * Emits the row, or **null once it is read and gone**. It deliberately does *not* model
     * "still loading": that is a property of a subscription, not of a node, and naming it
     * here forced an initial value the store cannot actually observe. A collector that has
     * not received anything yet simply has not — and the domain already owns the vocabulary
     * for that distinction in [dawn.system.anchor.domain.Presence], where the rule that
     * needs it lives.
     */
    fun node(id: String): SharedFlow<WorkspaceNode?>

    fun children(parentId: String?): Flow<List<WorkspaceNode>>

    /** Every artifact of one type — what the Sources section lists. */
    fun byArtifactType(artifactType: String): Flow<List<WorkspaceNode>>
    suspend fun byId(id: String): WorkspaceNode?

    /**
     * Applies one frame entity under the seq guard. Returns true when it actually changed
     * something — stale, duplicate and out-of-order entities return false and write
     * nothing.
     */
    suspend fun apply(kind: String, id: String, seq: Long, op: String, data: JsonObject?): Boolean

    /**
     * Applies a whole frame in **one transaction**, returning how many entities changed.
     *
     * Not an optimisation of the writes — of the *reads*. SQLDelight notifies query listeners
     * at table granularity, so applying a 2000-row snapshot one row at a time re-executes
     * every live query 2000 times. Committing once collapses that to a single notification,
     * which is the difference between a first sync that lands and one that thrashes.
     *
     * Semantics are unchanged: same order, same per-entity seq guard, same tombstones.
     */
    suspend fun applyAll(changes: List<NodeChange>): Int

    /** Snapshot REPLACE: drop every row the authoritative snapshot omitted. */
    suspend fun retainOnly(ids: Collection<String>)

    suspend fun clear()
}

internal class SqlDelightNodeStore(
    private val db: AnchorDatabase,
    private val writer: CoroutineDispatcher,
    /** Owns the retained per-key streams; lives as long as the store does. */
    private val scope: CoroutineScope,
) : NodeStore {

    /**
     * One shared stream per node id, created on first ask and kept.
     *
     * `WhileSubscribed` with a grace period is what makes a switch cheap: leaving a surface
     * stops the query, but coming back within a moment resumes from the retained value
     * rather than paying for a fresh read.
     *
     * Confined to the main thread, which is where composition asks for them.
     */
    private val streams = mutableMapOf<String, SharedFlow<WorkspaceNode?>>()

    private val queries get() = db.nodeQueries

    override fun liveNodes(): Flow<List<WorkspaceNode>> =
        queries.selectLive().asFlow().mapToList(writer).map { rows -> rows.map(::toDomain) }

    override fun node(id: String): SharedFlow<WorkspaceNode?> = streams.getOrPut(id) {
        queries.selectLiveById(id).asFlow().mapToOneOrNull(writer)
            .map { row -> row?.let(::toDomain) }
            // SQLDelight notifies at TABLE granularity, so every write to `node` re-runs
            // every registered query — including all N of these. `stateIn` used to absorb
            // that for free, because StateFlow conflates by equality; `shareIn` does not, so
            // without this one write would wake every open row with an identical value.
            .distinctUntilChanged()
            .shareIn(scope, SharingStarted.WhileSubscribed(RETENTION_MILLIS), replay = 1)
    }

    override fun children(parentId: String?): Flow<List<WorkspaceNode>> =
        queries.selectChildren(parentId).asFlow().mapToList(writer).map { rows -> rows.map(::toDomain) }

    override fun byArtifactType(artifactType: String): Flow<List<WorkspaceNode>> =
        queries.selectByArtifactType(artifactType).asFlow().mapToList(writer)
            .map { rows -> rows.map(::toDomain) }

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
        db.transactionWithResult { applyLocked(NodeChange(kind, id, seq, op, data)) }
    }

    override suspend fun applyAll(changes: List<NodeChange>): Int = withContext(writer) {
        db.transactionWithResult {
            changes.count(::applyLocked)
        }
    }

    /**
     * The guard and the write, assuming the caller already holds the writer and a
     * transaction. Kept separate so a batch pays for one commit rather than one per row.
     */
    private fun applyLocked(change: NodeChange): Boolean {
        val (kind, id, seq, op, data) = change
        val stored = queries.storedSeq(id).executeAsOneOrNull() ?: 0L
        if (seq <= stored) return false // stale / duplicate / out-of-order

        when (op) {
            OP_DELETE -> queries.softDelete(id = id, kind = kind, seq = seq, updatedAt = seq)
            else -> {
                // An upsert with no data cannot populate columns; skip rather than wipe.
                val light = data ?: return false
                queries.upsert(
                    id = id,
                    kind = light.string("kind") ?: kind,
                    parentId = light.string("parentId"),
                    position = light.string("position")?.toLongOrNull() ?: 0L,
                    title = light.string("title").orEmpty(),
                    // The wire calls it `type`, not `artifactType` — see the Rust client's
                    // LightNodeData (`#[serde(rename = "type")]`). Reading the wrong key
                    // left every artifact typeless: no chips, an empty Sources section, and
                    // a page that could never open as its editor.
                    artifactType = light.string("type"),
                    sourceUrl = light.string("sourceUrl"),
                    summary = light.string("summary"),
                    seq = seq,
                    updatedAt = seq,
                )
            }
        }
        return true
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
        /** Long enough that switching away and back does not re-read the row. */
        const val RETENTION_MILLIS = 5_000L
    }
}

/**
 * Watches [ids], one subscription each, and emits them **in the order given**.
 *
 * The list is assembled by `combine` rather than by a query returning rows and a caller
 * putting them back in order — so each id watches only its own row, a rename re-emits that
 * one and nothing else, and an id whose row is gone simply stops appearing. There is no map
 * to maintain and no ordering to restore, which is the whole reason this is not `IN (:ids)`.
 */
fun NodeStore.each(ids: List<String>): Flow<List<WorkspaceNode>> =
    if (ids.isEmpty()) flowOf(emptyList())
    else combine(ids.map(::node)) { rows -> rows.filterNotNull() }

/** The same, for each folder's contents: one subscription per folder, flattened. */
fun NodeStore.eachChildren(parentIds: List<String>): Flow<List<WorkspaceNode>> =
    if (parentIds.isEmpty()) flowOf(emptyList())
    else combine(parentIds.map { children(it) }) { lists -> lists.toList().flatten() }

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
    sourceUrl = row.source_url,
    summary = row.summary,
)
