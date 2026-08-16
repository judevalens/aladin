package dawn.system.anchor.domain

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * How open things map onto pager pages. The rule that carries the most weight is that every
 * web-backed artifact shares one page — there is a single web view, and it pages between its
 * own documents internally.
 */
class SurfaceSlotsTest {

    private fun artifact(id: String, type: String) = WorkspaceNode(
        id = id, kind = NodeKind.Artifact, parentId = null, position = 0,
        title = id, artifactType = type,
    )

    private val note1 = artifact("note-1", "page")
    private val note2 = artifact("note-2", "page")
    private val shard1 = artifact("shard-1", "app")
    private val pdf1 = artifact("pdf-1", "file")
    private val pdf2 = artifact("pdf-2", "file")
    private val folder1 = WorkspaceNode("folder-1", NodeKind.Folder, null, 0, "folder-1")

    @Test
    fun `every web-backed artifact shares one page`() {
        val slots = slotsFor(listOf(note1.id to note1, pdf1.id to pdf1, note2.id to note2, shard1.id to shard1))

        assertEquals(listOf(SurfaceSlot.Web, SurfaceSlot.Artifact(pdf1)), slots)
    }

    /**
     * Pinned first so the one view that must never be rebuilt never changes page identity
     * as other things open and close around it.
     */
    @Test
    fun `the web page is always first`() {
        val slots = slotsFor(listOf(pdf1.id to pdf1, folder1.id to folder1, note1.id to note1))

        assertEquals(SurfaceSlot.Web, slots.first())
        assertEquals(3, slots.size)
    }

    @Test
    fun `there is no web page when nothing web-backed is open`() {
        val slots = slotsFor(listOf(pdf1.id to pdf1, folder1.id to folder1))

        assertTrue(SurfaceSlot.Web !in slots)
        assertEquals(listOf(SurfaceSlot.Artifact(pdf1), SurfaceSlot.Detail(folder1)), slots)
    }

    @Test
    fun `everything else keeps the order it was opened in`() {
        val slots = slotsFor(listOf(pdf2.id to pdf2, pdf1.id to pdf1, folder1.id to folder1))

        assertEquals(
            listOf(SurfaceSlot.Artifact(pdf2), SurfaceSlot.Artifact(pdf1), SurfaceSlot.Detail(folder1)),
            slots,
        )
    }

    @Test
    fun `any web-backed node resolves to the shared page`() {
        val slots = slotsFor(listOf(note1.id to note1, pdf1.id to pdf1, note2.id to note2))

        assertEquals(0, slots.indexOfNode("note-1"))
        assertEquals(0, slots.indexOfNode("note-2"))
        assertEquals(1, slots.indexOfNode("pdf-1"))
    }

    @Test
    fun `a node that is not open has no page`() {
        val slots = slotsFor(listOf(pdf1.id to pdf1))

        assertEquals(-1, slots.indexOfNode("pdf-2"))
        assertEquals(-1, slots.indexOfNode(null))
    }

    /**
     * An item whose row has not been read yet still gets a slot. Driving the pager off the
     * resolved rows alone emptied it for the frames in between, which read as opening a note
     * and being dumped back on an empty screen.
     */
    /**
     * The pager follows the id, not the row. A tap is known immediately; the row behind it is
     * a store read that lands later — so resolving the page from the row left every tap on
     * the previous page until the read completed.
     */
    @Test
    fun `a page is found by id, before its row has been read`() {
        val slots = slotsFor(listOf("pdf-1" to null, "note-1" to note1))

        // Web is pinned first, so the still-unread PDF is page 1 — and findable, which is
        // the point: the pager can land on it before the store has answered.
        assertEquals(1, slots.indexOfNode("pdf-1"), "a pending item still has its page")
        assertEquals(0, slots.indexOfNode("note-1"), "web-backed ids share the web page")
    }

    @Test
    fun `an unresolved item holds its place`() {
        val slots = slotsFor(listOf("pdf-1" to pdf1, "later" to null))

        assertEquals(listOf(SurfaceSlot.Artifact(pdf1), SurfaceSlot.Pending("later")), slots)
    }

    @Test
    fun `slot shape follows the artifact type`() {
        // Panes in the shared web view.
        assertEquals(SurfaceKind.Web, artifact("a", "page").surfaceKind())
        assertEquals(SurfaceKind.Web, artifact("a", "app").surfaceKind())

        // Surfaces of their own.
        assertEquals(SurfaceKind.Artifact, artifact("a", "file").surfaceKind())
        assertEquals(SurfaceKind.Artifact, artifact("a", "voice").surfaceKind())
        assertEquals(SurfaceKind.Artifact, artifact("a", "link").surfaceKind())

        assertEquals(SurfaceKind.Detail, folder1.surfaceKind())
    }
}
