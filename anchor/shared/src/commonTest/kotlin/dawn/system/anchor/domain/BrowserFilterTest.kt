package dawn.system.anchor.domain

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class BrowserFilterTest {

    private fun BrowserFilter.accepts(
        kind: ArtifactKind? = null,
        purpose: FolderPurpose? = null,
        state: ItemState? = null,
    ) = matches(kind, purpose, state)

    @Test
    fun `an empty filter accepts everything`() {
        val filter = BrowserFilter()
        assertTrue(filter.accepts(kind = ArtifactKind.File))
        assertTrue(filter.accepts(kind = null, purpose = null, state = null))
        assertFalse(filter.isNarrowing)
    }

    @Test
    fun `facets in one group widen`() {
        val filter = BrowserFilter().toggle(ArtifactKind.File).toggle(ArtifactKind.Link)
        assertTrue(filter.accepts(kind = ArtifactKind.File))
        assertTrue(filter.accepts(kind = ArtifactKind.Link))
        assertFalse(filter.accepts(kind = ArtifactKind.Voice))
    }

    @Test
    fun `facets across groups narrow`() {
        val filter = BrowserFilter()
            .toggle(ArtifactKind.File)
            .toggle(FolderPurpose.Research)

        assertTrue(filter.accepts(kind = ArtifactKind.File, purpose = FolderPurpose.Research))
        assertFalse(
            filter.accepts(kind = ArtifactKind.File, purpose = FolderPurpose.Plain),
            "matching the kind is not enough when a purpose is also demanded",
        )
    }

    /**
     * Asking for "unread" asks for the unread ones — not for everything not marked otherwise.
     */
    @Test
    fun `a state facet excludes items with no state at all`() {
        val filter = BrowserFilter().toggle(ItemState.Unread)
        assertTrue(filter.accepts(state = ItemState.Unread))
        assertFalse(filter.accepts(state = null))
        assertFalse(filter.accepts(state = ItemState.Stale))
    }

    @Test
    fun `toggling twice returns to where it started`() {
        val filter = BrowserFilter()
        assertEquals(filter, filter.toggle(ArtifactKind.App).toggle(ArtifactKind.App))
    }

    /**
     * Sorting reorders; it does not narrow. If it counted, a merely-reordered list would show
     * a filter badge and a pill, and read as though rows were being hidden.
     */
    @Test
    fun `sorting is not filtering`() {
        val filter = BrowserFilter().sortedBy(ItemSort.Name)
        assertFalse(filter.isNarrowing)
        assertEquals(0, filter.activeCount)
    }

    @Test
    fun `clearing drops the facets but keeps the sort`() {
        val filter = BrowserFilter()
            .toggle(ArtifactKind.File)
            .toggle(ItemState.Recovered)
            .sortedBy(ItemSort.Name)

        val cleared = filter.cleared()
        assertFalse(cleared.isNarrowing)
        assertEquals(ItemSort.Name, cleared.sort, "a sort preference survives Clear")
    }

    @Test
    fun `the badge counts every active facet across groups`() {
        val filter = BrowserFilter()
            .toggle(ArtifactKind.File)
            .toggle(ArtifactKind.Link)
            .toggle(FolderPurpose.Learning)
            .toggle(ItemState.Stale)
        assertEquals(4, filter.activeCount)
        assertTrue(filter.isNarrowing)
    }
}
