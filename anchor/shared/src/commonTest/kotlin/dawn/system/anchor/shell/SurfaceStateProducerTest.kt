package dawn.system.anchor.shell

import com.slack.circuit.test.presenterTestOf
import dawn.system.anchor.domain.Destination
import dawn.system.anchor.domain.OpenItem
import dawn.system.anchor.domain.OpenItems
import dawn.system.anchor.features.shell.state.SurfaceStateProducer
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue
import kotlinx.coroutines.test.runTest

/**
 * The open set, as pager pages.
 *
 * The case that matters is the window before the rows have been read: an item is open, but
 * the store has not answered for it yet.
 */
class SurfaceStateProducerTest {

    private val pdf = artifact("pdf-1", type = "file", title = "Collars")
    private val openPdf = OpenItems().register(
        OpenItem(key = "folders:pdf-1", destination = Destination.Folders, nodeId = "pdf-1"),
    )

    @Test
    fun `an open artifact keeps its page while its row is still loading`() = runTest {
        val nodes = FakeNodeStore(listOf(pdf))
        val surfaces = SurfaceStateProducer(nodes, FakeResourceStore(), FakeDocumentStore())

        presenterTestOf({ surfaces(openPdf, activeNodeId = pdf.id) }) {
            val whileLoading = awaitItem()
            assertTrue(
                whileLoading.pages.isNotEmpty(),
                "an open item must hold its page while its row loads, not vanish and come back",
            )
            assertEquals(
                0,
                whileLoading.activePage,
                "and the pager must already be on it — the tap is known before the row is",
            )

            nodes.settle()

            val settled = awaitUntil { it.pages.isNotEmpty() && it.activePage >= 0 }
            assertEquals(1, settled.pages.size)
            cancelAndIgnoreRemainingEvents()
        }
    }

    private suspend fun <T> app.cash.turbine.ReceiveTurbine<T>.awaitUntil(
        limit: Int = 10,
        predicate: (T) -> Boolean,
    ): T {
        repeat(limit) {
            val item = awaitItem()
            if (predicate(item)) return item
        }
        error("no frame satisfied the condition within $limit")
    }
}
