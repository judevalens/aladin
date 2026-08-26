package dawn.system.anchor.features.shell.state

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import com.slack.circuit.runtime.CircuitUiState
import dawn.system.anchor.domain.ArtifactKind
import dawn.system.anchor.domain.Nav
import dawn.system.anchor.domain.OpenTab
import dawn.system.anchor.domain.TabKey
import dawn.system.anchor.domain.artifactKind
import dawn.system.anchor.domain.IngestedDocument
import dawn.system.anchor.services.data.ArtifactResourceStore
import dawn.system.anchor.services.data.DocumentStore
import dawn.system.anchor.services.data.NodeStore

/**
 * Where a file's bytes are.
 *
 * Three states, for the same reason the tree read has three: **a failure that looks like a
 * slow success is a surface that waits forever.** A nullable path cannot tell "still
 * downloading" from "the server said no", and the reader would sit on "Fetching…" either way.
 */
sealed interface Fetch {
    data object Pending : Fetch
    data class Ready(val path: String, val contentType: String? = null) : Fetch
    data class Failed(val reason: String) : Fetch
}

/** What one open document needs to render. */
sealed interface OpenDocument {
    val key: TabKey
    val title: String

    /**
     * A document that gets a surface to itself.
     *
     * The split is at the type level rather than checked at a call site, because the one
     * exception — the web-backed documents, which share a single view — is easy to route
     * wrong and impossible to notice: a note handed its own page renders a second editor.
     */
    sealed interface Standalone : OpenDocument

    /** A file for the reader: its bytes, its server-said type, and its ingested half. */
    data class Pdf(
        override val key: TabKey,
        override val title: String,
        val bytes: Fetch,
        /** The artifact's type string — picks the glyph on the unsupported-file card. */
        val artifactType: String? = null,
        /** Status/outline/page count from ingestion; null = not ingested (still readable). */
        val document: IngestedDocument? = null,
    ) : Standalone

    /**
     * A note or a shard — anything the embedded editor renders.
     *
     * Carries no content and no fetch, unlike [Pdf]: a note's text is not a file to
     * download but a Y.Doc joined over the collab socket, so the only thing needed to
     * render one is its id.
     */
    data class Web(
        override val key: TabKey,
        override val title: String,
        val kind: ArtifactKind,
        /** The node's sync version — the board pane refetches its snapshot when it moves. */
        val rev: Long = 0,
    ) : OpenDocument

    /** Anything with no surface yet — a voice memo, a link. */
    data class Unsupported(
        override val key: TabKey,
        override val title: String,
        val kind: ArtifactKind?,
    ) : Standalone
}

data class DocumentSlice(
    /**
     * **Every** open document, not just the visible one — each becomes a page and stays
     * composed, so none is ever rebuilt. Closing removes one; navigating away does not, which
     * is the whole point.
     */
    val documents: List<OpenDocument>,
    /** The one being shown, or null on a destination. */
    val active: OpenDocument?,
) : CircuitUiState

/**
 * Resolves open documents to something renderable.
 *
 * **Files are fetched once and cached on disk**, outside the sync spine — a PDF's bytes are
 * not a tree row. [ArtifactResourceStore.cached] answers synchronously so a document that has
 * been opened before paints on the first frame instead of flashing a spinner; only a genuine
 * first open waits.
 */
class DocumentStateProducer(
    private val nodes: NodeStore,
    private val resources: ArtifactResourceStore,
    private val documents: DocumentStore,
) {

    @Composable
    operator fun invoke(nav: Nav): DocumentSlice {
        // Fetches are keyed by artifact and outlive any one page, so switching away from a
        // download does not abandon it.
        val fetches = remember { mutableStateMapOf<String, Fetch>() }
        // The ingested half (outline, page count) per open file — resolved beside the
        // bytes, never blocking them: a PDF with no outline still reads.
        val ingested = remember { mutableStateMapOf<String, IngestedDocument?>() }

        // EVERY open document is resolved, not just the visible one, because every one of them
        // becomes a page and stays composed. A document you switch to must already have its
        // bytes, or "switching" is a download.
        //
        // The browser tab is filtered out rather than special-cased below: it is a place, not a
        // document, and asking the store about it would resolve an "Untitled" nothing.
        val documents = nav.open.filterIsInstance<OpenTab.Doc>().map { open ->
            val tab = open.key
            key(tab.asString()) {
                val node = nodes.nodeOf(tab.nodeId)
                val title = node?.title?.takeIf { it.isNotBlank() } ?: "Untitled"
                val kind = node?.artifactKind
                val id = node?.id

                if (kind == null || id == null) {
                    // Not yet read, or not an artifact at all. Deliberately the same branch:
                    // neither has a surface, and neither should start a fetch.
                    OpenDocument.Unsupported(tab, title, kind)
                } else when {
                    // The kind decides, not the file: a note and a shard are both rendered
                    // by the embedded editor, which is why they share one surface.
                    kind.isWebBacked -> OpenDocument.Web(tab, title, kind, rev = node.rev)

                    kind == ArtifactKind.File -> {
                        // Seeded from the cache, which answers synchronously, so anything
                        // fetched before paints on frame one. Only a genuine first open is
                        // ever Pending.
                        val seeded = fetches[id]
                            ?: resources.cached(id)?.let { Fetch.Ready(it.path, it.contentType) }
                            ?: Fetch.Pending

                        LaunchedEffect(id) {
                            if (fetches[id] !is Fetch.Ready && resources.cached(id) == null) {
                                fetches[id] = runCatching { resources.resource(id) }.fold(
                                    onSuccess = { Fetch.Ready(it.path, it.contentType) },
                                    // The server's own words where there are any. "Couldn't
                                    // open it" alone leaves nothing to act on, and silence
                                    // leaves a spinner that never resolves.
                                    onFailure = {
                                        Fetch.Failed(
                                            it.message?.takeIf(String::isNotBlank)
                                                ?: "couldn't reach the workspace",
                                        )
                                    },
                                )
                            }
                        }

                        LaunchedEffect(id) {
                            if (!ingested.containsKey(id)) {
                                ingested[id] = runCatching { documents.document(id) }.getOrNull()
                            }
                        }

                        OpenDocument.Pdf(
                            key = tab,
                            title = title,
                            bytes = seeded,
                            artifactType = node.artifactType,
                            document = ingested[id],
                        )
                    }

                    else -> OpenDocument.Unsupported(tab, title, kind)
                }
            }
        }

        val activeKey = nav.activeDoc
        return DocumentSlice(
            documents = documents,
            active = documents.firstOrNull { it.key == activeKey },
        )
    }
}
