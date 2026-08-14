package dawn.system.anchor.domain

/**
 * The workspace destinations, mirroring the desktop's `resolveWorkspaceDestination()`
 * (`aladin_react/src/services/workspace/workspace-helpers.ts`) exactly.
 *
 * The set is deliberately closed and deliberately identical to desktop: the iPad is a
 * companion to the same workspace, not a different product. Note what is absent —
 * there is no Tutor and no Learning destination. Tutor is what a *learning* folder opens
 * as ([[FolderPurpose]]), which is the handoff's load-bearing decision.
 */
enum class Destination(val id: String, val title: String) {
    Home("home", "Home"),
    Markets("markets", "Markets"),
    Folders("folders", "Folders"),
    Sources("sources", "Sources"),
    Insights("insights", "Insights"),
    Entities("entities", "Entities"),
    Graph("graph", "Graph"),
    ;

    companion object {
        val default = Folders

        fun fromId(id: String): Destination? = entries.firstOrNull { it.id == id }
    }
}
