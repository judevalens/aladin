package com.jvp.aladin_compose.service

import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.model.ArtifactKind
import com.jvp.aladin_compose.model.BreadcrumbItem
import com.jvp.aladin_compose.model.FolderNode
import com.jvp.aladin_compose.model.FolderWorkspace
import com.jvp.aladin_compose.repo.FolderRepository

class FolderService(
    private val repository: FolderRepository,
) {
    suspend fun children(parentId: String?): List<FolderNode> = repository.children(parentId)

    suspend fun breadcrumbs(parentId: String?): List<BreadcrumbItem> = repository.breadcrumbs(parentId)

    suspend fun workspace(folderId: String): FolderWorkspace? = repository.workspace(folderId)

    suspend fun createFolder(parentId: String?): FolderNode {
        val siblingCount = repository.children(parentId).size + 1
        return repository.createFolder(parentId, "New Folder $siblingCount")
    }

    suspend fun createArtifact(folderId: String): Artifact {
        val workspace = repository.workspace(folderId)
        val nextIndex = (workspace?.artifacts?.size ?: 0) + 1
        return repository.createArtifact(folderId, "New Artifact $nextIndex", ArtifactKind.Note)
    }
}
