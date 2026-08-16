package dawn.system.anchor.domain

/**
 * A per-item condition the workspace can report.
 *
 * **Nothing populates this yet, and that is deliberate.** The design draws a state dot on
 * every row, but the server has no field for any of the three: there is no read tracking of
 * any kind (`unread`), link artifacts are written once and never revisited (`stale`), and
 * `recovered` is a *derived* condition — a document whose outline came from layout analysis
 * rather than its own bookmarks — with no column and no place on the sync frame.
 *
 * Modelled rather than faked, on the same principle as [FolderPurpose]: the vocabulary is the
 * design's spine, and a row renders no dot until the tree can say otherwise. A guessed state
 * is worse than none.
 */
enum class ItemState {
    Unread,
    Recovered,
    Stale,
}

/** How a filtered list is ordered. Not a filter — see [BrowserFilter.isNarrowing]. */
enum class ItemSort {
    /** Most recently touched first. */
    Recent,

    /** By title, A–Z. */
    Name,
}

/**
 * What the Browser and the Open list are both narrowed by.
 *
 * One filter, two surfaces — the design's own words: *"applies to Browser and Open"*. Client
 * state only; no repo, service or backend change.
 *
 * **An empty group means no constraint**, so the facets are OR *within* a group and AND
 * *across* groups: picking two kinds widens, picking a kind and a state narrows.
 */
data class BrowserFilter(
    val kinds: Set<ArtifactKind> = emptySet(),
    val purposes: Set<FolderPurpose> = emptySet(),
    val states: Set<ItemState> = emptySet(),
    val sort: ItemSort = ItemSort.Recent,
) {

    /**
     * Whether anything is actually being filtered out.
     *
     * [sort] is deliberately excluded. Choosing A–Z reorders, it does not narrow — so it must
     * not light the filter badge or produce a pill, or a merely-reordered list would read as
     * a filtered one.
     */
    val isNarrowing: Boolean
        get() = kinds.isNotEmpty() || purposes.isNotEmpty() || states.isNotEmpty()

    /** How many facets are active — the badge's number, and the pill count. */
    val activeCount: Int get() = kinds.size + purposes.size + states.size

    /**
     * Whether one item survives.
     *
     * Note the state clause: an item with **no** state fails a state facet outright rather
     * than passing it. Filtering for "unread" asks for the unread ones, not for everything
     * that is not marked otherwise.
     */
    fun matches(
        kind: ArtifactKind?,
        purpose: FolderPurpose?,
        state: ItemState?,
    ): Boolean =
        (kinds.isEmpty() || kind in kinds) &&
            (purposes.isEmpty() || purpose in purposes) &&
            (states.isEmpty() || (state != null && state in states))

    /** Clears the facets. [sort] survives, because it is a preference rather than a filter. */
    fun cleared(): BrowserFilter = copy(kinds = emptySet(), purposes = emptySet(), states = emptySet())

    fun toggle(kind: ArtifactKind): BrowserFilter = copy(kinds = kinds.flip(kind))
    fun toggle(purpose: FolderPurpose): BrowserFilter = copy(purposes = purposes.flip(purpose))
    fun toggle(state: ItemState): BrowserFilter = copy(states = states.flip(state))
    fun sortedBy(order: ItemSort): BrowserFilter = copy(sort = order)
}

private fun <T> Set<T>.flip(value: T): Set<T> = if (contains(value)) this - value else this + value
