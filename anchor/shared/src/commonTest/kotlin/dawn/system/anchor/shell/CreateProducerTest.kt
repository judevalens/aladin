package dawn.system.anchor.shell

import app.cash.turbine.ReceiveTurbine
import com.slack.circuit.test.presenterTestOf
import dawn.system.anchor.domain.WorkspaceNode
import dawn.system.anchor.features.shell.state.ChromeEvent
import dawn.system.anchor.features.shell.state.WriteEvent
import dawn.system.anchor.features.shell.state.WriteStateProducer
import dawn.system.anchor.features.shell.state.rememberChromeState
import dawn.system.anchor.services.data.WorkspaceWriter
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue
import kotlinx.coroutines.test.runTest

/**
 * The plus, from the tap to the write.
 *
 * The board kind shipped before there was any way to make one from the iPad: the producer knew
 * `CreateBoard` and nothing emitted it. These cover the path that closes that — the menu opens
 * and closes as chrome, and each row reaches the writer with the folder you are standing in.
 */
class CreateProducerTest {

    private suspend fun <T> ReceiveTurbine<T>.awaitUntil(
        limit: Int = 12,
        predicate: (T) -> Boolean,
    ): T {
        repeat(limit) {
            val item = awaitItem()
            if (predicate(item)) return item
        }
        error("no frame satisfied the condition within $limit")
    }

    /** What the writer was asked to make. */
    private data class Made(val kind: String, val parentId: String?, val title: String)

    /** Records what was asked for, and hands back an id the store already knows. */
    private class RecordingWriter(private val newId: String = "made") : WorkspaceWriter {
        val calls = mutableListOf<Made>()

        override suspend fun createFolder(parentId: String?, title: String): String {
            calls += Made("folder", parentId, title)
            return newId
        }

        override suspend fun createPage(parentId: String?, title: String): String {
            calls += Made("page", parentId, title)
            return newId
        }

        override suspend fun createBoard(parentId: String?, title: String): String {
            calls += Made("board", parentId, title)
            return newId
        }

        override suspend fun rename(node: WorkspaceNode, title: String) = Unit
        override suspend fun delete(node: WorkspaceNode) = Unit
    }

    @Test
    fun `the plus opens a menu and going somewhere closes it`() = runTest {
        presenterTestOf({ rememberChromeState() }) {
            val closed = awaitItem()
            assertFalse(closed.createOpen, "the menu starts closed")

            closed.handle(ChromeEvent.ToggleCreate)
            val open = awaitUntil { it.createOpen }

            // Overlays close when you go somewhere — the menu is one, the sidebar is not.
            open.dismissOverlays()
            val dismissed = awaitUntil { !it.createOpen }
            assertTrue(dismissed.sidebarOpen, "the sidebar is not an overlay")
            cancelAndIgnoreRemainingEvents()
        }
    }

    /** Never both: a choice about the workspace is not a thing beside the browser's list. */
    @Test
    fun `opening the menu closes the browser dropdown`() = runTest {
        presenterTestOf({ rememberChromeState() }) {
            awaitItem().handle(ChromeEvent.ToggleBrowser)
            val browsing = awaitUntil { it.browserOpen }

            // Await the settled condition, not the first frame that mentions the menu: these
            // are two snapshot writes, so a frame with both open is a legal intermediate.
            browsing.handle(ChromeEvent.ToggleCreate)
            awaitUntil { it.createOpen && !it.browserOpen }
            cancelAndIgnoreRemainingEvents()
        }
    }

    @Test
    fun `new board writes a board into the folder you are standing in and opens it`() = runTest {
        val nodes = FakeNodeStore(listOf(artifact("made", type = "board", parentId = "f1")))
        val writer = RecordingWriter()
        var opened: WorkspaceNode? = null
        var menuClosed = false

        presenterTestOf({
            WriteStateProducer(nodes, writer)(
                parentId = "f1",
                onCreated = { opened = it },
                onDeleted = {},
                onCreateChosen = { menuClosed = true },
            )
        }) {
            awaitItem().handle(WriteEvent.CreateBoard)
            nodes.settle()
            // A successful board create leaves the slice untouched — no draft, no error — so
            // there is no frame to await here. Run the launched write out on the test clock.
            testScheduler.advanceUntilIdle()

            assertEquals(listOf(Made("board", "f1", "Untitled board")), writer.calls)
            assertEquals("made", opened?.id)
            assertTrue(menuClosed, "choosing a kind closes the menu")
            cancelAndIgnoreRemainingEvents()
        }
    }

    /**
     * A folder goes straight into a rename instead of being opened — an untitled folder is
     * never what you wanted, and naming it is the next thing you would do.
     */
    @Test
    fun `new folder starts a rename rather than opening anything`() = runTest {
        val nodes = FakeNodeStore(listOf(folder("made", title = "New folder")))
        val writer = RecordingWriter()
        var opened: WorkspaceNode? = null

        presenterTestOf({
            WriteStateProducer(nodes, writer)(
                parentId = null,
                onCreated = { opened = it },
                onDeleted = {},
                onCreateChosen = {},
            )
        }) {
            awaitItem().handle(WriteEvent.CreateFolder)
            nodes.settle()

            val renaming = awaitUntil { it.rename != null }
            assertEquals("made", renaming.rename?.node?.id)
            assertNull(opened, "a new folder is named, not opened")
            // null parent is the section root, not a missing argument.
            assertEquals(listOf(Made("folder", null, "New folder")), writer.calls)
            cancelAndIgnoreRemainingEvents()
        }
    }
}
