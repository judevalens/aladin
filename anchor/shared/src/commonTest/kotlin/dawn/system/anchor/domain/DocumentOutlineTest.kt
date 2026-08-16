package dawn.system.anchor.domain

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * The reader's two rules that are easy to get subtly wrong and hard to notice on a device:
 * which outline row is lit, and whether the page stage renders at all.
 */
class DocumentOutlineTest {

    private val document = IngestedDocument(
        status = DocumentStatus.Ready,
        pageCount = 361,
        outline = listOf(
            OutlineEntry("Front matter", page = 1, depth = 0),
            OutlineEntry("4 Options", page = 88, depth = 0),
            OutlineEntry("4.2 Collars", page = 92, depth = 1),
            OutlineEntry("5 Volatility", page = 140, depth = 0),
        ),
        outlineSource = OutlineSource.Recovered,
    )

    @Test
    fun `the active entry is a position, not a link list`() {
        // Page 94 starts no section, but it is unambiguously inside §4.2.
        assertEquals("4.2 Collars", document.entryAt(94)?.title)
        // Exactly on a boundary belongs to the section that starts there.
        assertEquals("4.2 Collars", document.entryAt(92)?.title)
        assertEquals("4 Options", document.entryAt(91)?.title)
        assertEquals("5 Volatility", document.entryAt(361)?.title)
    }

    @Test
    fun `a page before the first entry lights nothing`() {
        val late = document.copy(
            outline = listOf(OutlineEntry("4 Options", page = 88, depth = 0)),
        )

        assertNull(late.entryAt(12))
        assertEquals("4 Options", late.entryAt(88)?.title)
    }

    @Test
    fun `a document with no outline still reads`() {
        val bare = document.copy(outline = emptyList(), outlineSource = OutlineSource.None)

        assertNull(bare.entryAt(94))
        assertTrue(bare.status.isReadable)
    }

    @Test
    fun `only a ready document renders its pages`() {
        assertTrue(DocumentStatus.Ready.isReadable)
        assertFalse(DocumentStatus.Pending.isReadable)
        assertFalse(DocumentStatus.Ingesting.isReadable)
        assertFalse(DocumentStatus.Unsupported.isReadable)
        assertFalse(DocumentStatus.Failed.isReadable)
    }

    /**
     * The bytes are already on the device by the time status is read, so an unrecognised
     * ingestion state must not be a reason to withhold the file.
     */
    @Test
    fun `an unknown status still shows the file`() {
        assertEquals(DocumentStatus.Ready, DocumentStatus.fromWire(null))
        assertEquals(DocumentStatus.Ready, DocumentStatus.fromWire("something-new"))
        assertEquals(DocumentStatus.Failed, DocumentStatus.fromWire("failed"))
    }
}
