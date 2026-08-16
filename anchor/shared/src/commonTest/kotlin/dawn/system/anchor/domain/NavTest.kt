package dawn.system.anchor.domain

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * The navigation model.
 *
 * Every rule here was previously spread across a sidebar stack, an open set, a side map of
 * positions, a correction effect and a pager index — and therefore reachable only by tapping
 * a device. That is why navigation could be deterministically broken and still ship.
 */
class NavTest {

    private fun doc(id: String) = TabKey.Artifact(id)
    private fun surfaceOf(nav: Nav) = nav.here.surface

    // ── identity ─────────────────────────────────────────────────────────────

    @Test
    fun `a key's string form matches the desktop's, because row ids are built from it`() {
        assertEquals("a1", doc("a1").asString())
        assertEquals(
            "research:r1:overview",
            TabKey.Research("r1", ResearchView.Overview).asString(),
        )
    }

    @Test
    fun `the same research folder at two views is two keys`() {
        val overview = TabKey.Research("r1", ResearchView.Overview)
        val manifest = TabKey.Research("r1", ResearchView.Manifest)
        val nav = Nav().openDoc(overview).openDoc(manifest)
        assertEquals(listOf(overview, manifest), nav.open)
    }

    // ── the trail ────────────────────────────────────────────────────────────

    @Test
    fun `back and forward are inverses`() {
        val nav = Nav().openDoc(doc("a")).openDoc(doc("b"))
        assertEquals(nav, nav.step(-1).step(1))
    }

    @Test
    fun `stepping past either end is a no-op, never a wrap`() {
        val nav = Nav().openDoc(doc("a"))
        assertEquals(nav, nav.step(1), "already at the end")
        assertEquals(nav.step(-1), nav.step(-1).step(-1), "already at the start")
    }

    @Test
    fun `re-pushing the position you are on leaves the trail alone`() {
        val nav = Nav().goTo(Destination.Markets)
        assertEquals(nav, nav.goTo(Destination.Markets))
    }

    @Test
    fun `back restores the whole entry, columns included`() {
        val nav = Nav().goToBrowser(listOf("f1"), "i1").goToBrowser(listOf("f2"), "i2")
        val back = nav.step(-1)
        assertEquals("f1", back.here.folderId)
        assertEquals("i1", back.here.itemId)
    }

    /**
     * The reason [Nav.entries] and [Nav.open] cannot be derived from each other.
     */
    @Test
    fun `navigating after stepping back truncates the trail but not the open set`() {
        val nav = Nav()
            .openDoc(doc("a")).openDoc(doc("b")).openDoc(doc("c"))
            .step(-1).step(-1)     // back to a
            .openDoc(doc("d"))

        assertEquals(
            listOf(Surface.Dest(Destination.Home), Surface.Doc(doc("a")), Surface.Doc(doc("d"))),
            nav.entries.map { it.surface },
            "b and c branched off the path",
        )
        assertEquals(
            listOf(doc("a"), doc("b"), doc("c"), doc("d")),
            nav.open,
            "but they are still open",
        )
        assertTrue(!nav.canForward, "the abandoned branch is not reachable by forward")
    }

    @Test
    fun `the trail is capped, keeping the most recent positions`() {
        var nav = Nav()
        repeat(60) {
            nav = nav.goTo(if (it % 2 == 0) Destination.Markets else Destination.Home)
        }
        assertEquals(Nav.TRAIL_CAP, nav.entries.size)
        assertEquals(Nav.TRAIL_CAP - 1, nav.index, "still standing on the newest position")
    }

    // ── the Browser's columns ────────────────────────────────────────────────

    @Test
    fun `selecting a column is not navigation, so it adds no history`() {
        val nav = Nav().goToBrowser(listOf("f1"), null)
        val after = nav.selectFolder(0, "f2").selectItem("i9")
        assertEquals(nav.entries.size, after.entries.size)
        assertEquals("f2", after.here.folderId)
        assertEquals("i9", after.here.itemId)
    }

    @Test
    fun `choosing a different folder drops the item, which does not live there`() {
        val nav = Nav().goToBrowser(listOf("f1"), "i1").selectFolder(0, "f2")
        assertNull(nav.here.itemId)
    }

