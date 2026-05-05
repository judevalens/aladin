package com.jvp.aladin_compose.repo

import com.jvp.aladin_compose.api.ApiClient
import com.jvp.aladin_compose.api.FolderCreateRequest
import com.jvp.aladin_compose.api.FolderUpdateRequest
import com.jvp.aladin_compose.api.UserArtifact
import com.jvp.aladin_compose.api.UserArtifactCreateRequest
import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.model.ArtifactKind
import com.jvp.aladin_compose.model.ArtifactPreview
import com.jvp.aladin_compose.model.BreadcrumbItem
import com.jvp.aladin_compose.model.BrowserNodeKind
import com.jvp.aladin_compose.model.BrowserTreeNode
import com.jvp.aladin_compose.model.FolderNode
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.contentOrNull

interface FolderRepository {
    suspend fun browserTree(): List<BrowserTreeNode>

    suspend fun folders(parentId: String?): List<FolderNode>

    suspend fun folder(id: String): FolderNode

    suspend fun folderBreadcrumbs(folderId: String?): List<BreadcrumbItem>

    suspend fun artifacts(folderId: String?): List<Artifact>

    suspend fun createFolder(parentId: String?, title: String): FolderNode

    suspend fun createArtifact(
        folderId: String?,
        title: String,
        kind: ArtifactKind = ArtifactKind.Note,
    ): Artifact

    suspend fun renameFolder(folderId: String, title: String): FolderNode
}

class ApiFolderRepository : FolderRepository {
    override suspend fun browserTree(): List<BrowserTreeNode> {
        return ApiClient.getBrowserTree().map(::toBrowserTreeNode)
    }

    override suspend fun folders(parentId: String?): List<FolderNode> {
        return ApiClient.getFolders(parentId).map { FolderNode(it.id, it.parentId, it.title) }
    }

    override suspend fun folder(id: String): FolderNode {
        val folder = ApiClient.getFolder(id)
        return FolderNode(folder.id, folder.parentId, folder.title)
    }

    override suspend fun folderBreadcrumbs(folderId: String?): List<BreadcrumbItem> {
        if (folderId == null) return listOf(BreadcrumbItem(id = null, label = "Folders"))
        return ApiClient.getFolderBreadcrumbs(folderId).map { BreadcrumbItem(it.id, it.label) }
    }

    override suspend fun artifacts(folderId: String?): List<Artifact> {
        return ApiClient.getUserArtifacts(folderId).map(::toArtifact)
    }

    override suspend fun createFolder(parentId: String?, title: String): FolderNode {
        val created =
            ApiClient.createFolder(FolderCreateRequest(title = title, parentId = parentId))
        return FolderNode(created.id, created.parentId, created.title)
    }

    override suspend fun createArtifact(
        folderId: String?,
        title: String,
        kind: ArtifactKind,
    ): Artifact {
        val type =
            when (kind) {
                ArtifactKind.Note -> "page"
                ArtifactKind.Link -> "link"
                ArtifactKind.Voice -> "voice"
                ArtifactKind.File -> "file"
            }
        val created =
            ApiClient.createUserArtifact(
                UserArtifactCreateRequest(
                    type = type,
                    folderId = folderId,
                    title = title,
                    content = if (kind == ArtifactKind.Note) "New artifact" else "",
                    sourceUrl = if (kind == ArtifactKind.Link) "https://example.com" else null,
                )
            )
        return toArtifact(created)
    }

    override suspend fun renameFolder(folderId: String, title: String): FolderNode {
        val updated = ApiClient.updateFolder(folderId, FolderUpdateRequest(title = title))
        return FolderNode(updated.id, updated.parentId, updated.title)
    }

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
                if (kind == ArtifactKind.Voice || kind == ArtifactKind.File)
                    ApiClient.userArtifactResourceUrl(record.id)
                else null,
        )
    }

    private fun toBrowserTreeNode(
        record: com.jvp.aladin_compose.api.BrowserTreeRecord
    ): BrowserTreeNode {
        val kind =
            when (record.kind.lowercase()) {
                "folder" -> BrowserNodeKind.Folder
                else -> BrowserNodeKind.Artifact
            }
        val artifactPreview =
            if (kind == BrowserNodeKind.Artifact && record.artifactId != null) {
                ArtifactPreview(
                    id = record.artifactId,
                    title = record.title,
                    kind = artifactKind(record.artifactType),
                    updatedLabel = record.updatedAt,
                )
            } else {
                null
            }
        return BrowserTreeNode(
            id = record.id,
            parentId = record.parentId,
            kind = kind,
            title = record.title,
            artifactId = record.artifactId,
            artifactPreview = artifactPreview,
            children = record.children.map(::toBrowserTreeNode),
        )
    }

    private fun artifactKind(type: String?): ArtifactKind {
        return when (type?.lowercase()) {
            "page" -> ArtifactKind.Note
            "link" -> ArtifactKind.Link
            "voice" -> ArtifactKind.Voice
            "file" -> ArtifactKind.File
            else -> ArtifactKind.Note
        }
    }

    private fun metadataString(metadata: Map<String, JsonElement>, key: String): String? {
        val value = metadata[key] ?: return null
        return (value as? JsonPrimitive)?.contentOrNull
    }
}
