package dawn.system.anchor.shell

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import app.cash.turbine.ReceiveTurbine
import com.slack.circuit.test.presenterTestOf
import dawn.system.anchor.domain.ArtifactKind
import dawn.system.anchor.domain.BrowserFilter
import dawn.system.anchor.domain.BrowserTree
import dawn.system.anchor.domain.Entry
import dawn.system.anchor.domain.Nav
import dawn.system.anchor.domain.NodeKind
import dawn.system.anchor.domain.ResearchView
import dawn.system.anchor.domain.Surface
import dawn.system.anchor.domain.TabKey
import dawn.system.anchor.domain.WorkspaceNode
import dawn.system.anchor.features.shell.state.TREE_DEPTH_CAP
import dawn.system.anchor.features.shell.state.TreeEvent
import dawn.system.anchor.features.shell.state.TreeStateProducer
import dawn.system.anchor.features.shell.state.flattenTree
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue
import kotlinx.coroutines.test.runTest

/**
 * The sidebar tree: a pure flatten, and a producer that remembers only ids.
 *
 * The flatten is tested without a composition because the order and the depth cap are facts
 * about data; the producer is tested as a real composition because reveal-on-open and the
 * create target are facts about frames.
 */
class TreeStateProducerTest {

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

    private fun research(id: String, title: String = id, parentId: String? = null) =
        WorkspaceNode(id = id, kind = NodeKind.Research, parentId = parentId, position = 0, title = title)

    /** `zeta`, `alpha › (mid › (deep-leaf)), note-a`, `empty`, plus a research folder. */
    private val nodes = listOf(
        folder("zeta", "Zeta"),
        folder("alpha", "Alpha"),
        folder("mid", "Mid", parentId = "alpha"),
        artifact("deep-leaf", type = "file", title = "Deep", parentId = "mid"),
        artifact("note-a", type = "page", title = "Note A", parentId = "alpha"),
        folder("empty", "Empty"),
        research("lab", "Lab"),
        artifact("lab-note", type = "page", title = "Lab note", parentId = "lab"),
    )

    // ── flatten ──────────────────────────────────────────────────────────────

    @Test
    fun `roots come folders A-Z then leaves, and children follow their parent one level deeper`() {
        val tree = BrowserTree(nodes)
        val collapsed = flattenTree(tree, emptySet(), BrowserFilter(), activeId = null)
        assertEquals(listOf("alpha", "empty", "lab", "zeta"), collapsed.map { it.id })
        assertTrue(collapsed.all { it.depth == 0 })

        val open = flattenTree(tree, setOf("alpha"), BrowserFilter(), activeId = null)
        assertEquals(listOf("alpha", "mid", "note-a", "empty", "lab", "zeta"), open.map { it.id })
        assertEquals(listOf(0, 1, 1, 0, 0, 0), open.map { it.depth })
        assertTrue(open.first { it.id == "alpha" }.expanded)
        assertFalse(open.first { it.id == "mid" }.expanded)
    }

    @Test
    fun `an empty folder is listed unfiltered, so the one you just made is reachable`() {
        val rows = flattenTree(BrowserTree(nodes), emptySet(), BrowserFilter(), activeId = null)
        val empty = rows.first { it.id == "empty" }
        assertFalse(empty.hasChildren)
        assertEquals(0, empty.count)
    }

    @Test
    fun `a filter drops folders nothing matches inside, and counts what survives`() {
        val onlyNotes = BrowserFilter(kinds = setOf(ArtifactKind.Page))
        val rows = flattenTree(BrowserTree(nodes), setOf("alpha", "mid"), onlyNotes, activeId = null)
        // `mid` holds only a file and `empty` holds nothing: both gone; alpha keeps note-a.
        assertEquals(listOf("alpha", "note-a", "lab"), rows.map { it.id })
        assertEquals(1, rows.first { it.id == "alpha" }.count)
    }

    @Test
    fun `depth is capped in the builder, chevrons still carry the hierarchy`() {
        val chain = listOf(
            folder("d0"), folder("d1", parentId = "d0"), folder("d2", parentId = "d1"),
            folder("d3", parentId = "d2"), folder("d4", parentId = "d3"), folder("d5", parentId = "d4"),
            artifact("leaf", parentId = "d5"),
        )
        val all = setOf("d0", "d1", "d2", "d3", "d4", "d5")
        val rows = flattenTree(BrowserTree(chain), all, BrowserFilter(), activeId = null)
        assertEquals(listOf(0, 1, 2, 3, 4, 4, 4), rows.map { it.depth })
        assertEquals(TREE_DEPTH_CAP, rows.last().depth)
        assertTrue(rows.first { it.id == "d5" }.expanded)
    }

