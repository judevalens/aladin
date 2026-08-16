package dawn.system.anchor.domain

/**
 * One page of the detail pager.
 *
 * The mapping is not one slot per open item, because **every web-backed artifact shares one
 * slot** — there is a single web view, and it pages between its own documents internally.
 * So a workspace with three notes and a PDF is two slots, not four.
 */
sealed interface SurfaceSlot {
    /** The shared web view. Present whenever anything web-backed is open, always first. */
    data object Web : SurfaceSlot

    /** An artifact with a surface of its own: a PDF, a voice note, a link. */
    data class Artifact(val node: WorkspaceNode) : SurfaceSlot

    /** A folder or research folder — rendered as a detail, not an artifact surface. */
    data class Detail(val node: WorkspaceNode) : SurfaceSlot

    /**
     * Open, but its row has not been read yet.
     *
     * The slot exists anyway. What is *open* is known synchronously; what each open thing
     * *is* arrives from the store a moment later, and letting the second decide the first
     * makes every artifact vanish for the frames between — which reads as opening a note and
     * being dumped back on an empty screen.
     */
    data class Pending(val nodeId: String) : SurfaceSlot
}

/**
 * The pager's pages, derived from what is open.
 *
 * Ordering rules, both chosen for stability rather than beauty:
 *
 *  - **The web slot is always index 0** when it exists. Pinning it means it does not shift
 *    as other things open and close, so the one view that must never be rebuilt also never
 *    changes its page identity.
 *  - Everything else follows in the order it was opened, so the pager's left-to-right order
 *    matches the switcher's list and the stepper's count.
 */
fun slotsFor(open: List<Pair<String, WorkspaceNode?>>): List<SurfaceSlot> = buildList {
    if (open.any { (_, node) -> node?.surfaceKind() == SurfaceKind.Web }) add(SurfaceSlot.Web)
    open.forEach { (id, node) ->
        when (node?.surfaceKind()) {
            SurfaceKind.Web -> Unit // already represented by the shared slot
            SurfaceKind.Artifact -> add(SurfaceSlot.Artifact(node))
            SurfaceKind.Detail -> add(SurfaceSlot.Detail(node))
            null -> add(SurfaceSlot.Pending(id))
        }
    }
}

/**
 * Which page shows [nodeId], by **id alone**.
 *
 * Deliberately not "which page shows this resolved node": which item is active is known the
 * instant you tap it, while the row behind it arrives from the store a moment later. Deriving
 * the page from the row meant every tap landed on -1 until the read completed — the pager
 * stayed where it was, and the artifact you just opened never came to the front.
 *
 * Returns the shared web page for anything web-backed, and -1 when the id is not open.
 */
fun List<SurfaceSlot>.indexOfNode(nodeId: String?): Int {
    if (nodeId == null) return -1
    val direct = indexOfFirst { slot ->
        when (slot) {
            is SurfaceSlot.Artifact -> slot.node.id == nodeId
            is SurfaceSlot.Detail -> slot.node.id == nodeId
            is SurfaceSlot.Pending -> slot.nodeId == nodeId
            SurfaceSlot.Web -> false
        }
    }
    // Not a page of its own — it is one of the documents sharing the web page.
    return if (direct >= 0) direct else indexOf(SurfaceSlot.Web)
}

/** Which of the three slot shapes a node takes. */
enum class SurfaceKind { Web, Artifact, Detail }

/**
 * A node's slot shape. `page` and `app` are web-backed and therefore share the one web
 * view; other artifacts get their own surface; folders render as a detail.
 */
fun WorkspaceNode.surfaceKind(): SurfaceKind = when {
    kind != NodeKind.Artifact -> SurfaceKind.Detail
    artifactKind?.isWebBacked == true -> SurfaceKind.Web
    else -> SurfaceKind.Artifact
}
