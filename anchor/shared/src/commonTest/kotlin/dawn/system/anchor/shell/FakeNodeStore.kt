package dawn.system.anchor.shell

import dawn.system.anchor.domain.NodeKind
import dawn.system.anchor.domain.WorkspaceNode
import dawn.system.anchor.services.data.NodeChange
import dawn.system.anchor.services.data.NodeState
import dawn.system.anchor.services.data.NodeStore
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.map
import kotlinx.serialization.json.JsonObject

/**
 * An in-memory tree with the real store's *reactive* shape.
 *
 * The point is the per-key streams: [node] hands back the same [MutableStateFlow] for an id
 * every time, so a test can [rename] or [remove] a row and watch what a producer emits next.
 * That is the behaviour the shell actually depends on, and the thing a plain list of nodes
 * could not exercise.
 *
 * [NodeState.Loading] is the honest starting point for a row nobody has read yet, matching
 * the real store — which is what makes it possible to test that a producer does *not* treat
 * an unread row as a deleted one.
 */
class FakeNodeStore(initial: List<WorkspaceNode> = emptyList()) : NodeStore {

    private val rows = MutableStateFlow(initial.associateBy { it.id })
    private val streams = mutableMapOf<String, MutableStateFlow<NodeState>>()
    private var settled = false

    // ── test controls ────────────────────────────────────────────────────────

    fun put(node: WorkspaceNode) {
        rows.value = rows.value + (node.id to node)
        streams[node.id]?.value = NodeState.Present(node)
    }

    fun rename(id: String, title: String) {
        rows.value[id]?.let { put(it.copy(title = title)) }
    }

    /** Deleted, as the tree reports it: a *read* that finds nothing. */
    fun remove(id: String) {
        rows.value = rows.value - id
        streams[id]?.value = NodeState.Missing
    }

    // ── NodeStore ────────────────────────────────────────────────────────────

    /**
     * Starts at [NodeState.Loading], exactly like the real store: a retained stream has read
     * nothing when it is first asked. Seeding `Present` here would hide every bug that only
     * appears during that first window — which is most of them.
     */
    override fun node(id: String): StateFlow<NodeState> = streams.getOrPut(id) {
        MutableStateFlow(if (settled) read(id) else NodeState.Loading)
    }

    /**
     * Completes the pending reads, as the database would a moment later — and stays settled,
     * so a stream requested *after* this call is already read rather than starting over.
     * Composition order decides when a producer subscribes, and a test should not have to.
     */
    fun settle() {
        settled = true
        streams.forEach { (id, stream) -> stream.value = read(id) }
    }

    private fun read(id: String): NodeState =
        rows.value[id]?.let(NodeState::Present) ?: NodeState.Missing

    override fun liveNodes(): Flow<List<WorkspaceNode>> = rows.map { it.values.toList() }

    override fun children(parentId: String?): Flow<List<WorkspaceNode>> =
        rows.map { all -> all.values.filter { it.parentId == parentId } }

    override fun byArtifactType(artifactType: String): Flow<List<WorkspaceNode>> =
        rows.map { all -> all.values.filter { it.artifactType == artifactType } }

    override suspend fun byId(id: String): WorkspaceNode? = rows.value[id]

    override suspend fun apply(
        kind: String,
        id: String,
        seq: Long,
        op: String,
        data: JsonObject?,
    ): Boolean = false

    override suspend fun applyAll(changes: List<NodeChange>): Int = 0

    override suspend fun retainOnly(ids: Collection<String>) = Unit

    override suspend fun clear() {
        rows.value = emptyMap()
    }
}

/** A folder, with the fields the shell actually reads. */
fun folder(id: String, title: String = id, parentId: String? = null) =
    WorkspaceNode(id = id, kind = NodeKind.Folder, parentId = parentId, position = 0, title = title)

/** An artifact of [type] — `page` and `app` are web-backed, everything else has its own surface. */
fun artifact(id: String, type: String = "file", title: String = id, parentId: String? = null) =
    WorkspaceNode(
        id = id,
        kind = NodeKind.Artifact,
        parentId = parentId,
        position = 0,
        title = title,
        artifactType = type,
    )

/** The reader never runs in these tests; a page only needs the surface to exist. */
class FakeResourceStore : dawn.system.anchor.services.data.ArtifactResourceStore {
    override suspend fun resource(artifactId: String) = error("not used")
    override fun cached(artifactId: String) = null
}

class FakeDocumentStore : dawn.system.anchor.services.data.DocumentStore {
    override suspend fun document(artifactId: String) = null
}
