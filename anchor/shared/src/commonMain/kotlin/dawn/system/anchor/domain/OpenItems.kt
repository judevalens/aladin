package dawn.system.anchor.domain

/**
 * What a desktop tab was: something you return to in one tap.
 *
 * The desktop strip encoded ownership as a 2px divider and required hover to close — both
 * unusable on a tablet. The handoff replaces it with an explicit list grouped by [owner],
 * which is how the desktop rule "tabs of one research folder must be contiguous" survives
 * as something you can actually see.
 *
 * Pure list algebra, no UI: the switcher popover and the prev/next stepper are two views
 * of this one value, which is why both stay consistent for free.
 */
data class OpenItem(
    val key: String,
    val destination: Destination,
    val nodeId: String,
    val title: String,
    /** The folder that owns it, or the destination's name — the grouping heading. */
    val owner: String,
    val meta: String? = null,
)

data class OpenItems(
    val items: List<OpenItem> = emptyList(),
    val activeKey: String? = null,
) {
    val size: Int get() = items.size
    val active: OpenItem? get() = items.firstOrNull { it.key == activeKey }

    /** Position of the active item, 1-based — the "2 of 4" stepper. Zero when empty. */
    val activeOrdinal: Int get() = items.indexOfFirst { it.key == activeKey } + 1

    /**
     * Registers an item and makes it active. Re-registering an existing key activates it
     * in place rather than duplicating or reordering, so returning to something does not
     * shuffle the list under the user.
     */
    fun register(item: OpenItem): OpenItems {
        val existing = items.firstOrNull { it.key == item.key }
        if (existing != null) {
            return copy(
                items = items.map { if (it.key == item.key) item else it },
                activeKey = item.key,
            )
        }
        val appended = items + item
        // Cap at CAP, dropping the oldest — but never the one just opened.
        val trimmed = if (appended.size <= CAP) appended else appended.drop(appended.size - CAP)
        return copy(items = trimmed, activeKey = item.key)
    }

    /** Closes an item. If it was active, activity moves to its neighbour, never to nothing. */
    fun close(key: String): OpenItems {
        val index = items.indexOfFirst { it.key == key }
        if (index < 0) return this
        val remaining = items.filterNot { it.key == key }
        val nextActive = when {
            activeKey != key -> activeKey
            remaining.isEmpty() -> null
            else -> remaining[index.coerceAtMost(remaining.lastIndex)].key
        }
        return copy(items = remaining, activeKey = nextActive)
    }

    fun activate(key: String): OpenItems =
        if (items.any { it.key == key }) copy(activeKey = key) else this

    /** Cycles by [delta], wrapping — the stepper, and the swipe gesture it stands in for. */
    fun step(delta: Int): OpenItems {
        if (items.isEmpty()) return this
        val current = items.indexOfFirst { it.key == activeKey }.coerceAtLeast(0)
        val next = ((current + delta) % items.size + items.size) % items.size
        return copy(activeKey = items[next].key)
    }

    /** Grouped for the switcher, preserving first-seen owner order. */
    fun grouped(): List<Pair<String, List<OpenItem>>> =
        items.groupBy { it.owner }.entries.map { it.key to it.value }

    companion object {
        const val CAP = 8
    }
}