    /** The defining Miller move: the columns to the right described a path you left. */
    @Test
    fun `picking in a shallower column discards every column to its right`() {
        val nav = Nav()
            .selectFolder(0, "a")
            .selectFolder(1, "b")
            .selectFolder(2, "c")
        assertEquals(listOf("a", "b", "c"), nav.here.path)

        val sideways = nav.selectFolder(1, "b2")
        assertEquals(
            listOf("a", "b2"),
            sideways.here.path,
            "c described a child of b, and you are no longer in b",
        )
    }

    @Test
    fun `descending appends a column, so the browser is as deep as the tree`() {
        var nav = Nav()
        repeat(6) { depth -> nav = nav.selectFolder(depth, "f$depth") }
        assertEquals(List(6) { "f$it" }, nav.here.path)
    }

    @Test
    fun `re-picking the folder you are already in changes nothing`() {
        val nav = Nav().selectFolder(0, "a").selectFolder(1, "b")
        assertEquals(nav, nav.selectFolder(1, "b"))
    }

    @Test
    fun `a deleted folder takes every column to its right with it`() {
        val nav = Nav()
            .selectFolder(0, "a").selectFolder(1, "b").selectFolder(2, "c")
            .selectItem("i1")
            .corrected(goneOnly("b"))

        assertEquals(listOf("a"), nav.here.path, "b's descendants are unreachable")
        assertNull(nav.here.itemId)
    }

    // ── the open set ─────────────────────────────────────────────────────────

    @Test
    fun `opening registers, activates and records recency in one move`() {
        val nav = Nav().openDoc(doc("a"))
        assertEquals(Surface.Doc(doc("a")), surfaceOf(nav))
        assertEquals(listOf(doc("a")), nav.open)
        assertEquals(listOf(doc("a")), nav.mru)
    }

    @Test
    fun `re-opening keeps its place in the list but becomes the most recent`() {
        val nav = Nav().openDoc(doc("a")).openDoc(doc("b")).openDoc(doc("a"))
        assertEquals(listOf(doc("a"), doc("b")), nav.open, "the list must not reorder")
        assertEquals(listOf(doc("a"), doc("b")), nav.mru, "recency must")
    }

    @Test
    fun `opening a document keeps the browser selection, so the crumb still jumps`() {
        val nav = Nav().goToBrowser(listOf("f1"), "i1").openDoc(doc("a"))
        assertEquals("f1", nav.here.folderId)
        assertEquals("i1", nav.here.itemId)
    }

    // ── closing ──────────────────────────────────────────────────────────────

    @Test
    fun `closing what you are showing lands on the trail, not on the most recent`() {
        val nav = Nav().openDoc(doc("a")).openDoc(doc("b")).openDoc(doc("c"))
        val after = nav.closeDoc(doc("c"))
        assertEquals(Surface.Doc(doc("b")), surfaceOf(after))
        assertEquals(listOf(doc("a"), doc("b")), after.open)
    }

    @Test
    fun `closing something else leaves you exactly where you were`() {
        val nav = Nav().openDoc(doc("a")).openDoc(doc("b"))
        val after = nav.closeDoc(doc("a"))
        assertEquals(Surface.Doc(doc("b")), surfaceOf(after))
    }

    @Test
    fun `a closed document is pruned from the trail, so back can never reach it`() {
        val nav = Nav().openDoc(doc("a")).openDoc(doc("b")).closeDoc(doc("a"))
        assertTrue(
            nav.entries.none { it.surface == Surface.Doc(doc("a")) },
            "back must not land on something you closed",
        )
    }

    @Test
    fun `when nothing in the trail survives, the most recent survivor takes over`() {
        val nav = Nav(
            entries = listOf(Entry(Surface.Doc(doc("a")))),
            index = 0,
            open = listOf(doc("a"), doc("b")),
            mru = listOf(doc("a"), doc("b")),
        )
        assertEquals(Surface.Doc(doc("b")), surfaceOf(nav.closeDoc(doc("a"))))
    }

    @Test
    fun `when nothing survives at all, it falls back to Home`() {
        val nav = Nav(
            entries = listOf(Entry(Surface.Doc(doc("a")))),
            index = 0,
            open = listOf(doc("a")),
            mru = listOf(doc("a")),
        )
        assertEquals(Surface.Dest(Destination.Home), surfaceOf(nav.closeDoc(doc("a"))))
    }

