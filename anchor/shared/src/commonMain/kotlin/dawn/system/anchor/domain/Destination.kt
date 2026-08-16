package dawn.system.anchor.domain

/**
 * The four places the sidebar can send you.
 *
 * Deliberately **four**, and deliberately not the desktop's set. The first iPad pass rendered
 * every desktop destination through one tree-shaped pane, and the handoff names that as the
 * failure: *"Markets is a dashboard, Graph is a canvas, Insights is a feed — none of them are
 * trees, but all of them arrived inside a tree-shaped pane."*
 *
 * So each surface owns its own shape, and **tree-ness is quarantined in [Browser]** — which is
 * the desktop's Folders destination, renamed for what it actually is.
 *
 * What is absent is as considered as what is here. Insights folds into [Home]. Sources and
 * Entities stay addressable — an entity or a source still opens — but neither earns a
 * permanent seat, and whether Entities eventually does is an open question for the owner
 * rather than something to decide in code.
 */
enum class Destination(val id: String, val title: String) {
    Home("home", "Home"),
    Browser("browser", "Browser"),
    Markets("markets", "Markets"),
    Graph("graph", "Graph"),
    ;

    companion object {
        val default = Home

        fun fromId(id: String): Destination? = entries.firstOrNull { it.id == id }
    }
}
