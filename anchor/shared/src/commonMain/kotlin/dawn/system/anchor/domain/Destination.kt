package dawn.system.anchor.domain

/**
 * The three places the sidebar can send you.
 *
 * **Three, because the browser is not one of them.** The previous pass made it a fourth, and
 * testing killed that: picking a resource is something you do constantly, mid-task, and a
 * destination charges a full context switch for it — leave the document, go to Browser, pick,
 * come back. Destinations are places you *go*; the browser is a tool you *summon*, so it lives
 * in chrome (a dropdown over your content, or a tab in Open) and is a [Surface] without being
 * a destination.
 *
 * Each surface still owns its own shape, which was the first pass's correct half: *"Markets is
 * a dashboard, Graph is a canvas, Insights is a feed — none of them are trees, but all of them
 * arrived inside a tree-shaped pane."*
 *
 * What is absent is as considered as what is here. Insights folds into [Home]. Sources and
 * Entities stay addressable — an entity or a source still opens — but neither earns a
 * permanent seat, and whether Entities eventually does is an open question for the owner
 * rather than something to decide in code.
 */
enum class Destination(val id: String, val title: String) {
    Home("home", "Home"),
    Markets("markets", "Markets"),
    Graph("graph", "Graph"),
    ;

    companion object {
        val default = Home

        fun fromId(id: String): Destination? = entries.firstOrNull { it.id == id }
    }
}
