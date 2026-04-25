package com.jvp.aladin_compose.repo

import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.model.ArtifactKind
import com.jvp.aladin_compose.model.BreadcrumbItem
import com.jvp.aladin_compose.model.FolderNode
import com.jvp.aladin_compose.model.FolderWorkspace

interface FolderRepository {
    suspend fun children(parentId: String?): List<FolderNode>
    suspend fun breadcrumbs(parentId: String?): List<BreadcrumbItem>
    suspend fun workspace(folderId: String): FolderWorkspace?
    suspend fun createFolder(parentId: String?, title: String): FolderNode
    suspend fun createArtifact(folderId: String, title: String, kind: ArtifactKind = ArtifactKind.Note): Artifact
}

class FakeFolderRepository : FolderRepository {
    private val nodes = mutableListOf(
        FolderNode("folder_market", null, "Market Research"),
        FolderNode("folder_watch", null, "Watchlist"),
        FolderNode("folder_rivian", "folder_watch", "Rivian"),
        FolderNode("folder_supply", "folder_market", "AI Supply Chain"),
        FolderNode("folder_gtm", "folder_market", "GTM Research"),
    )

    private val artifacts = mutableListOf(
        Artifact(
            id = "artifact_rivian_note",
            folderId = "folder_rivian",
            title = "Rivian thesis",
            summary = "Watch whether manufacturing pressure and adjacent AI capex narratives change sentiment over the next quarter.",
            kind = ArtifactKind.Note,
            updatedLabel = "Updated today",
        ),
        Artifact(
            id = "artifact_rivian_link",
            folderId = "folder_rivian",
            title = "Rivian earnings clip",
            summary = "Saved link with a short note about margin sensitivity and delivery pacing.",
            kind = ArtifactKind.Link,
            updatedLabel = "Yesterday",
        ),
        Artifact(
            id = "artifact_supply_voice",
            folderId = "folder_supply",
            title = "Voice memo on chip pressure",
            summary = "Quick recorded thought about second-order demand shifts from AI infra spending.",
            kind = ArtifactKind.Voice,
            updatedLabel = "2 days ago",
        ),
    )

    override suspend fun children(parentId: String?): List<FolderNode> {
        return nodes.filter { it.parentId == parentId }.sortedBy { it.title }
    }

    override suspend fun breadcrumbs(parentId: String?): List<BreadcrumbItem> {
        val items = mutableListOf(BreadcrumbItem(id = null, label = "Folders"))
        var current = nodes.firstOrNull { it.id == parentId }
        val stack = mutableListOf<FolderNode>()
        while (current != null) {
            stack += current
            current = nodes.firstOrNull { it.id == current.parentId }
        }
        stack.asReversed().forEach { items += BreadcrumbItem(it.id, it.title) }
        return items
    }

    override suspend fun workspace(folderId: String): FolderWorkspace? {
        val folder = nodes.firstOrNull { it.id == folderId } ?: return null
        return FolderWorkspace(
            folder = folder,
            artifacts = artifacts.filter { it.folderId == folderId },
            signalCount = when (folderId) {
                "folder_rivian" -> 3
                "folder_supply" -> 5
                else -> 0
            },
        )
    }

    override suspend fun createFolder(parentId: String?, title: String): FolderNode {
        val folder = FolderNode(
            id = "folder_${title.lowercase().replace(" ", "_")}_${nodes.size}",
            parentId = parentId,
            title = title,
        )
        nodes += folder
        return folder
    }

    override suspend fun createArtifact(folderId: String, title: String, kind: ArtifactKind): Artifact {
        val artifact = Artifact(
            id = "artifact_${artifacts.size}",
            folderId = folderId,
            title = title,
            summary = when (kind) {
                ArtifactKind.Note -> "New note artifact"
                ArtifactKind.Link -> "New saved link"
                ArtifactKind.Voice -> "New voice artifact"
            },
            kind = kind,
            updatedLabel = "Just now",
        )
        artifacts.add(0, artifact)
        return artifact
    }
}
