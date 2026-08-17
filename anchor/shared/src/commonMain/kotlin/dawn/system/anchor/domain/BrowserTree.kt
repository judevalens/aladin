package dawn.system.anchor.domain

/**
 * The tree, indexed for the two questions a filtered browser asks that a single column cannot
 * answer: **what does a folder contain**, and **what purpose does an item inherit**.
 *
 * Both are whole-subtree questions. A folder's row shows how many matching items are *under*
 * it — not how many children it has — and a folder with none drops out of its column entirely,
 * because a folder you can enter only to find nothing is worse than one that is not offered.
 * Neither can be answered from the column's own rows.
 *
 * **Purpose is inherited, not owned.** A note inside a research folder is research material;
 * the note itself carries no purpose and never will, since the server has no field for one on
 * an artifact. So filtering by Research has to reach the leaves through their ancestry, which
 * is what the prototype does when it carries `purpose` down its walk (`:643`). Without this,
 * a purpose facet would match no leaf at all and read as "nothing here".
 *
 * Built from a flat list because that is what the store hands over — the tree is a local
 * replica, so this is an index over memory rather than a query.
 */
class BrowserTree(nodes: List<WorkspaceNode>) {

    private val byParent: Map<String?, List<WorkspaceNode>> = nodes.groupBy { it.parentId }
    private val byId: Map<String, WorkspaceNode> = nodes.associateBy { it.id }

    /**
     * The purpose an item answers to: its own if it is a folder, otherwise the topmost folder
     * it sits under.
     *
     * The **topmost**, matching the prototype: a plain folder inside a research folder is
     * still research material, because the thing that gives the work its character is the
     * project it belongs to rather than the drawer within it.
     */
    fun purposeOf(node: WorkspaceNode): FolderPurpose? {
        FolderPurpose.of(node)?.let { own -> if (node.parentId == null) return own }
        var cursor: WorkspaceNode = node
        var depth = 0
        while (depth++ < MAX_DEPTH) {
            val parent = cursor.parentId?.let(byId::get) ?: break
            cursor = parent
        }
        return FolderPurpose.of(cursor)
    }

    /**
     * How many items under [folderId] survive [filter] — the number a folder row shows, and
     * the test for whether it is offered at all.
     *
     * Counts **leaves only**: a folder is a container, not a thing you can open, so counting
     * folders would report a shape rather than an amount.
     */
    fun matchingLeaves(folderId: String, filter: BrowserFilter): Int =
        leavesUnder(folderId, 0).count { leaf ->
            filter.matches(
                kind = leaf.artifactKind,
                purpose = purposeOf(leaf),
                // Nothing can report a state yet — see ItemState. A state facet therefore
                // matches nothing, which is honest and visible rather than silently ignored.
                state = null,
            )
        }

    private fun leavesUnder(folderId: String, depth: Int): List<WorkspaceNode> {
        if (depth > MAX_DEPTH) return emptyList()
        return byParent[folderId].orEmpty().flatMap { child ->
            if (child.isContainer) leavesUnder(child.id, depth + 1) else listOf(child)
        }
    }

    private companion object {
        /**
         * A cycle in the parent links would otherwise recurse forever. The tree cannot have
         * one, but a partially-synced replica briefly can, and a hang is a worse failure than
         * a count that is momentarily short.
         */
        const val MAX_DEPTH = 16
    }
}
