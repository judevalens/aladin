package com.jvp.aladin_compose.service

import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.repo.ArtifactRepository
import com.jvp.aladin_compose.repo.PageDocument
import com.jvp.aladin_compose.repo.UploadedFile

class ArtifactService(
    private val repository: ArtifactRepository,
) {
    private val artifactById = mutableMapOf<String, Artifact>()
    private val pageById = mutableMapOf<String, PageDocument>()

    suspend fun artifact(artifactId: String?): Artifact? {
        val id = artifactId ?: return null
        artifactById[id]?.let { return it }
        return repository.artifact(id).also { artifactById[id] = it }
    }

    suspend fun page(pageId: String?): PageDocument? {
        val id = pageId ?: return null
        pageById[id]?.let { return it }
        return repository.page(id).also { document ->
            pageById[id] = document
            artifactById[id]?.let { cachedArtifact ->
                artifactById[id] =
                    cachedArtifact.copy(
                        title = document.title,
                        content = document.content,
                        updatedLabel = document.updatedAt,
                    )
            }
        }
    }

    suspend fun savePage(pageId: String, markdown: String): PageDocument {
        val saved = repository.savePage(pageId, markdown)
        pageById[pageId] = saved
        artifactById[pageId]?.let { cachedArtifact ->
            artifactById[pageId] =
                cachedArtifact.copy(
                    title = saved.title,
                    content = saved.content,
                    updatedLabel = saved.updatedAt,
                )
        }
        return saved
    }

    suspend fun renameArtifact(artifactId: String, title: String): Artifact {
        val renamed = repository.renameArtifact(artifactId, title)
        artifactById[artifactId] = renamed
        pageById[artifactId]?.let { cachedPage ->
            pageById[artifactId] =
                cachedPage.copy(
                    title = renamed.title,
                    updatedAt = renamed.updatedLabel,
                )
        }
        return renamed
    }

    suspend fun uploadFile(
        filename: String,
        bytes: ByteArray,
        contentType: String?,
    ): UploadedFile = repository.uploadFile(filename, bytes, contentType)

    fun rememberArtifact(artifact: Artifact) {
        artifactById[artifact.id] = artifact
    }

    fun clearArtifact(artifactId: String) {
        artifactById.remove(artifactId)
        pageById.remove(artifactId)
    }
}
