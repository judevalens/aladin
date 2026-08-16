package dawn.system.anchor.shell

import app.cash.turbine.ReceiveTurbine
import com.slack.circuit.test.presenterTestOf
import dawn.system.anchor.domain.Surface
import dawn.system.anchor.domain.TabKey
import dawn.system.anchor.features.shell.state.NavEvent
import dawn.system.anchor.features.shell.state.NavStateProducer
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue
import kotlinx.coroutines.test.runTest

/**
 * The navigation producer as a real composition.
 *
 * The point of these is that correction is a *derivation*: there is no effect to wait for, so
 * a deletion shows up on the next frame and a restoration shows up on the frame after that.
 * None of this was reachable while the rule lived in a `LaunchedEffect` inside the presenter.
 */
class NavProducerTest {

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

    private fun doc(id: String) = TabKey.Artifact(id)

    @Test
    fun `opening a document makes it the surface and puts it in the open set`() = runTest {
        val nodes = FakeNodeStore(listOf(artifact("a1")))
        presenterTestOf({ NavStateProducer(nodes)() }) {
            awaitItem().handle(NavEvent.OpenDoc(doc("a1")))
            nodes.settle()

            val opened = awaitUntil { it.nav.activeDoc == doc("a1") }
            assertEquals(listOf(doc("a1")), opened.nav.open)
            cancelAndIgnoreRemainingEvents()
        }
    }

    /**
     * The regression the whole rebuild exists for: an unread row must not be read as a
     * deleted one, or the position you just moved to is undone before you see it.
     */
    @Test
    fun `an unread row never moves the position`() = runTest {
        val nodes = FakeNodeStore(listOf(folder("f1")))
        presenterTestOf({ NavStateProducer(nodes)() }) {
            // Deliberately do NOT settle: nothing has been read.
            awaitItem().handle(NavEvent.GoToBrowser(listOf("f1"), null))

            val browsing = awaitUntil { it.nav.here.folderId != null }
            assertEquals("f1", browsing.nav.here.folderId)
            cancelAndIgnoreRemainingEvents()
        }
    }

    @Test
    fun `a deleted folder stops being the selection`() = runTest {
        val nodes = FakeNodeStore(listOf(folder("f1")))
        presenterTestOf({ NavStateProducer(nodes)() }) {
            awaitItem().handle(NavEvent.GoToBrowser(listOf("f1"), null))
            nodes.settle()
            awaitUntil { it.nav.here.folderId == "f1" }

            nodes.remove("f1")

            assertNull(awaitUntil { it.nav.here.folderId == null }.nav.here.folderId)
            cancelAndIgnoreRemainingEvents()
        }
    }

    /**
     * A paged snapshot transiently deletes rows it has not re-sent yet. A write-back
     * correction destroys the position on that frame; a derivation simply reports the truth
     * of each frame, so the position returns with the row.
     */
    @Test
    fun `a row that comes back restores the position on its own`() = runTest {
        val nodes = FakeNodeStore(listOf(folder("f1")))
        presenterTestOf({ NavStateProducer(nodes)() }) {
            awaitItem().handle(NavEvent.GoToBrowser(listOf("f1"), null))
            nodes.settle()
            awaitUntil { it.nav.here.folderId == "f1" }

            nodes.remove("f1")
            awaitUntil { it.nav.here.folderId == null }

            nodes.put(folder("f1"))

            assertEquals(
                "f1",
                awaitUntil { it.nav.here.folderId == "f1" }.nav.here.folderId,
                "nothing was destroyed, so the next frame is simply right again",
            )
            cancelAndIgnoreRemainingEvents()
        }
    }

    @Test
    fun `a deleted document is closed and pruned from the trail`() = runTest {
        val nodes = FakeNodeStore(listOf(artifact("a1"), artifact("a2")))
        presenterTestOf({ NavStateProducer(nodes)() }) {
            awaitItem().handle(NavEvent.OpenDoc(doc("a1")))
            nodes.settle()
            awaitUntil { it.nav.open == listOf(doc("a1")) }.handle(NavEvent.OpenDoc(doc("a2")))
            awaitUntil { it.nav.open.size == 2 }

            nodes.remove("a2")

            val settled = awaitUntil { it.nav.open == listOf(doc("a1")) }
            assertTrue(settled.nav.entries.none { it.surface == Surface.Doc(doc("a2")) })
            assertEquals(Surface.Doc(doc("a1")), settled.nav.here.surface)
            cancelAndIgnoreRemainingEvents()
        }
    }

    @Test
    fun `back returns to the previous position`() = runTest {
        val nodes = FakeNodeStore(listOf(artifact("a1")))
        presenterTestOf({ NavStateProducer(nodes)() }) {
            nodes.settle()
            awaitItem().handle(NavEvent.OpenDoc(doc("a1")))
            val opened = awaitUntil { it.nav.activeDoc == doc("a1") }
            assertTrue(opened.nav.canBack)

            opened.handle(NavEvent.Back)

            assertNull(awaitUntil { it.nav.activeDoc == null }.nav.activeDoc)
            cancelAndIgnoreRemainingEvents()
        }
    }
}
