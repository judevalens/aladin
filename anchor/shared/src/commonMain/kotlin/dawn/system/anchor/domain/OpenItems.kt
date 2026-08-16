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
/**
 * **Identity only.** A title, an owner and a kind are all properties of the *node*, and the
 * node lives in the store — so copying them in here would freeze them at the moment the item
 * was opened, and a rename would never reach the switcher. Everything displayable is
 * resolved from the tree at render time; this records what is open, nothing more.
 */
data class OpenItem(
    val key: String,
    val destination: Destination,
    val nodeId: String,
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
        // Unbounded on purpose. The handoff caps this at 8, dropping the oldest, but that
        // conflates two different lifetimes: **open** is the user's statement of intent, and
        // **resident** is whether a surface currently exists in memory. Only the second is a
        // memory question, and KeepAlive already answers it — a surface you scroll away from
        // loses its view and rebinds from its snapshot, the way a list recycler rebinds a
        // row. Capping the list here instead would silently close things you had opened, to
        // save memory that residency has already bounded.
        return copy(items = items + item, activeKey = item.key)
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


}
