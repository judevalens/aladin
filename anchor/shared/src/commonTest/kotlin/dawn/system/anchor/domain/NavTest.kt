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
    private fun tab(id: String) = OpenTab.Doc(TabKey.Artifact(id))
    private fun tabs(vararg ids: String) = ids.map(::tab)
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
        assertEquals(listOf(OpenTab.Doc(overview), OpenTab.Doc(manifest)), nav.open)
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
        val nav = Nav().goToBrowser(listOf("f1", "i1")).goToBrowser(listOf("f2", "i2"))
        assertEquals(listOf("f1", "i1"), nav.step(-1).here.path)
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
            tabs("a", "b", "c", "d"),
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
        val nav = Nav().goToBrowser(listOf("f1"))
        val after = nav.select(0, "f2").select(1, "i9")
        assertEquals(nav.entries.size, after.entries.size)
        assertEquals(listOf("f2", "i9"), after.here.path)
    }

    @Test
    fun `choosing a different folder drops the item, which does not live there`() {
        val nav = Nav().goToBrowser(listOf("f1", "i1")).select(0, "f2")
        assertEquals(listOf("f2"), nav.here.path)
    }

    /**
     * The rule a separate `itemId` could not express, and the reason the leaf lives in the
     * path: the prototype has ONE row handler (`:738`), so picking a leaf in a shallow column
     * collapses the deep columns exactly as picking a folder there would.
     */
    @Test
    fun `picking a leaf in a shallower column also discards the columns to its right`() {
        val nav = Nav()
            .select(0, "a").select(1, "b").select(2, "leaf")
        assertEquals(listOf("a", "b", "leaf"), nav.here.path)

        assertEquals(
            listOf("other-leaf"),
            nav.select(0, "other-leaf").here.path,
            "a leaf is not a special case; it truncates like anything else",
        )
    }

    /** The defining Miller move: the columns to the right described a path you left. */
    @Test
    fun `picking in a shallower column discards every column to its right`() {
        val nav = Nav()
            .select(0, "a")
            .select(1, "b")
            .select(2, "c")
        assertEquals(listOf("a", "b", "c"), nav.here.path)

        val sideways = nav.select(1, "b2")
        assertEquals(
            listOf("a", "b2"),
            sideways.here.path,
            "c described a child of b, and you are no longer in b",
        )
    }

    @Test
    fun `descending appends a column, so the browser is as deep as the tree`() {
        var nav = Nav()
        repeat(6) { depth -> nav = nav.select(depth, "f$depth") }
        assertEquals(List(6) { "f$it" }, nav.here.path)
    }

    @Test
    fun `re-picking the folder you are already in changes nothing`() {
        val nav = Nav().select(0, "a").select(1, "b")
        assertEquals(nav, nav.select(1, "b"))
    }

    @Test
    fun `a deleted folder takes every column to its right with it`() {
        val nav = Nav()
            .select(0, "a").select(1, "b").select(2, "c").select(3, "i1")
            .corrected(goneOnly("b"))

        assertEquals(listOf("a"), nav.here.path, "b's descendants are unreachable")
    }

    // ── the open set ─────────────────────────────────────────────────────────

    @Test
    fun `opening registers, activates and records recency in one move`() {
        val nav = Nav().openDoc(doc("a"))
        assertEquals(Surface.Doc(doc("a")), surfaceOf(nav))
        assertEquals(tabs("a"), nav.open)
        assertEquals(tabs("a"), nav.mru)
    }

    @Test
    fun `re-opening keeps its place in the list but becomes the most recent`() {
        val nav = Nav().openDoc(doc("a")).openDoc(doc("b")).openDoc(doc("a"))
        assertEquals(tabs("a", "b"), nav.open, "the list must not reorder")
        assertEquals(tabs("a", "b"), nav.mru, "recency must")
    }

    @Test
    fun `opening a document keeps the browser selection, so the crumb still jumps`() {
        val nav = Nav().goToBrowser(listOf("f1", "i1")).openDoc(doc("a"))
        assertEquals(listOf("f1", "i1"), nav.here.path)
    }

    // ── closing ──────────────────────────────────────────────────────────────

    @Test
    fun `closing what you are showing lands on the trail, not on the most recent`() {
        val nav = Nav().openDoc(doc("a")).openDoc(doc("b")).openDoc(doc("c"))
        val after = nav.close(tab("c"))
        assertEquals(Surface.Doc(doc("b")), surfaceOf(after))
        assertEquals(tabs("a", "b"), after.open)
    }

    @Test
    fun `closing something else leaves you exactly where you were`() {
        val nav = Nav().openDoc(doc("a")).openDoc(doc("b"))
        val after = nav.close(tab("a"))
        assertEquals(Surface.Doc(doc("b")), surfaceOf(after))
    }

    @Test
    fun `a closed document is pruned from the trail, so back can never reach it`() {
        val nav = Nav().openDoc(doc("a")).openDoc(doc("b")).close(tab("a"))
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
            open = tabs("a", "b"),
            mru = tabs("a", "b"),
        )
        assertEquals(Surface.Doc(doc("b")), surfaceOf(nav.close(tab("a"))))
    }

    @Test
    fun `when nothing survives at all, it falls back to Home`() {
        val nav = Nav(
            entries = listOf(Entry(Surface.Doc(doc("a")))),
            index = 0,
            open = tabs("a"),
            mru = tabs("a"),
        )
        assertEquals(Surface.Dest(Destination.Home), surfaceOf(nav.close(tab("a"))))
    }

    @Test
    fun `closing something that is not open changes nothing`() {
        val nav = Nav().openDoc(doc("a"))
        assertEquals(nav, nav.close(tab("zzz")))
    }

    // ── the browser tab ──────────────────────────────────────────────────────

    /**
     * **The reason the browser is not a [TabKey].**
     *
     * A key names a row in the tree, so a key is a thing the store gets asked about — and the
     * store answers null for an id it has no row for, which reads as [Presence.Gone]. Modelled
     * as a key, the browser tab would be closed by the correction on the frame after it opened,
     * and the bug would look like "the tab won't stay open" rather than like a type mistake.
     */
    @Test
    fun `correction never closes the browser tab, whatever the tree says`() {
        val nav = Nav().openDoc(doc("a")).openBrowserTab()

        val corrected = nav.corrected { Presence.Gone }

        assertEquals(
            listOf(OpenTab.Browser),
            corrected.open,
            "the document went, as it should; the browser stayed, having no row to lose",
        )
        assertEquals(Surface.Browser, surfaceOf(corrected))
    }

    @Test
    fun `promoting the browser opens a tab, activates it and records recency`() {
        val nav = Nav().openDoc(doc("a")).openBrowserTab()

        assertEquals(Surface.Browser, surfaceOf(nav))
        assertEquals(listOf(tab("a"), OpenTab.Browser), nav.open)
        assertEquals(listOf(OpenTab.Browser, tab("a")), nav.mru)
        assertTrue(nav.hasBrowserTab)
    }

    /**
     * The two doors are different: the toggle promotes, the breadcrumb only shows. Conflating
     * them would grow an Open row every time you tapped an ancestor crumb.
     */
    @Test
    fun `a breadcrumb jump shows the browser without opening a tab`() {
        val nav = Nav().openDoc(doc("a")).goToBrowser(listOf("f1"))

        assertEquals(Surface.Browser, surfaceOf(nav))
        assertTrue(!nav.hasBrowserTab, "showing the columns is not promoting them")
        assertEquals(tabs("a"), nav.open)
    }

    @Test
    fun `returning to an open browser tab keeps its place but becomes the most recent`() {
        val nav = Nav().openBrowserTab().openDoc(doc("a")).openBrowserTab()

        assertEquals(listOf(OpenTab.Browser, tab("a")), nav.open, "the list must not reorder")
        assertEquals(listOf(OpenTab.Browser, tab("a")), nav.mru)
    }

    @Test
    fun `closing the browser tab prunes the trail exactly as a document does`() {
        val nav = Nav().openDoc(doc("a")).openBrowserTab()
        val after = nav.close(OpenTab.Browser)

        assertEquals(Surface.Doc(doc("a")), surfaceOf(after))
        assertEquals(tabs("a"), after.open)
        assertTrue(after.entries.none { it.surface == Surface.Browser })
    }

    // ── the switcher ─────────────────────────────────────────────────────────

    @Test
    fun `the switcher orders by recency, but the open list decides membership`() {
        val nav = Nav(
            open = tabs("a", "b", "c"),
            mru = tabs("c", "gone"),
        )
        assertEquals(
            tabs("c", "a", "b"),
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
        assertEquals(tabs("c", "b", "a"), nav.switcherOrder())

        val stepped = nav.openDoc(doc("b"))
        assertEquals(
            tabs("b", "c", "a"),
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
        val nav = Nav().goToBrowser(listOf("f1", "i1")).openDoc(doc("a"))
        assertEquals(nav, nav.corrected { Presence.Unknown })
    }

    @Test
    fun `correction is idempotent, so applying it every frame is free`() {
        val nav = Nav().goToBrowser(listOf("f1", "i1")).openDoc(doc("a"))
        val once = nav.corrected(goneOnly("f1"))
        assertEquals(once, once.corrected(goneOnly("f1")))
    }

    @Test
    fun `a deleted folder stops being the selection, and takes its item with it`() {
        val nav = Nav().goToBrowser(listOf("f1", "i1")).corrected(goneOnly("f1"))
        assertEquals(
            emptyList<String>(),
            nav.here.path,
            "an item cannot still be inside a folder that is gone",
        )
    }

    @Test
    fun `a deleted item leaves the folder alone`() {
        val nav = Nav().goToBrowser(listOf("f1", "i1")).corrected(goneOnly("i1"))
        assertEquals(listOf("f1"), nav.here.path)
    }

    @Test
    fun `a deleted document stops being open and is pruned from the trail`() {
        val nav = Nav().openDoc(doc("a")).openDoc(doc("b")).corrected(goneOnly("b"))
        assertEquals(tabs("a"), nav.open)
        assertTrue(nav.entries.none { it.surface == Surface.Doc(doc("b")) })
        assertEquals(Surface.Doc(doc("a")), surfaceOf(nav))
    }

    /**
     * The property a write-back correction cannot have: the sync snapshot window transiently
     * removes rows, and the position must come back on its own when they return.
     */
    @Test
    fun `a row that comes back un-corrects, because nothing was destroyed`() {
        val nav = Nav().goToBrowser(listOf("f1", "i1"))
        val duringSnapshot = nav.corrected(goneOnly("f1"))
        assertEquals(emptyList<String>(), duringSnapshot.here.path)
        // The raw value was never written to, so the next frame is simply right again.
        assertEquals(listOf("f1", "i1"), nav.corrected { Presence.There }.here.path)
    }

    @Test
    fun `back walks the trail while recency walks the open set, and they disagree`() {
        val nav = Nav()
            .openDoc(doc("a")).openDoc(doc("b")).openDoc(doc("c"))
            .step(-1).step(-1)          // back to a; the trail still holds b and c
            .openDoc(doc("d"))          // truncates b and c out of the trail

        assertEquals(Surface.Doc(doc("a")), surfaceOf(nav.step(-1)), "Back walks the path")
        assertTrue(
            nav.switcherOrder().containsAll(tabs("b", "c")),
            "but recency still reaches documents the trail has forgotten",
        )
    }
}
