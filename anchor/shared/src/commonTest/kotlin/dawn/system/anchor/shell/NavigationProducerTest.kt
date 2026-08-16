package dawn.system.anchor.shell

import app.cash.turbine.ReceiveTurbine
import com.slack.circuit.test.presenterTestOf
import dawn.system.anchor.domain.Destination
import dawn.system.anchor.domain.SidebarNav
import dawn.system.anchor.features.shell.state.NavigationStateProducer
import dawn.system.anchor.services.data.NodeState
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertIs
import kotlin.test.assertTrue
import kotlinx.coroutines.test.runTest

/**
 * The navigation producer, run as a real composition.
 *
 * This is the layer every regression this session came from, and none of it was reachable
 * while it lived inside an 800-line presenter. Each test below is a bug that shipped.
 */
class NavigationProducerTest {

    /**
     * Awaits the frame that satisfies [predicate].
     *
     * A composition emits whenever anything it reads changes, so the number of frames between
     * an action and its effect is an implementation detail — a store read landing, a
     * derivation settling. Asserting on "the next item" makes a test that fails when the
     * producer merely recomposes differently, which is not a bug. Waiting for the condition
     * tests the behaviour instead.
     */
    private suspend fun <T> ReceiveTurbine<T>.awaitUntil(
        limit: Int = 10,
        predicate: (T) -> Boolean,
    ): T {
        repeat(limit) {
            val item = awaitItem()
            if (predicate(item)) return item
        }
        error("no frame satisfied the condition within $limit")
    }

    private val insideF2 = SidebarNav()
        .openSection(Destination.Folders)
        .select(folder("f1"))
        .drillInto("f2")

    @Test
    fun `labels resolve from the store, so a rename reaches all of them`() = runTest {
        val nodes = FakeNodeStore(
            listOf(
                folder("f1", "Option strategies"),
                folder("f2", "Greeks deep-dive", parentId = "f1"),
            ),
        )
        val navigation = NavigationStateProducer(nodes)

        presenterTestOf({ navigation(insideF2, query = "") }) {
            nodes.settle()
            // Both rows of the path, not just the deepest: `combine` emits as each one lands,
            // so asserting on the first frame that has the title would race the other.
            val loaded = awaitUntil {
                it.title == "Greeks deep-dive" && it.backLabel == "Option strategies"
            }
            assertEquals("Option strategies", loaded.backLabel)

            // The bug this replaces: the title was copied into the path when you drilled in,
            // so renaming the folder you were inside left every label showing the old name.
            nodes.rename("f2", "Greeks, revised")

            val renamed = awaitUntil { it.title == "Greeks, revised" }
            assertEquals(
                "Greeks, revised",
                renamed.pathLevels.last().title,
                "the path popover reads the same source as the heading",
            )
            cancelAndIgnoreRemainingEvents()
        }
    }

    /**
     * The regression that reverted every drill: a producer that treats "not read yet" as
     * "deleted" tells the shell to climb out of the folder you just entered.
     */
    @Test
    fun `an unread folder is Loading, never Missing`() = runTest {
        val nodes = FakeNodeStore(listOf(folder("f1"), folder("f2", parentId = "f1")))
        val navigation = NavigationStateProducer(nodes)

        presenterTestOf({ navigation(insideF2, query = "") }) {
            val first = awaitItem()
            nodes.settle()
            assertTrue(
                first.folderState !is NodeState.Missing,
                "a folder that exists must never report Missing, got ${first.folderState}",
            )
            awaitUntil { it.folderState is NodeState.Present }
            cancelAndIgnoreRemainingEvents()
        }
    }

    @Test
    fun `a deleted folder reports Missing, which is what pops the sidebar`() = runTest {
        val nodes = FakeNodeStore(listOf(folder("f1"), folder("f2", parentId = "f1")))
        val navigation = NavigationStateProducer(nodes)

        presenterTestOf({ navigation(insideF2, query = "") }) {
            nodes.settle()
            awaitUntil { it.folderState is NodeState.Present }

            nodes.remove("f2")

            assertIs<NodeState.Missing>(awaitUntil { it.folderState is NodeState.Missing }.folderState)
            cancelAndIgnoreRemainingEvents()
        }
    }

    @Test
    fun `the search field narrows the rows it lists`() = runTest {
        val nodes = FakeNodeStore(
            listOf(folder("f1", "Semis cycle"), folder("f2", "Reading list")),
        )
        val navigation = NavigationStateProducer(nodes)
        val section = SidebarNav().openSection(Destination.Folders)

        presenterTestOf({ navigation(section, query = "semis") }) {
            val narrowed = awaitUntil { it.rows.isNotEmpty() }

            assertEquals(listOf("Semis cycle"), narrowed.rows.map { it.title })
            assertEquals("1 items", narrowed.count)
            cancelAndIgnoreRemainingEvents()
        }
    }
}
