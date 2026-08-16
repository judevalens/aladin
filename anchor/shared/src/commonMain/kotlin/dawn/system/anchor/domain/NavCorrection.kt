package dawn.system.anchor.domain

/**
 * What the tree knows about the two nodes the sidebar points at.
 *
 * Deliberately a *read* of each, not a nullable node: "not looked up yet" and "not there" are
 * different answers, and only the second is grounds for moving the user. Conflating them
 * reverted every drill the frame after it happened, twice.
 */
enum class Presence { Unknown, Gone, There }

/**
 * The sidebar's position, corrected for things that are no longer there.
 *
 * Three rules, and each exists because of a bug that shipped:
 *
 *  1. The folder you are standing in was deleted — climb out. One rung at a time, so a whole
 *     deleted branch unwinds correctly as each new parent is read.
 *  2. What the detail is showing was deleted — stop showing it.
 *  3. What the detail is showing is no longer *open* — stop showing it. Closing a tab does
 *     not delete its node, so nothing else would clear this and the header would keep naming
 *     something the user just closed.
 *
 * **[Presence.Unknown] never moves anything.** That is the whole point of splitting it out.
 */
fun SidebarNav.corrected(
    folder: Presence,
    displayed: Presence,
    isOpen: (String) -> Boolean,
): SidebarNav {
    if (folderId != null && folder == Presence.Gone) return back()

    val selected = selectedId ?: return this
    val drop = displayed == Presence.Gone || !isOpen(selected)
    return if (drop) copy(selectedId = null) else this
}
