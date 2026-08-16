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
import dawn.system.anchor.domain.TabKey
import dawn.system.anchor.domain.artifactKind
import dawn.system.anchor.services.data.ArtifactResourceStore
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
    data class Ready(val path: String) : Fetch
    data class Failed(val reason: String) : Fetch
}

/** What one open document needs to render. */
sealed interface OpenDocument {
    val key: TabKey
    val title: String

    /** A PDF, and whether its bytes have landed. */
    data class Pdf(
        override val key: TabKey,
        override val title: String,
        val bytes: Fetch,
    ) : OpenDocument

    /** Anything with no surface yet — a note, a shard, a voice memo, a link. */
    data class Unsupported(
        override val key: TabKey,
        override val title: String,
        val kind: ArtifactKind?,
    ) : OpenDocument
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
) {

    @Composable
    operator fun invoke(nav: Nav): DocumentSlice {
        // Fetches are keyed by artifact and outlive any one page, so switching away from a
        // download does not abandon it.
        val fetches = remember { mutableStateMapOf<String, Fetch>() }

        // EVERY open document is resolved, not just the visible one, because every one of them
        // becomes a page and stays composed. A document you switch to must already have its
        // bytes, or "switching" is a download.
        val documents = nav.open.map { tab ->
            key(tab.asString()) {
                val node = nodes.nodeOf(tab.nodeId)
                val title = node?.title?.takeIf { it.isNotBlank() } ?: "Untitled"

                if (node?.artifactKind != ArtifactKind.File) {
                    OpenDocument.Unsupported(tab, title, node?.artifactKind)
                } else {
                    // Seeded from the cache, which answers synchronously, so anything fetched
                    // before paints on frame one. Only a genuine first open is ever Pending.
                    val seeded = fetches[node.id]
                        ?: resources.cached(node.id)?.let { Fetch.Ready(it.path) }
                        ?: Fetch.Pending

                    LaunchedEffect(node.id) {
                        if (fetches[node.id] !is Fetch.Ready && resources.cached(node.id) == null) {
                            fetches[node.id] = runCatching { resources.resource(node.id) }.fold(
                                onSuccess = { Fetch.Ready(it.path) },
                                // The server's own words where there are any. "Couldn't open
                                // it" alone leaves nothing to act on, and silence leaves a
                                // spinner that never resolves.
                                onFailure = {
                                    Fetch.Failed(
                                        it.message?.takeIf(String::isNotBlank)
                                            ?: "couldn't reach the workspace",
                                    )
                                },
                            )
                        }
                    }

                    OpenDocument.Pdf(tab, title, seeded)
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
