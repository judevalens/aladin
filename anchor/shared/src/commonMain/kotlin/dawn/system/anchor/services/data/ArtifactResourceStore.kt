package dawn.system.anchor.services.data

import dawn.system.anchor.services.platform.documentsDirPath
import dawn.system.anchor.services.platform.fileExists
import dawn.system.anchor.services.platform.writeBytes
import io.ktor.client.HttpClient
import io.ktor.client.request.get
import io.ktor.client.statement.bodyAsBytes
import io.ktor.http.contentType
import io.ktor.http.isSuccess

/**
 * A file artifact's bytes, on disk.
 *
 * Bodies are deliberately outside the sync spine — the spine carries the workspace *index*,
 * and a 40 MB PDF has no business in a frame. So this is a plain authenticated fetch, with
 * the file kept on disk afterwards: PDFKit needs a real path, and re-downloading a paper
 * every time you tap it would make the reader useless on a train.
 */
interface ArtifactResourceStore {
    /**
     * Fetches the artifact's resource, or returns the cached copy. Throws on transport or
     * auth failure — the caller decides how to say so.
     */
    suspend fun resource(artifactId: String): CachedResource

    /**
     * The cached copy, or null if it has to be fetched. **Synchronous on purpose.**
     *
     * A cache hit is a `stat`, but reaching it through [resource] costs a frame, because a
     * suspend call cannot complete before the first composition. That frame is a full-pane
     * "Fetching…" notice over a file that is already on disk — the flash you see every time
     * a reader is composed. A surface that can answer "already here" synchronously renders
     * the document on its first frame instead.
     */
    fun cached(artifactId: String): CachedResource?
}

/**
 * [path] is an absolute on-device path. [contentType] is what the server said it is, which
 * is more trustworthy than a filename: an upload keeps whatever extension it arrived with.
 */
data class CachedResource(val path: String, val contentType: String?) {
    /** PDFKit is the only renderer the companion has today. */
    val isPdf: Boolean
        get() = contentType?.startsWith("application/pdf") == true ||
            path.endsWith(".pdf", ignoreCase = true)

    /**
     * Whether an Apple platform can decode this at all.
     *
     * Not academic: the desktop capture records `audio/mp4` in Safari/WKWebView but falls
     * back to **WebM/Opus** on Chromium, and no Apple platform ships a WebM decoder. Such a
     * note can only ever be silent here, so the surface says that rather than showing a
     * transport that does nothing.
     */
    val isPlayableAudio: Boolean
        get() = contentType?.substringBefore(';')?.trim()?.lowercase() !in UNPLAYABLE

    private companion object {
        val UNPLAYABLE = setOf("audio/webm", "video/webm", "audio/ogg", "audio/opus")
    }
}

internal class KtorArtifactResourceStore(private val client: HttpClient) : ArtifactResourceStore {

    override suspend fun resource(artifactId: String): CachedResource {
        // A cache hit has to answer the content type too, so the extension carries it.
        cached(artifactId)?.let { return it }

        val response = client.get("/api/artifacts/$artifactId/resource")
        // Without this the error body — a JSON message, or a login page — is written to
        // disk as if it were the recording, and the failure surfaces much later as a
        // player that silently refuses to start.
        if (!response.status.isSuccess()) {
            error("the server answered ${response.status.value} for this artifact's file")
        }

        val contentType = response.contentType()?.toString()
        val bytes = response.bodyAsBytes()
        if (bytes.isEmpty()) error("the server returned an empty file")

        val path = pathFor(artifactId, contentType)
        writeBytes(path, bytes)
        return CachedResource(path, contentType)
    }

    /** The extension the file was saved under is what remembers its type. */
    override fun cached(artifactId: String): CachedResource? = CACHED_TYPES
        .firstNotNullOfOrNull { (extension, contentType) ->
            "${cacheDir()}/$artifactId$extension"
                .takeIf { fileExists(it) }
                ?.let { CachedResource(it, contentType) }
        }

    /**
     * The extension is not cosmetic. AVFoundation infers a file's container from it, so an
     * extensionless m4a simply refuses to play — which is exactly how a voice note fails
     * silently. PDFKit is more forgiving, but the same rule keeps both honest.
     */
    private fun pathFor(artifactId: String, contentType: String?): String =
        "${cacheDir()}/$artifactId${extensionFor(contentType)}"

    private fun cacheDir(): String = "${documentsDirPath()}/artifacts"

    private companion object {
        /** Extension → the content type it stands for, for answering a cache hit. */
        val CACHED_TYPES = listOf(
            ".pdf" to "application/pdf",
            ".m4a" to "audio/mp4",
            ".mp3" to "audio/mpeg",
            ".wav" to "audio/wav",
            ".aac" to "audio/aac",
            ".webm" to "video/webm",
            ".ogg" to "audio/ogg",
        )
    }
}

/** Maps a content type to the extension the platform decoders expect. */
internal fun extensionFor(contentType: String?): String {
    val type = contentType?.substringBefore(';')?.trim()?.lowercase() ?: return ""
    return when {
        type == "application/pdf" -> ".pdf"
        // The voice capture records m4a, which arrives as audio/mp4 (audio-only) — the
        // exact content type WebKit and AVFoundation want.
        type == "audio/mp4" || type == "audio/m4a" || type == "audio/x-m4a" -> ".m4a"
        type == "audio/aac" -> ".aac"
        type == "audio/mpeg" || type == "audio/mp3" -> ".mp3"
        type == "audio/wav" || type == "audio/x-wav" -> ".wav"
        // Chromium's MediaRecorder labels an audio-only WebM `video/webm`. Nothing on iOS
        // can play it, but it still gets its real extension — a file named for what it is
        // is what lets the surface explain the refusal.
        type == "audio/webm" || type == "video/webm" -> ".webm"
        type == "audio/ogg" || type == "audio/opus" -> ".ogg"
        type == "image/png" -> ".png"
        type == "image/jpeg" -> ".jpg"
        else -> ""
    }
}
