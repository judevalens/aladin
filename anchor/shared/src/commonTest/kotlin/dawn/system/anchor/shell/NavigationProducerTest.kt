package dawn.system.anchor.shell

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import app.cash.turbine.ReceiveTurbine
import com.slack.circuit.test.presenterTestOf
import dawn.system.anchor.domain.Destination
import dawn.system.anchor.domain.SidebarNav
import dawn.system.anchor.features.shell.state.NavigationStateProducer
import dawn.system.anchor.domain.Presence
import kotlin.test.Test
import kotlin.test.assertEquals
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
    fun `an unread folder is Unknown, never Gone`() = runTest {
        val nodes = FakeNodeStore(listOf(folder("f1"), folder("f2", parentId = "f1")))
        val navigation = NavigationStateProducer(nodes)

        presenterTestOf({ navigation(insideF2, query = "") }) {
            val first = awaitItem()
            nodes.settle()
            assertTrue(
                first.folderPresence != Presence.Gone,
                "a folder that exists must never report Gone, got ${first.folderPresence}",
            )
            awaitUntil { it.folderPresence == Presence.There }
            cancelAndIgnoreRemainingEvents()
        }
    }

    /**
     * The drill-reverting bug, at its source.
     *
     * `collectAsState` is `produceState`, whose backing `remember { mutableStateOf(initial) }`
     * has no keys — so a changed flow keeps the *previous* flow's value and `initial` never
     * applies again. At section level `folderId` is null and the producer's `?: flowOf(Missing)`
     * branch emits a **genuine** Missing; without a `key`, that read survived into the frame
     * after a drill and was attributed to the folder just entered. `corrected()` then saw
     * `Gone` and called `back()`, so no folder could ever be entered.
     *
     * Frames are matched on [NavigationSlice.searchPlaceholder] because it is derived from the
     * level alone — unlike the title, it does not wait on a store read to become meaningful.
     */
    @Test
    fun `drilling in does not inherit the section level's read`() = runTest {
        val nodes = FakeNodeStore(listOf(folder("f1"), folder("f2", parentId = "f1")))
        val navigation = NavigationStateProducer(nodes)
        var nav by mutableStateOf(SidebarNav().openSection(Destination.Folders))

        presenterTestOf({ navigation(nav, query = "") }) {
            nodes.settle()
            // At section level there is no folder, so Missing here is correct and expected.
            awaitUntil { it.folderPresence == Presence.Gone }

            nav = insideF2

            var resolved = false
            repeat(10) {
                if (resolved) return@repeat
                val frame = awaitItem()
                // Ignore any trailing frame still describing the section level.
                if (frame.searchPlaceholder != "Search this folder") return@repeat
                assertTrue(
                    frame.folderPresence != Presence.Gone,
                    "a drilled folder that exists must never read Gone — that is the " +
                        "section level's answer leaking across the key change",
                )
                if (frame.folderPresence == Presence.There) resolved = true
            }
            assertTrue(resolved, "the drilled folder never resolved to Present")
            cancelAndIgnoreRemainingEvents()
        }
    }

    @Test
    fun `a deleted folder reports Gone, which is what pops the sidebar`() = runTest {
        val nodes = FakeNodeStore(listOf(folder("f1"), folder("f2", parentId = "f1")))
        val navigation = NavigationStateProducer(nodes)

        presenterTestOf({ navigation(insideF2, query = "") }) {
            nodes.settle()
            awaitUntil { it.folderPresence == Presence.There }

            nodes.remove("f2")

            assertEquals(Presence.Gone, awaitUntil { it.folderPresence == Presence.Gone }.folderPresence)
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
