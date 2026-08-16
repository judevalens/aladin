package dawn.system.anchor.data

import dawn.system.anchor.domain.NodeKind
import dawn.system.anchor.domain.WorkspaceNode
import dawn.system.anchor.services.data.NodeState
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotEquals
import kotlin.test.assertNull

/**
 * Three states rather than a nullable node, and the distinction is not academic: it cost two
 * regressions.
 *
 * A prune that treated "no node" as "deleted" reverted every drill on the frame it happened,
 * because a retained stream has not read anything yet when the sidebar first asks. Only
 * [NodeState.Missing] is evidence of deletion; [NodeState.Loading] is evidence of nothing.
 */
class NodeStateTest {

    private val folder = WorkspaceNode("f1", NodeKind.Folder, null, 0, "Semis cycle")

    @Test
    fun `only Present carries a node`() {
        assertEquals(folder, NodeState.Present(folder).node)
        assertNull(NodeState.Loading.node)
        assertNull(NodeState.Missing.node)
    }

    /**
     * The property that matters: the two node-less states are not interchangeable, so code
     * that acts on deletion cannot accidentally match a first frame.
     */
    @Test
    fun `loading and missing are distinguishable`() {
        assertNotEquals<NodeState>(NodeState.Loading, NodeState.Missing)

        val deleted: NodeState = NodeState.Missing
        val unread: NodeState = NodeState.Loading

        assertEquals(true, deleted is NodeState.Missing)
        assertEquals(false, unread is NodeState.Missing)
    }
}
