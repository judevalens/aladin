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

/** What one open document needs to render. */
sealed interface OpenDocument {
    val key: TabKey
    val title: String

    /** A PDF, once its bytes are on disk. [filePath] is null while the download is in flight. */
    data class Pdf(
        override val key: TabKey,
        override val title: String,
        val filePath: String?,
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

        var path by remember(activeKey) {
            mutableStateOf(activeKey?.nodeId?.let(resources::cached)?.path)
        }
        LaunchedEffect(activeKey, node?.artifactKind) {
            if (path == null && node?.artifactKind == ArtifactKind.File) {
                path = runCatching { resources.resource(node.id) }.getOrNull()?.path
            }
        }

        return DocumentSlice(
            active = activeKey?.let { key ->
                val title = node?.title?.takeIf { it.isNotBlank() } ?: "Untitled"
                when (node?.artifactKind) {
                    ArtifactKind.File -> OpenDocument.Pdf(key, title, path)
                    else -> OpenDocument.Unsupported(key, title, node?.artifactKind)
                }
            },
            residentPdfs = residentPdfs,
        )
    }
}
