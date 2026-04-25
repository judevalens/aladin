package com.jvp.aladin_compose.repo

import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.model.ArtifactKind
import com.jvp.aladin_compose.model.BreadcrumbItem
import com.jvp.aladin_compose.model.Item
import com.jvp.aladin_compose.model.ItemKind

interface ItemRepository {
    suspend fun items(): List<Item>
    suspend fun children(parentId: String?): List<Item>
    suspend fun item(itemId: String): Item?
    suspend fun breadcrumbs(itemId: String?): List<BreadcrumbItem>
    suspend fun artifact(artifactId: String): Artifact?
    suspend fun createFolder(parentId: String?, title: String): Item
    suspend fun createArtifactItem(parentId: String?, title: String, kind: ArtifactKind = ArtifactKind.Note): Pair<Item, Artifact>
}

class FakeItemRepository : ItemRepository {
    private val items = mutableListOf(
        Item("item_market", null, "Market Research", ItemKind.Folder, "001"),
        Item("item_watch", null, "Watchlist", ItemKind.Folder, "002"),
        Item("item_supply", "item_market", "AI Supply Chain", ItemKind.Folder, "001"),
        Item("item_gtm", "item_market", "GTM Research", ItemKind.Folder, "002"),
        Item("item_rivian", "item_watch", "Rivian", ItemKind.Folder, "001"),
        Item("item_supply_voice", "item_supply", "Voice memo on chip pressure", ItemKind.Artifact, "001", "artifact_supply_voice"),
        Item("item_rivian_note", "item_rivian", "Rivian thesis", ItemKind.Artifact, "001", "artifact_rivian_note"),
        Item("item_rivian_link", "item_rivian", "Rivian earnings clip", ItemKind.Artifact, "002", "artifact_rivian_link"),
    )

    private val artifacts = mutableListOf(
        Artifact(
            id = "artifact_rivian_note",
            folderId = "item_rivian",
            title = "Rivian thesis",
            summary = "Watch whether manufacturing pressure and adjacent AI capex narratives change sentiment over the next quarter.",
            kind = ArtifactKind.Note,
            updatedLabel = "Updated today",
        ),
        Artifact(
            id = "artifact_rivian_link",
            folderId = "item_rivian",
            title = "Rivian earnings clip",
            summary = "Saved link with a short note about margin sensitivity and delivery pacing.",
            kind = ArtifactKind.Link,
            updatedLabel = "Yesterday",
        ),
        Artifact(
            id = "artifact_supply_voice",
            folderId = "item_supply",
            title = "Voice memo on chip pressure",
            summary = "Quick recorded thought about second-order demand shifts from AI infra spending.",
            kind = ArtifactKind.Voice,
            updatedLabel = "2 days ago",
        ),
    )

    override suspend fun items(): List<Item> = items.sortedWith(compareBy<Item> { it.parentId ?: "" }.thenBy { it.position })

    override suspend fun children(parentId: String?): List<Item> {
        return items.filter { it.parentId == parentId }.sortedBy { it.position }
    }

    override suspend fun item(itemId: String): Item? = items.firstOrNull { it.id == itemId }

    override suspend fun breadcrumbs(itemId: String?): List<BreadcrumbItem> {
        val crumbs = mutableListOf(BreadcrumbItem(id = null, label = "Folders"))
        var current = items.firstOrNull { it.id == itemId }
        val stack = mutableListOf<Item>()
        while (current != null) {
            stack += current
            current = items.firstOrNull { it.id == current.parentId }
        }
        stack.asReversed().forEach { crumbs += BreadcrumbItem(it.id, it.title) }
        return crumbs
    }

    override suspend fun artifact(artifactId: String): Artifact? {
        return artifacts.firstOrNull { it.id == artifactId }
    }

    override suspend fun createFolder(parentId: String?, title: String): Item {
        val item = Item(
            id = "item_${title.lowercase().replace(" ", "_")}_${items.size}",
            parentId = parentId,
            title = title,
            kind = ItemKind.Folder,
            position = nextPosition(parentId),
        )
        items += item
        return item
    }

    override suspend fun createArtifactItem(parentId: String?, title: String, kind: ArtifactKind): Pair<Item, Artifact> {
        val artifact = Artifact(
            id = "artifact_${artifacts.size}",
            folderId = parentId.orEmpty(),
            title = title,
            summary = when (kind) {
                ArtifactKind.Note -> "New note artifact"
                ArtifactKind.Link -> "New saved link"
                ArtifactKind.Voice -> "New voice artifact"
            },
            kind = kind,
            updatedLabel = "Just now",
        )
        val item = Item(
            id = "item_artifact_${items.size}",
            parentId = parentId,
            title = title,
            kind = ItemKind.Artifact,
            position = nextPosition(parentId),
            targetId = artifact.id,
        )
        artifacts.add(0, artifact)
        items += item
        return item to artifact
    }

    private fun nextPosition(parentId: String?): String {
        val nextIndex = childrenForPosition(parentId).size + 1
        return nextIndex.toString().padStart(3, '0')
    }

    private fun childrenForPosition(parentId: String?): List<Item> {
        return items.filter { it.parentId == parentId }
    }
}
