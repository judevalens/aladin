package dawn.system.anchor.services.data

import io.ktor.client.HttpClient
import io.ktor.client.call.body
import io.ktor.client.request.put
import io.ktor.client.request.setBody
import io.ktor.http.ContentType
import io.ktor.http.contentType
import io.ktor.http.isSuccess
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put

/**
 * Reports the current page, [WorkspaceWriter]-style: PUT to Go (last-write-wins on the
 * server — no baseRevision, because "the page I most recently looked at" IS the right
 * merge), then apply the committed row through the store's seq guard so the echo frame
 * that follows is a no-op rather than a second write.
 */
interface ReadingPositionWriter {
    /** Reports [page]; a failure just means this report is lost (the next one heals). */
    suspend fun report(artifactId: String, page: Int)
}

internal class KtorReadingPositionWriter(
    private val client: HttpClient,
    private val store: ReadingPositionStore,
) : ReadingPositionWriter {

    override suspend fun report(artifactId: String, page: Int) {
        val response = client.put("/api/reading-positions/$artifactId") {
            contentType(ContentType.Application.Json)
            setBody(buildJsonObject { put("page", page) })
        }
        if (!response.status.isSuccess()) {
            error("the server refused the position report (${response.status.value})")
        }
        val committed: CommittedReadingPosition = response.body()
        store.applyAll(
            listOf(
                ReadingPositionChange(
                    id = committed.artifactId,
                    seq = committed.seq,
                    op = "upsert",
                    data = buildJsonObject {
                        put("artifactId", committed.artifactId)
                        put("page", committed.page)
                        put("updatedAt", committed.updatedAt)
                    },
                ),
            ),
        )
    }
}

/** The committed row a report returns (mirrors Go `service.ReadingPosition`). */
@Serializable
private data class CommittedReadingPosition(
    val artifactId: String,
    val page: Int,
    val seq: Long,
    val updatedAt: Long,
)
