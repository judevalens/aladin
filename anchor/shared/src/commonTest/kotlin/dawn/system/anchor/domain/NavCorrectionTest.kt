package dawn.system.anchor.domain

import kotlin.test.Test
import kotlin.test.assertEquals

/**
 * When the sidebar is allowed to move the user, and — far more importantly — when it is not.
 */
class NavCorrectionTest {

    private fun folder(id: String) =
        WorkspaceNode(id, NodeKind.Folder, null, 0, id)

    private fun artifact(id: String) =
        WorkspaceNode(id, NodeKind.Artifact, null, 0, id, artifactType = "file")

    private val insideF = SidebarNav()
        .openSection(Destination.Folders)
        .select(folder("f"))

    private val readingA = insideF.select(artifact("a"))

    private val everythingOpen: (String) -> Boolean = { true }

    /**
     * The reported bug: open an artifact and it shows for a frame, then the shell falls back
     * to the folder. Nothing about a freshly opened artifact is grounds for dropping it —
     * least of all that its row has not been read yet.
     */
    @Test
    fun `an artifact just opened is never dropped`() {
        assertEquals(
            readingA,
            readingA.corrected(Presence.Unknown, Presence.Unknown, everythingOpen),
            "rows not read yet must not move the selection",
        )
        assertEquals(
            readingA,
            readingA.corrected(Presence.There, Presence.Unknown, everythingOpen),
        )
        assertEquals(
            readingA,
            readingA.corrected(Presence.There, Presence.There, everythingOpen),
        )
    }

    @Test
    fun `a deleted folder climbs out, one rung at a time`() {
        val corrected = insideF.corrected(Presence.Gone, Presence.Unknown, everythingOpen)

        assertEquals(emptyList(), corrected.drill)
    }

    @Test
    fun `a deleted artifact stops being the selection`() {
        val corrected = readingA.corrected(Presence.There, Presence.Gone, everythingOpen)

        assertEquals(null, corrected.selectedId)
        assertEquals("f", corrected.currentId, "the folder it was in still shows")
    }

    @Test
    fun `a selection that is no longer open is dropped`() {
        val corrected = readingA.corrected(Presence.There, Presence.There) { it != "a" }

        assertEquals(null, corrected.selectedId)
    }

    @Test
    fun `an unknown folder never climbs out`() {
        assertEquals(insideF, insideF.corrected(Presence.Unknown, Presence.Unknown, everythingOpen))
    }

    @Test
    fun `nothing selected is left alone`() {
        val root = SidebarNav()

        assertEquals(root, root.corrected(Presence.Unknown, Presence.Gone) { false })
    }
}