    @Test
    fun `closing something that is not open changes nothing`() {
        val nav = Nav().openDoc(doc("a"))
        assertEquals(nav, nav.closeDoc(doc("zzz")))
    }

    // ── the switcher ─────────────────────────────────────────────────────────

    @Test
    fun `the switcher orders by recency, but the open list decides membership`() {
        val nav = Nav(
            open = listOf(doc("a"), doc("b"), doc("c")),
            mru = listOf(doc("c"), doc("gone")),
        )
        assertEquals(
            listOf(doc("c"), doc("a"), doc("b")),
            nav.switcherOrder(),
            "a stale recency entry is dropped; documents missing from it append in open order",
        )
    }

    /**
     * Why cycling is not a `Nav` operation. Opening promotes, so a step would reorder the
     * list being stepped through — the switcher must take this order **once** and hold it.
     */
    @Test
    fun `opening reorders recency, which is why a switcher must freeze this order`() {
        val nav = Nav().openDoc(doc("a")).openDoc(doc("b")).openDoc(doc("c"))
        assertEquals(listOf(doc("c"), doc("b"), doc("a")), nav.switcherOrder())

        val stepped = nav.openDoc(doc("b"))
        assertEquals(
            listOf(doc("b"), doc("c"), doc("a")),
            stepped.switcherOrder(),
            "one step moved b to the head — a second step would advance from a different list",
        )
    }

    // ── correction ───────────────────────────────────────────────────────────

    private fun goneOnly(vararg ids: String): (String) -> Presence =
        { id -> if (id in ids) Presence.Gone else Presence.There }

    /** The bug that shipped twice: an unread row is not a deleted one. */
    @Test
    fun `an unread id moves nothing`() {
        val nav = Nav().goToBrowser(listOf("f1"), "i1").openDoc(doc("a"))
        assertEquals(nav, nav.corrected { Presence.Unknown })
    }

    @Test
    fun `correction is idempotent, so applying it every frame is free`() {
        val nav = Nav().goToBrowser(listOf("f1"), "i1").openDoc(doc("a"))
        val once = nav.corrected(goneOnly("f1"))
        assertEquals(once, once.corrected(goneOnly("f1")))
    }

    @Test
    fun `a deleted folder stops being the selection, and takes its item with it`() {
        val nav = Nav().goToBrowser(listOf("f1"), "i1").corrected(goneOnly("f1"))
        assertNull(nav.here.folderId)
        assertNull(nav.here.itemId, "an item cannot still be inside a folder that is gone")
    }

    @Test
    fun `a deleted item leaves the folder alone`() {
        val nav = Nav().goToBrowser(listOf("f1"), "i1").corrected(goneOnly("i1"))
        assertEquals("f1", nav.here.folderId)
        assertNull(nav.here.itemId)
    }

    @Test
    fun `a deleted document stops being open and is pruned from the trail`() {
        val nav = Nav().openDoc(doc("a")).openDoc(doc("b")).corrected(goneOnly("b"))
        assertEquals(listOf(doc("a")), nav.open)
        assertTrue(nav.entries.none { it.surface == Surface.Doc(doc("b")) })
        assertEquals(Surface.Doc(doc("a")), surfaceOf(nav))
    }

    /**
     * The property a write-back correction cannot have: the sync snapshot window transiently
     * removes rows, and the position must come back on its own when they return.
     */
    @Test
    fun `a row that comes back un-corrects, because nothing was destroyed`() {
        val nav = Nav().goToBrowser(listOf("f1"), "i1")
        val duringSnapshot = nav.corrected(goneOnly("f1"))
        assertNull(duringSnapshot.here.folderId)
        // The raw value was never written to, so the next frame is simply right again.
        assertEquals("f1", nav.corrected { Presence.There }.here.folderId)
    }

    @Test
    fun `back walks the trail while recency walks the open set, and they disagree`() {
        val nav = Nav()
            .openDoc(doc("a")).openDoc(doc("b")).openDoc(doc("c"))
            .step(-1).step(-1)          // back to a; the trail still holds b and c
            .openDoc(doc("d"))          // truncates b and c out of the trail

        assertEquals(Surface.Doc(doc("a")), surfaceOf(nav.step(-1)), "Back walks the path")
        assertTrue(
            nav.switcherOrder().containsAll(listOf(doc("b"), doc("c"))),
            "but recency still reaches documents the trail has forgotten",
        )
    }
}
