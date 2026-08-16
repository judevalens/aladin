package dawn.system.anchor.services.data

import dawn.system.anchor.domain.DocumentStatus
import dawn.system.anchor.domain.IngestedDocument
import dawn.system.anchor.domain.OutlineEntry
import dawn.system.anchor.domain.OutlineSource
import io.ktor.client.HttpClient
import io.ktor.client.call.body
import io.ktor.client.request.get
import io.ktor.http.isSuccess
import kotlinx.serialization.Serializable

/**
 * A file artifact's *ingested* half — status, page count, outline.
 *
 * Deliberately outside the sync spine, like the bytes themselves: this is derived data
 * about one document you are looking at right now, not workspace index the client keeps a
 * replica of. Text is not requested (`?text=1`) — the companion renders the page, not the
 * extracted layer, so the words would be megabytes it never shows.
 */
interface DocumentStore {
    /**
     * The document for [artifactId], or null when nothing has been ingested for it — the
     * normal state for a file that is not a PDF, and not an error.
     */
    suspend fun document(artifactId: String): IngestedDocument?
}

internal class KtorDocumentStore(private val client: HttpClient) : DocumentStore {

    override suspend fun document(artifactId: String): IngestedDocument? {
        val response = client.get("/api/artifacts/$artifactId/document")
        // 404 is "not ingested", which the reader treats as "no outline", not as failure.
        if (!response.status.isSuccess()) return null
        val document: DocumentDto = response.body()

        val authored = document.sections
            .filter { it.title.isNotBlank() }
            .map { OutlineEntry(it.title, it.page, (it.level - 1).coerceAtLeast(0)) }

        // Authored structure beats inferred structure, so the recovered tree is only
        // fetched when the file carried nothing of its own.
        val (outline, source) = when {
            authored.isNotEmpty() -> authored to OutlineSource.Authored
            else -> recoveredOutline(artifactId).let { recovered ->
                recovered to if (recovered.isEmpty()) OutlineSource.None else OutlineSource.Recovered
            }
        }

        return IngestedDocument(
            status = DocumentStatus.fromWire(document.status),
            pageCount = document.pageCount,
            outline = outline,
            outlineSource = source,
            error = document.error?.takeIf { it.isNotBlank() },
        )
    }

    private suspend fun recoveredOutline(artifactId: String): List<OutlineEntry> {
        val response = client.get("/api/artifacts/$artifactId/outline")
        if (!response.status.isSuccess()) return emptyList()
        return runCatching { response.body<List<ChunkDto>>() }
            .getOrDefault(emptyList())
            .let(::flatten)
    }

    /**
     * Sections only. A `block` is a leaf of body text with no title — real in the chunk
     * tree, meaningless as a navigation row.
     */
    private fun flatten(chunks: List<ChunkDto>, into: MutableList<OutlineEntry> = mutableListOf()):
        List<OutlineEntry> {
        chunks.forEach { chunk ->
            if (chunk.kind == "section" && chunk.title.isNotBlank()) {
                into += OutlineEntry(chunk.title, chunk.pageFrom, chunk.depth)
            }
            flatten(chunk.children, into)
        }
        return into
    }
}

@Serializable
private data class DocumentDto(
    val status: String = "",
    val error: String? = null,
    val pageCount: Int = 0,
    val sections: List<SectionDto> = emptyList(),
)

@Serializable
private data class SectionDto(val title: String = "", val level: Int = 1, val page: Int = 1)

@Serializable
private data class ChunkDto(
    val depth: Int = 0,
    val kind: String = "",
    val title: String = "",
    val pageFrom: Int = 1,
    val children: List<ChunkDto> = emptyList(),
)
