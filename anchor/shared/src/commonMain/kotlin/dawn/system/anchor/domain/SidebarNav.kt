package dawn.system.anchor.domain

/**
 * The path is **ids only**.
 *
 * It used to carry titles too, "so Back and the path popover can label themselves without a
 * store round-trip" — but there is no round-trip to avoid: R1 keeps a full local replica and
 * the presenter already collects it, so a title is a lookup in a list it is holding. What
 * the copy did buy was a stale label: rename a folder you are inside and Back kept calling
 * it by its old name until you navigated out.
 *
 * So the labels are resolved by the caller, from the store, every time they are drawn.
 */
typealias NodePath = List<String>

/** One rung of the path popover's "You are in" list. */
data class PathLevel(
    val title: String,
    /** `root` · `section` · the folder's purpose — or "· here" for the current rung. */
    val tag: String,
    val depth: Int,
    val isCurrent: Boolean,
)

/**
 * The no-rail shell's whole navigation model (PRD rev 2).
 *
 * One sidebar carries a stack: **root** (Aladin + the 7 sections) → **section** (its rows)
 * → **folder** (its contents) → **subfolder** (…). There is no breadcrumb; spatial
 * awareness comes from the slide animation, a plain Back chevron, and the path popover —
 * all three of which are views of this one value.
 *
 * Two distinct notions live here on purpose, and keeping them apart is the whole point:
 * - [selectedId] is **what the detail pane shows**. An artifact selects without descending,
 *   because an artifact *is* its view.
 * - [drill] is **where the sidebar is**. Only a container pushes onto it, which is what
 *   keeps Back honest and stops artifacts from creating levels to climb back out of.
 *
 * Conflating the two is what made opening a note inside a folder throw you back to the
 * folder's parent: selection cleared the descent, so the list you were reading from
 * vanished underneath the thing you had just opened.
 *
 * Pure: no Compose, no store. That is what makes "where does Back go from a subfolder"
 * a test rather than a thing you check by hand on a device.
 */
data class SidebarNav(
    /** Null at root — the sections menu. */
    val destination: Destination? = null,
    val selectedId: String? = null,
    val drill: NodePath = emptyList(),
) {
    val isRoot: Boolean get() = destination == null
    val isDrilled: Boolean get() = drill.isNotEmpty()

    /** Back exists everywhere but the root, and always pops exactly one rung. */
    val canBack: Boolean get() = !isRoot

    /**
     * The node the detail pane shows. The selection wins over the descent, because
     * selecting something inside a folder is exactly the case where the two differ.
     */
    val currentId: String? get() = selectedId ?: drill.lastOrNull()

    /** The folder whose contents the sidebar lists. Null anywhere above a drill. */
    val folderId: String? get() = drill.lastOrNull()

    /** Which level the sidebar list renders. */
    val level: Level
        get() = when {
            isRoot -> Level.Root
            isDrilled -> Level.Drill
            else -> Level.Section
        }

    enum class Level { Root, Section, Drill }

    fun openSection(destination: Destination): SidebarNav =
        SidebarNav(destination = destination)

    /**
     * Selecting a row. A container descends immediately — that is the rev-2 behaviour that
     * replaces "contents appear on the right and you cannot go anywhere". Anything else
     * only changes what is displayed, leaving the list you picked it from exactly where
     * it was.
     */
    fun select(node: WorkspaceNode): SidebarNav = when {
        !node.isContainer -> copy(selectedId = node.id)
        else -> drillInto(node.id)
    }

    /** A subfolder inside the current contents. Re-entering the same folder is a no-op. */
    fun drillInto(nodeId: String): SidebarNav = when {
        nodeId != drill.lastOrNull() -> copy(selectedId = nodeId, drill = drill + nodeId)
        // Already inside it — re-tapping just puts the folder itself back on display,
        // which is how you get from "reading a note in here" back to "reading the folder".
        nodeId != selectedId -> copy(selectedId = nodeId)
        else -> this
    }

    /** Pops one rung: subfolder → folder → section list → root. */
    fun back(): SidebarNav = when {
        isDrilled -> {
            val rest = drill.dropLast(1)
            // Landing back at section level, the folder you just left stays highlighted —
            // Back should return you to where you were, not to a blank slate.
            copy(drill = rest, selectedId = rest.lastOrNull() ?: drill.last())
        }
        !isRoot -> SidebarNav()
        else -> this
    }

    /** What Back returns to — the tooltip, and the only label the design gives Back. */
    fun backLabel(sectionTitle: String?, titleOf: (String) -> String): String? = when {
        drill.size >= 2 -> titleOf(drill[drill.size - 2])
        drill.size == 1 -> sectionTitle
        !isRoot -> ROOT_TITLE
        else -> null
    }

    /**
     * The path popover's rungs, root first. [sectionTitle] and [purposeOf] come from the
     * caller because the tree lives in the store, not here.
     */
    fun levels(
        sectionTitle: String?,
        titleOf: (String) -> String,
        purposeOf: (String) -> String = { "folder" },
    ): List<PathLevel> = buildList {
        add(PathLevel(ROOT_TITLE, "root", depth = 0, isCurrent = isRoot))
        if (!isRoot && sectionTitle != null) {
            add(PathLevel(sectionTitle, "section", depth = 1, isCurrent = !isDrilled))
        }
        drill.forEachIndexed { index, nodeId ->
            add(
                PathLevel(
                    title = titleOf(nodeId),
                    tag = purposeOf(nodeId),
                    depth = index + 2,
                    isCurrent = index == drill.lastIndex,
                ),
            )
        }
    }

    /** Jumping to a rung of [levels] — index 0 is root, 1 the section, 2+ the drill path. */
    fun jumpTo(levelIndex: Int): SidebarNav = when {
        levelIndex <= 0 -> SidebarNav()
        else -> {
            val rest = drill.take(levelIndex - 1)
            copy(drill = rest, selectedId = rest.lastOrNull() ?: drill.firstOrNull())
        }
    }

    /** Lateral move: swap the deepest rung for a sibling folder, staying at this depth. */
    fun switchTo(nodeId: String): SidebarNav = when {
        drill.isEmpty() -> copy(selectedId = nodeId)
        else -> copy(selectedId = nodeId, drill = drill.dropLast(1) + nodeId)
    }

    companion object {
        const val ROOT_TITLE = "Aladin"
    }
}
