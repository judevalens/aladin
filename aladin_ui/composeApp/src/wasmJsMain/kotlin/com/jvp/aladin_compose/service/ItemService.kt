package com.jvp.aladin_compose.service

import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.model.BreadcrumbItem
import com.jvp.aladin_compose.model.Item
import com.jvp.aladin_compose.model.ItemKind
import com.jvp.aladin_compose.model.canHaveChildren
import com.jvp.aladin_compose.repo.ItemRepository

class ItemService(
    private val repository: ItemRepository,
) {
    suspend fun items(): List<Item> = repository.items()

    suspend fun breadcrumbs(itemId: String?): List<BreadcrumbItem> = repository.breadcrumbs(itemId)

    suspend fun item(itemId: String?): Item? = itemId?.let { repository.item(it) }

    suspend fun artifactFor(item: Item?): Artifact? {
        if (item?.kind != ItemKind.Artifact) return null
        return item.targetId?.let { repository.artifact(it) }
    }

    suspend fun ancestorIds(itemId: String?): Set<String> {
        val ancestors = mutableSetOf<String>()
        var current = item(itemId)
        while (current?.parentId != null) {
            val parent = repository.item(current.parentId)
            if (parent != null) ancestors += parent.id
            current = parent
        }
        return ancestors
    }

    suspend fun createFolder(parentId: String?): Item {
        val siblingCount = repository.children(parentId).size + 1
        return repository.createFolder(parentId, "New Folder $siblingCount")
    }

    suspend fun createArtifactItem(parentId: String?): Pair<Item, Artifact> {
        val siblingCount = repository.children(parentId).size + 1
        return repository.createArtifactItem(parentId, "New Artifact $siblingCount")
    }

    suspend fun nearestContainerId(itemId: String?): String? {
        val selected = item(itemId) ?: return null
        return if (selected.kind.canHaveChildren()) selected.id else selected.parentId
    }
}
