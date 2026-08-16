package dawn.system.anchor.domain

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * The open-items rules from the handoff. These are the behaviours a user would notice
 * immediately if they broke — a list that reorders under you, a close that strands you on
 * nothing, a cap that drops the thing you just opened.
 */
class OpenItemsTest {

    private fun item(id: String) = OpenItem(
        key = "folders:$id",
        destination = Destination.Folders,
        nodeId = id,
    )

    @Test
    fun registering_activates_and_appends() {
        val items = OpenItems().register(item("a")).register(item("b"))

        assertEquals(listOf("a", "b"), items.items.map { it.nodeId })
        assertEquals("folders:b", items.activeKey)
        assertEquals(2, items.activeOrdinal)
    }

    @Test
    fun re_registering_activates_in_place_without_reordering() {
        val items = OpenItems().register(item("a")).register(item("b")).register(item("a"))

        assertEquals(listOf("a", "b"), items.items.map { it.nodeId }, "order must not shuffle")
        assertEquals("folders:a", items.activeKey)
    }

    /**
     * Opening things must never close other things. Memory is bounded by *residency*
     * (KeepAlive), which drops a surface's view and rebinds it from its snapshot — the way
     * a list recycler rebinds a row. Capping this list instead would discard the user's
     * statement of intent to save memory that is already accounted for.
     */
    @Test
    fun opening_many_never_closes_the_earlier_ones() {
        var items = OpenItems()
        repeat(30) { i -> items = items.register(item("n$i")) }

        assertEquals(30, items.size)
        assertTrue(items.items.any { it.nodeId == "n0" }, "the first must still be open")
        assertEquals(items.items.last().key, items.activeKey, "the newest stays active")
        assertEquals(30, items.activeOrdinal, "the stepper counts the whole list")
    }

    @Test
    fun closing_the_active_item_moves_to_a_neighbour() {
        val items = OpenItems()
            .register(item("a")).register(item("b")).register(item("c"))
            .activate("folders:b")
            .close("folders:b")

        assertEquals(listOf("a", "c"), items.items.map { it.nodeId })
        assertEquals("folders:c", items.activeKey, "activity moves on, never to nothing")
    }

    @Test
    fun closing_the_last_item_leaves_nothing_active() {
        val items = OpenItems().register(item("a")).close("folders:a")

        assertEquals(0, items.size)
        assertNull(items.activeKey)
    }

    @Test
    fun stepping_wraps_in_both_directions() {
        val items = OpenItems().register(item("a")).register(item("b")).register(item("c"))

        assertEquals("folders:a", items.step(1).activeKey, "forward from the last wraps")
        assertEquals("folders:b", items.step(-1).activeKey)
    }

    @Test
    fun stepping_an_empty_list_is_a_no_op() {
        assertEquals(OpenItems(), OpenItems().step(1))
    }

    /**
     * Grouping moved out of here on purpose. An owner is a property of the *node*, so it
     * has to be read from the tree when the switcher renders — snapshotting it into the
     * open list froze it, and renaming a folder left its tabs filed under the old name.
     */
}
