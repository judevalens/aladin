package com.jvp.aladin_compose.repo

import com.jvp.aladin_compose.api.ApiClient
import com.jvp.aladin_compose.api.FileUploadRequest
import com.jvp.aladin_compose.api.PageDocumentRecord
import com.jvp.aladin_compose.api.PageSaveRequest
import com.jvp.aladin_compose.api.UploadedFileRecord
import com.jvp.aladin_compose.api.UserArtifactUpdateRequest
import com.jvp.aladin_compose.api.UserArtifact
import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.model.ArtifactKind
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.contentOrNull

interface ArtifactRepository {
    suspend fun artifact(id: String): Artifact
    suspend fun page(id: String): PageDocument
    suspend fun savePage(id: String, markdown: String): PageDocument
    suspend fun renameArtifact(id: String, title: String): Artifact
    suspend fun uploadFile(filename: String, bytes: ByteArray, contentType: String?): UploadedFile
    fun artifactResourceUrl(id: String): String
}

data class PageDocument(
    val id: String,
    val title: String,
    val content: String,
    val updatedAt: String,
)

data class UploadedFile(
    val id: String,
    val url: String,
    val uploadedAt: String,
)

class ApiArtifactRepository : ArtifactRepository {
    override suspend fun artifact(id: String): Artifact = toArtifact(ApiClient.getUserArtifact(id))

    override suspend fun page(id: String): PageDocument = toPageDocument(ApiClient.getPage(id))

    override suspend fun savePage(id: String, markdown: String): PageDocument =
        toPageDocument(ApiClient.savePage(id, PageSaveRequest(content = markdown)))

    override suspend fun renameArtifact(id: String, title: String): Artifact =
        toArtifact(ApiClient.updateUserArtifact(id, UserArtifactUpdateRequest(title = title)))

    override suspend fun uploadFile(filename: String, bytes: ByteArray, contentType: String?): UploadedFile =
        toUploadedFile(ApiClient.uploadFile(FileUploadRequest(filename = filename, bytes = bytes, contentType = contentType)))

    override fun artifactResourceUrl(id: String): String = ApiClient.userArtifactResourceUrl(id)

    private fun toPageDocument(record: PageDocumentRecord): PageDocument =
        PageDocument(
            id = record.id,
            title = record.title,
            content = record.content,
            updatedAt = record.updatedAt,
        )

    private fun toUploadedFile(record: UploadedFileRecord): UploadedFile =
        UploadedFile(
            id = record.id,
            url = ApiClient.backendUrl(record.url),
            uploadedAt = record.uploadedAt,
        )

    private fun toArtifact(record: UserArtifact): Artifact {
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
                if (kind == ArtifactKind.Voice || kind == ArtifactKind.File) artifactResourceUrl(record.id)
                else null,
        )
    }

    private fun metadataString(metadata: Map<String, JsonElement>, key: String): String? {
        val value = metadata[key] ?: return null
        return (value as? JsonPrimitive)?.contentOrNull
    }
}
