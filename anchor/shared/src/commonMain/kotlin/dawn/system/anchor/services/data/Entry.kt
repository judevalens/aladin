package dawn.system.anchor.services.data

/**
 * A journal entry — **metadata only**. The actual content (BlockNote blocks) is a
 * collaborative Yjs document served by Hocuspocus, NOT stored on this row.
 *
 * As in Aladin, the entry [id] is also the collaborative document's address (a 1:1
 * shared id): `artifacts.id == page_ydoc.page_id ==` the Hocuspocus room name. So there
 * is no separate foreign-key column — [id] (via [collabDocName]) is the link. Metadata
 * rides the sync/repository path; the Yjs content syncs over Hocuspocus separately.
 */
data class Entry(
    override val id: String,
    val title: String?,
    val createdAt: Long,
    val updatedAt: Long,
) : Identifiable {
    /**
     * The Hocuspocus/Yjs document name (room) for this entry's content — identical to
     * [id]. The client's offline persistence key is `"page-$id"`.
     */
    val collabDocName: String get() = id
}
