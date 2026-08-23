package dawn.system.anchor.domain

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

/**
 * The two whole-subtree questions a filtered browser asks. Both were previously unaskable —
 * the producer watched one folder's children at a time — and both are the difference between
 * a filter that narrows and a filter that empties the screen.
 */
class BrowserTreeTest {

    private fun folder(id: String, parent: String? = null, research: Boolean = false) =
        WorkspaceNode(
            id = id,
            kind = if (research) NodeKind.Research else NodeKind.Folder,
            parentId = parent,
            position = 0,
            title = id,
        )

    private fun note(id: String, parent: String?, type: String = "page") =
        WorkspaceNode(
            id = id,
            kind = NodeKind.Artifact,
            parentId = parent,
            position = 0,
            title = id,
            artifactType = type,
        )

    /** `Semis cycle › Sources › Transcripts`, with leaves at three depths. */
    private val tree = BrowserTree(
        listOf(
            folder("semis", research = true),
            note("overview", "semis"),
            folder("sources", "semis"),
            note("tsm", "sources", type = "file"),
            folder("transcripts", "sources"),
            note("nvda", "transcripts", type = "file"),
            folder("empty"),
        ),
    )

    @Test
    fun `the index answers children and lookups directly, for the tree walk`() {
        assertEquals(listOf("semis", "empty"), tree.childrenOf(null).map { it.id })
        assertEquals(listOf("tsm", "transcripts"), tree.childrenOf("sources").map { it.id })
        assertEquals("nvda", tree.node("nvda")?.id)
        assertNull(tree.node("nope"))
    }

    @Test
    fun `a folder counts every matching item beneath it, not just its children`() {
        assertEquals(3, tree.matchingLeaves("semis", BrowserFilter()), "overview, tsm, nvda")
        assertEquals(2, tree.matchingLeaves("sources", BrowserFilter()))
        assertEquals(1, tree.matchingLeaves("transcripts", BrowserFilter()))
    }

    /**
     * The rule the count exists for: a folder you can enter only to find nothing is worse than
     * one that is not offered.
     */
    @Test
    fun `a folder with nothing matching counts zero, which is what drops it from its column`() {
        val onlyNotes = BrowserFilter(kinds = setOf(ArtifactKind.Page))

        assertEquals(0, tree.matchingLeaves("transcripts", onlyNotes), "it holds only a file")
        assertEquals(1, tree.matchingLeaves("semis", onlyNotes), "but the overview survives")
        assertEquals(0, tree.matchingLeaves("empty", BrowserFilter()))
    }

    /**
     * Without inheritance a purpose facet matches no leaf at all — artifacts carry no purpose
     * and the server has no field to give them one — so "Research" would empty the screen.
     */
    @Test
    fun `a leaf inherits the purpose of the project it sits in, however deep`() {
        val nvda = note("nvda", "transcripts")

        assertEquals(FolderPurpose.Research, tree.purposeOf(nvda), "three levels down")
        assertEquals(
            1,
            tree.matchingLeaves("transcripts", BrowserFilter(purposes = setOf(FolderPurpose.Research))),
        )
    }

    @Test
    fun `a folder answers to its own purpose when it is a root`() {
        assertEquals(FolderPurpose.Research, tree.purposeOf(folder("semis", research = true)))
        assertEquals(FolderPurpose.Plain, tree.purposeOf(folder("empty")))
    }

    /**
     * A partially-synced replica can transiently hold a parent link that loops. A short count
     * is recoverable; a hang is not.
     */
    @Test
    fun `a cycle in the parent links terminates instead of hanging`() {
        val cyclic = BrowserTree(
            listOf(
                folder("a", parent = "b"),
                folder("b", parent = "a"),
                note("leaf", "a"),
            ),
        )

        assertEquals(0, cyclic.matchingLeaves("a", BrowserFilter(kinds = setOf(ArtifactKind.File))))
        assertNull(cyclic.purposeOf(note("leaf", "a")).takeIf { it == FolderPurpose.Research })
    }
}
