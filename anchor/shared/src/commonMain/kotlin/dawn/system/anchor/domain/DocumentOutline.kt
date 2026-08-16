package dawn.system.anchor.domain

/**
 * One navigable row of a document's contents.
 *
 * [depth] indents it; [page] is where it starts. Both come from the ingestion pipeline —
 * either the file's own bookmarks or the outline segmentation recovered (INGESTION_PRD
 * §11), which for most research PDFs is the only one there is.
 */
data class OutlineEntry(val title: String, val page: Int, val depth: Int)

/** Where a document's outline came from. Authored beats recovered — and says so. */
enum class OutlineSource {
    /** The file carried its own bookmarks. Shown without comment: that is the normal case. */
    Authored,

    /**
     * The file carried none and segmentation inferred one. Flagged in the UI, because a
     * recovered heading can be wrong in a way an authored one cannot.
     */
    Recovered,

    /** No outline at all. The PDF still reads; it just has no contents rail. */
    None,
}

/**
 * A document as the reader needs it: what state its ingestion is in, how long it is, and
 * how to navigate it.
 */
data class IngestedDocument(
    val status: DocumentStatus,
    val pageCount: Int,
    val outline: List<OutlineEntry> = emptyList(),
    val outlineSource: OutlineSource = OutlineSource.None,
    val error: String? = null,
) {
    /**
     * The outline row for [page] — the **last entry starting at or before it**, which makes
     * the rail a position rather than a list of links. A page in the middle of §4.2 is in
     * §4.2 even though nothing starts there.
     */
    fun entryAt(page: Int): OutlineEntry? = outline.lastOrNull { it.page <= page }
}

/**
 * The ingestion statuses a reader must distinguish, mirroring desktop's `DocumentStatus`.
 *
 * The distinction that matters is "still working" vs "quietly gave up": a spinner shown
 * over a failed extraction is the worst of the four, because nothing will ever end it.
 */
enum class DocumentStatus(val wire: String) {
    Pending("pending"),
    Ingesting("ingesting"),
    Ready("ready"),
    Unsupported("unsupported"),
    Failed("failed"),
    ;

    /** Whether the page stage should render rather than a notice. */
    val isReadable: Boolean get() = this == Ready

    companion object {
        /**
         * Unknown statuses read as [Ready] on purpose. The PDF bytes are already on the
         * device by the time this is asked, so the reader's job is to show the file — an
         * unrecognised ingestion state is not a reason to withhold it.
         */
        fun fromWire(value: String?): DocumentStatus =
            entries.firstOrNull { it.wire == value } ?: Ready
    }
}
