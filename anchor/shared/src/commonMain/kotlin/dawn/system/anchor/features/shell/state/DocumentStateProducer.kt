package dawn.system.anchor.features.shell.state

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
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
    /** The document being shown, or null on a destination. */
    val active: OpenDocument?,
    /**
     * Every open PDF, so the pool knows what may stay resident. Closing one is what removes
     * it — navigating away is not, which is the whole point.
     */
    val residentPdfs: List<String>,
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
        // Every open PDF's id — what the pool is allowed to keep alive.
        val residentPdfs = nav.open.mapNotNull { key ->
            nodes.nodeOf(key.nodeId)?.takeIf { it.artifactKind == ArtifactKind.File }?.id
        }

        val activeKey = nav.activeDoc
        val node = nodes.nodeOf(activeKey?.nodeId)

        // Seeded from the cache, which answers synchronously — a document opened before paints
        // on frame one instead of flashing. Only a genuine first open is Pending.
        var bytes: Fetch by remember(activeKey) {
            mutableStateOf(
                activeKey?.nodeId?.let(resources::cached)
                    ?.let { Fetch.Ready(it.path) }
                    ?: Fetch.Pending,
            )
        }
        LaunchedEffect(activeKey, node?.artifactKind) {
            if (bytes is Fetch.Pending && node?.artifactKind == ArtifactKind.File) {
                bytes = runCatching { resources.resource(node.id) }.fold(
                    onSuccess = { Fetch.Ready(it.path) },
                    // The server's own words where there are any. "Couldn't open it" alone
                    // leaves nothing to act on, and silence leaves a spinner forever.
                    onFailure = { Fetch.Failed(it.message?.takeIf(String::isNotBlank) ?: "couldn't reach the workspace") },
                )
            }
        }

        return DocumentSlice(
            active = activeKey?.let { key ->
                val title = node?.title?.takeIf { it.isNotBlank() } ?: "Untitled"
                when (node?.artifactKind) {
                    ArtifactKind.File -> OpenDocument.Pdf(key, title, bytes)
                    else -> OpenDocument.Unsupported(key, title, node?.artifactKind)
                }
            },
            residentPdfs = residentPdfs,
        )
    }
}
