package com.jvp.aladin_compose.repo

import com.jvp.aladin_compose.api.ApiClient
import com.jvp.aladin_compose.api.FileUploadRequest
import com.jvp.aladin_compose.api.PageDocumentRecord
import com.jvp.aladin_compose.api.UploadedFileRecord
import com.jvp.aladin_compose.api.UserArtifact
import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.model.ArtifactKind
import com.jvp.aladin_compose.model.BreadcrumbItem
import com.jvp.aladin_compose.repo.doa.ArtifactDoa
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.emitAll
import kotlinx.coroutines.flow.filter
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.runningFold
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.contentOrNull

interface ArtifactRepository {
    fun artifact(id: String): Flow<Artifact>

    fun artifacts(ids: List<String>): Flow<List<Artifact>>

    fun observeArtifactBreadcrumbs(id: String?): Flow<List<BreadcrumbItem>>

    suspend fun page(id: String): Flow<PageDocument>

    suspend fun savePage(id: String, markdown: String, revision: Long): PageDocument

    suspend fun renameArtifact(id: String, title: String): Artifact

    suspend fun uploadFile(filename: String, bytes: ByteArray, contentType: String?): UploadedFile

    fun artifactResourceUrl(id: String): String
}

data class PageDocument(
    val id: String,
    val title: String,
    val content: String,
    val revision: Long,
    val updatedAt: String,
)

data class UploadedFile(val id: String, val url: String, val uploadedAt: String)

class ArtifactRepositoryImpl(val inMemoryArtifactDoa: ArtifactDoa) : ArtifactRepository {

    val artifactFlow: MutableSharedFlow<Artifact> = MutableSharedFlow()
    val pageFLow: MutableSharedFlow<PageDocument> = MutableSharedFlow()

    override fun artifact(id: String): Flow<Artifact> = flow {
        val artifact = inMemoryArtifactDoa.getArtifact(id)?.let(::userArtifactToArtifact)
        if (artifact != null) {
            emit(artifact)
            emitAll(artifactFlow.filter { it.id == id })
        } else {
            throw IllegalStateException("Artifact not found")
        }
    }

    override fun artifacts(ids: List<String>): Flow<List<Artifact>> {
        if (ids.isEmpty()) return flowOf(emptyList())
        val idSet = ids.toSet()

        return flow {
            val initialArtifacts =
                ids.mapNotNull {
                        inMemoryArtifactDoa.getArtifact(it)?.let(::userArtifactToArtifact)
                    }
                    .associateBy { it.id }

            val updateFlow =
                artifactFlow
                    .filter { it.id in idSet }
                    .runningFold(initialArtifacts) { oldArtifacts, updatedArtifact ->
                        oldArtifacts + (updatedArtifact.id to updatedArtifact)
                    }
                    .map { artifacts -> ids.mapNotNull { artifacts[it] } }
            emitAll(updateFlow)
        }
    }

    override fun observeArtifactBreadcrumbs(id: String?): Flow<List<BreadcrumbItem>> {
        if (id == null) return flowOf(listOf(BreadcrumbItem(id = null, label = "Folders")))

        return flow {
            val initialArtifact =
                inMemoryArtifactDoa.getArtifact(id)?.let(::userArtifactToArtifact)
                    ?: throw IllegalStateException("Artifact not found")
            emit(artifactBreadcrumbs(initialArtifact))
            val updateFlow =
                artifactFlow
                    .filter { it.id == id }
                    .map { artifact -> artifactBreadcrumbs(artifact) }
            emitAll(updateFlow)
        }
    }

    override suspend fun page(id: String): Flow<PageDocument> = flow {
        val pageDocument = inMemoryArtifactDoa.getPage(id)?.let(::toPageDocument)
        if (pageDocument != null) {
            emit(pageDocument)
            emitAll(pageFLow.filter { it.id == id })
        } else {
            throw IllegalStateException("Page not found")
        }
    }

    override suspend fun savePage(id: String, markdown: String, revision: Long): PageDocument =
        toPageDocument(inMemoryArtifactDoa.savePage(id, markdown, revision))

    override suspend fun renameArtifact(id: String, title: String): Artifact {
        val artifact = userArtifactToArtifact(inMemoryArtifactDoa.renameArtifact(id, title))
        artifactFlow.emit(artifact)
        return artifact
    }

    override suspend fun uploadFile(
        filename: String,
        bytes: ByteArray,
        contentType: String?,
    ): UploadedFile =
        toUploadedFile(
            ApiClient.uploadFile(
                FileUploadRequest(filename = filename, bytes = bytes, contentType = contentType)
            )
        )

    override fun artifactResourceUrl(id: String): String = ApiClient.userArtifactResourceUrl(id)

    private fun toPageDocument(record: PageDocumentRecord): PageDocument =
        PageDocument(
            id = record.id,
            title = record.title,
            content = record.content,
            revision = record.revision,
            updatedAt = record.updatedAt,
        )

    private fun toUploadedFile(record: UploadedFileRecord): UploadedFile =
        UploadedFile(
            id = record.id,
            url = ApiClient.backendUrl(record.url),
            uploadedAt = record.uploadedAt,
        )

    private fun artifactBreadcrumbs(artifact: Artifact): List<BreadcrumbItem> =
        listOf(
            BreadcrumbItem(id = null, label = "Folders"),
            BreadcrumbItem(id = artifact.id, label = artifact.title),
        )
}

fun userArtifactToArtifact(record: UserArtifact): Artifact {
    val kind =
        when (record.type.lowercase()) {
            "page" -> ArtifactKind.Note
            "note" -> ArtifactKind.Note
            "link" -> ArtifactKind.Link
            "voice" -> ArtifactKind.Voice
            "file" -> ArtifactKind.File
            else -> ArtifactKind.Note
        }
    val title =
        record.title.ifBlank {
            metadataString(record.metadata, "originalFilename")
                ?: metadataString(record.metadata, "storageKey")
                ?: "Untitled artifact"
        }
    return Artifact(
        id = record.id,
        folderId = record.folderId,
        title = title,
        content = record.content,
        summary = record.summary,
        kind = kind,
        updatedLabel = record.updatedAt,
        sourceUrl = record.sourceUrl,
        resourceUrl =
            if (kind == ArtifactKind.Voice || kind == ArtifactKind.File)
                ApiClient.userArtifactResourceUrl(record.id)
            else null,
    )
}

private fun metadataString(metadata: Map<String, JsonElement>, key: String): String? {
    val value = metadata[key] ?: return null
    return (value as? JsonPrimitive)?.contentOrNull
}