    @Test
    fun `only a research folder is research — its plain subfolders are not`() {
        val withSub = nodes + folder("drawer", "Drawer", parentId = "lab")
        val rows = flattenTree(BrowserTree(withSub), setOf("lab"), BrowserFilter(), activeId = "lab")
        assertTrue(rows.first { it.id == "lab" }.isResearch)
        assertTrue(rows.first { it.id == "lab" }.active, "its Overview is the open document")
        assertFalse(rows.first { it.id == "drawer" }.isResearch)
        assertEquals("lab", rows.first { it.id == "lab-note" }.parentId)
    }

    // ── producer ─────────────────────────────────────────────────────────────

    @Test
    fun `toggling expands, collapses, and focuses the folder`() = runTest {
        val store = FakeNodeStore(nodes)
        presenterTestOf({ TreeStateProducer(store)(Nav(), BrowserFilter()) }) {
            val start = awaitUntil { it.rows.isNotEmpty() }
            assertNull(start.focusedFolderId)

            start.handle(TreeEvent.ToggleFolder("alpha"))
            // Await the condition, not a frame: immediate recomposition emits per write.
            val opened = awaitUntil { it.rows.any { r -> r.id == "mid" } && it.focusedFolderId == "alpha" }
            assertTrue(opened.rows.first { it.id == "alpha" }.expanded)

            opened.handle(TreeEvent.ToggleFolder("alpha"))
            val closed = awaitUntil { it.rows.none { r -> r.id == "mid" } }
            assertEquals("alpha", closed.focusedFolderId, "collapsing keeps the focus")
            cancelAndIgnoreRemainingEvents()
        }
    }

    @Test
    fun `opening a document reveals its ancestors once, and a later collapse sticks`() = runTest {
        val store = FakeNodeStore(nodes)
        var nav by mutableStateOf(Nav())
        presenterTestOf({ TreeStateProducer(store)(nav, BrowserFilter()) }) {
            awaitUntil { it.rows.isNotEmpty() }
            nav = nav.openDoc(TabKey.Artifact("deep-leaf"))
            store.settle()

            val revealed = awaitUntil { it.rows.any { r -> r.id == "deep-leaf" } }
            assertTrue(revealed.rows.first { it.id == "alpha" }.expanded)
            assertTrue(revealed.rows.first { it.id == "mid" }.expanded)
            assertTrue(revealed.rows.first { it.id == "deep-leaf" }.active)

            revealed.handle(TreeEvent.ToggleFolder("mid"))
            val collapsed = awaitUntil { it.rows.none { r -> r.id == "deep-leaf" } }
            assertFalse(collapsed.rows.first { it.id == "mid" }.expanded)
            cancelAndIgnoreRemainingEvents()
        }
    }

    @Test
    fun `the create target follows focus, then the open document's folder, then the Browser, then root`() = runTest {
        val store = FakeNodeStore(nodes)
        var nav by mutableStateOf(Nav())
        presenterTestOf({ TreeStateProducer(store)(nav, BrowserFilter()) }) {
            store.settle()
            val root = awaitUntil { it.rows.isNotEmpty() && it.createTarget.title == "Workspace" }
            assertNull(root.createTarget.folderId)

            // The Browser standing in a folder — today's rule, as the last resort.
            nav = Nav(entries = listOf(Entry(Surface.Browser, listOf("zeta"))))
            val browsing = awaitUntil { it.createTarget.folderId == "zeta" }
            assertEquals("Zeta", browsing.createTarget.title)

            // An open document's folder beats the Browser's column.
            nav = nav.openDoc(TabKey.Artifact("deep-leaf"))
            val reading = awaitUntil { it.createTarget.folderId == "mid" }
            assertEquals("Mid", reading.createTarget.title)

            // A research Overview targets the research folder itself.
            nav = nav.openDoc(TabKey.Research("lab", ResearchView.Overview))
            val lab = awaitUntil { it.createTarget.folderId == "lab" }

            // Focus beats everything.
            lab.handle(TreeEvent.FocusFolder("alpha"))
            val focused = awaitUntil { it.createTarget.folderId == "alpha" }
            assertEquals("Alpha", focused.createTarget.title)

            // A deleted focused folder falls through to the next rule.
            store.remove("alpha")
            awaitUntil { it.createTarget.folderId == "lab" }
            cancelAndIgnoreRemainingEvents()
        }
    }
}
