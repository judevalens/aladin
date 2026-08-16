package dawn.system.anchor.domain

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertSame
import kotlin.test.assertTrue

/**
 * The rev-2 navigation rules, as tests. Each one is a rule the shell would otherwise get
 * wrong in a way the user feels: an artifact that creates a level you have to climb out
 * of, a Back that skips a rung, a path popover that disagrees with the sidebar.
 */
class SidebarNavTest {

    /** Stands in for the presenter's lookup into the live tree. */
    private val titles = mapOf(
        "f1" to "Option strategies",
        "f2" to "Greeks deep-dive",
        "f3" to "Second-order greeks",
        "f9" to "nine",
    )
    private val titleOf: (String) -> String = { titles[it] ?: it }

    private fun folder(id: String, title: String = "folder $id") =
        WorkspaceNode(id = id, kind = NodeKind.Folder, parentId = null, position = 0, title = title)

    private fun artifact(id: String, title: String = "artifact $id") = WorkspaceNode(
        id = id,
        kind = NodeKind.Artifact,
        parentId = null,
        position = 0,
        title = title,
        artifactType = "file",
    )

    @Test
    fun `starts at the root menu with no way back`() {
        val nav = SidebarNav()

        assertTrue(nav.isRoot)
        assertEquals(SidebarNav.Level.Root, nav.level)
        assertFalse(nav.canBack)
        assertNull(nav.currentId)
        assertNull(nav.backLabel(null, titleOf))
    }

    @Test
    fun `opening a section shows its rows and can go back to root`() {
        val nav = SidebarNav().openSection(Destination.Folders)

        assertEquals(SidebarNav.Level.Section, nav.level)
        assertTrue(nav.canBack)
        assertEquals("Aladin", nav.backLabel("Folders", titleOf))
        assertTrue(nav.back().isRoot)
    }

    @Test
    fun `selecting a folder descends, selecting an artifact does not`() {
        val section = SidebarNav().openSection(Destination.Folders)

        val intoFolder = section.select(folder("f1"))
        assertEquals(SidebarNav.Level.Drill, intoFolder.level)
        assertEquals("f1", intoFolder.currentId)

        val ontoArtifact = section.select(artifact("a1"))
        assertEquals(SidebarNav.Level.Section, ontoArtifact.level)
        assertEquals("a1", ontoArtifact.currentId)
        // An artifact is its own view, so there is no rung to climb back out of.
        assertFalse(ontoArtifact.isDrilled)
    }

    @Test
    fun `subfolders stack and back pops one rung at a time`() {
        val nav = SidebarNav()
            .openSection(Destination.Folders)
            .select(folder("f1", "Option strategies"))
            .drillInto("f2")
            .drillInto("f3")

        assertEquals("f3", nav.currentId)
        assertEquals("Greeks deep-dive", nav.backLabel("Folders", titleOf))

        val up1 = nav.back()
        assertEquals("f2", up1.currentId)

        val up2 = up1.back()
        assertEquals("f1", up2.currentId)
        assertEquals("Folders", up2.backLabel("Folders", titleOf))

        // Out of the folder entirely, back to the section list — not straight to root.
        val up3 = up2.back()
        assertEquals(SidebarNav.Level.Section, up3.level)
        assertEquals("Aladin", up3.backLabel("Folders", titleOf))

        assertTrue(up3.back().isRoot)
    }

    @Test
    fun `drilling into the folder you are already in changes nothing`() {
        val nav = SidebarNav().openSection(Destination.Folders).select(folder("f1"))
        val again = nav.drillInto("f1")

        assertSame(nav, again)
    }

    @Test
    fun `path levels mirror the descent and mark where you are`() {
        val nav = SidebarNav()
            .openSection(Destination.Folders)
            .select(folder("f1", "Option strategies"))
            .drillInto("f2")

        val levels = nav.levels(sectionTitle = "Folders", titleOf = titleOf)

        assertEquals(listOf("Aladin", "Folders", "Option strategies", "Greeks deep-dive"), levels.map { it.title })
        assertEquals(listOf(0, 1, 2, 3), levels.map { it.depth })
        assertEquals(listOf(false, false, false, true), levels.map { it.isCurrent })
    }

    @Test
    fun `jumping to a level pops the path to it`() {
        val nav = SidebarNav()
            .openSection(Destination.Folders)
            .select(folder("f1"))
            .drillInto("f2")
            .drillInto("f3")

        assertEquals("f2", nav.jumpTo(3).currentId)
        assertEquals("f1", nav.jumpTo(2).currentId)
        assertEquals(SidebarNav.Level.Section, nav.jumpTo(1).level)
        assertTrue(nav.jumpTo(0).isRoot)
    }

    /**
     * The regression that threw you back up the tree: opening a note inside a folder
     * cleared the descent, so the folder you were reading from vanished and the sidebar
     * snapped back to the section list.
     */
    @Test
    fun `opening an artifact inside a folder stays in that folder`() {
        val inFolder = SidebarNav()
            .openSection(Destination.Folders)
            .select(folder("f1", "Option strategies"))
            .drillInto("f2")

        val reading = inFolder.select(artifact("a1"))

        // The detail shows the note…
        assertEquals("a1", reading.currentId)
        // …while the sidebar is still listing the folder it came from.
        assertEquals(SidebarNav.Level.Drill, reading.level)
        assertEquals("f2", reading.folderId)
        assertEquals(inFolder.drill, reading.drill)
        // And Back still climbs one rung of the tree rather than out of it.
        assertEquals("Option strategies", reading.backLabel("Folders", titleOf))
        assertEquals("f1", reading.back().currentId)
    }

    @Test
    fun `re-tapping the folder you are inside puts the folder back on display`() {
        val reading = SidebarNav()
            .openSection(Destination.Folders)
            .select(folder("f1"))
            .select(artifact("a1"))

        assertEquals("a1", reading.currentId)
        assertEquals("f1", reading.drillInto("f1").currentId)
    }

    @Test
    fun `switching to a sibling stays at the same depth`() {
        val nav = SidebarNav()
            .openSection(Destination.Folders)
            .select(folder("f1"))
            .drillInto("f2")

        val sideways = nav.switchTo("f9")

        assertEquals("f9", sideways.currentId)
        assertEquals(2, sideways.drill.size)
        assertEquals("f1", sideways.drill.first())
    }
}
