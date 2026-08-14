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

    private fun item(id: String, owner: String = "Folders") = OpenItem(
        key = "folders:$id",
        destination = Destination.Folders,
        nodeId = id,
        title = id,
        owner = owner,
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

    @Test
    fun cap_drops_the_oldest_and_never_the_newest() {
        var items = OpenItems()
        repeat(OpenItems.CAP + 3) { i -> items = items.register(item("n$i")) }

        assertEquals(OpenItems.CAP, items.size)
        assertEquals("n${OpenItems.CAP + 2}", items.items.last().nodeId)
        assertEquals(items.items.last().key, items.activeKey, "the newest stays active")
        assertTrue(items.items.none { it.nodeId == "n0" }, "the oldest was dropped")
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

    @Test
    fun grouping_preserves_owner_order() {
        val items = OpenItems()
            .register(item("a", owner = "Semis cycle"))
            .register(item("b", owner = "Option strategies"))
            .register(item("c", owner = "Semis cycle"))

        assertEquals(
            listOf("Semis cycle" to 2, "Option strategies" to 1),
            items.grouped().map { (owner, group) -> owner to group.size },
        )
    }
}
