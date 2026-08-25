package dawn.system.anchor.domain

/**
 * One row of the workspace tree — a folder, a research folder, or an artifact. This is the
 * client's domain model, deliberately independent of both the sync wire format and the
 * SQLDelight row: the store maps into this, features only ever see this.
 *
 * Mirrors the light node shape the server puts in a frame's `data` (see
 * `backend_v2/internal/service/sync_types.go`).
 */
data class WorkspaceNode(
    val id: String,
    val kind: NodeKind,
    val parentId: String?,
    val position: Long,
    val title: String,
    /** For [NodeKind.Artifact]: page | app | link | file | voice. Null otherwise. */
    val artifactType: String? = null,
    /** A `link` artifact's target. Null for every other kind. */
    val sourceUrl: String? = null,
    val summary: String? = null,
    /**
     * The per-entity sync version (the frame seq that produced this row). Monotonic per
     * node; a change means the server-side artifact moved — which is the whole signal a
     * live surface (the board) needs to refetch its content.
     */
    val rev: Long = 0,
) {
    val isContainer: Boolean get() = kind != NodeKind.Artifact
}

/**
 * The three tree kinds the sync registry routes into one table. `research` is a third tree
 * kind rather than a separate entity — the same light node shape plus a research
 * sub-object — which is what "one tree implementation" buys (RESEARCH_SURFACE_PRD §11).
 */
enum class NodeKind(val wire: String) {
    Folder("folder"),
    Research("research"),
    Artifact("artifact"),
    ;

    companion object {
        fun fromWire(value: String): NodeKind? = entries.firstOrNull { it.wire == value }
    }
}
