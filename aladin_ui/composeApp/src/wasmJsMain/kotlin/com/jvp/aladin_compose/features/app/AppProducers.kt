package com.jvp.aladin_compose.features.app

import androidx.compose.runtime.Composable
import androidx.compose.runtime.produceState
import com.jvp.aladin_compose.features.app.BrowserFilter.All
import com.jvp.aladin_compose.features.app.BrowserFilter.Artifacts
import com.jvp.aladin_compose.features.app.BrowserFilter.Folders
import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.model.BreadcrumbItem
import com.jvp.aladin_compose.model.Item
import com.jvp.aladin_compose.model.ItemKind
import com.jvp.aladin_compose.model.canHaveChildren
import com.jvp.aladin_compose.service.ItemService

@Composable
fun browserRowsProducer(
    service: ItemService,
    expandedItemIds: Set<String>,
    selectedItemId: String?,
    filter: BrowserFilter,
    refreshKey: Int,
): List<BrowserTreeRow> {
    return produceState(initialValue = emptyList(), service, expandedItemIds, selectedItemId, filter, refreshKey) {
        val items = service.items()
        val artifactByItemId =
            items.filter { it.kind == ItemKind.Artifact }
                .associate { item -> item.id to service.artifactFor(item) }

        value = buildBrowserRows(
            items = items,
            artifactByItemId = artifactByItemId,
            expandedItemIds = expandedItemIds,
            selectedItemId = selectedItemId,
            filter = filter,
        )
    }.value
}

@Composable
fun itemBreadcrumbsProducer(
    service: ItemService,
    itemId: String?,
    refreshKey: Int,
): List<BreadcrumbItem> {
    return produceState(initialValue = emptyList(), service, itemId, refreshKey) {
        value = service.breadcrumbs(itemId)
    }.value
}

@Composable
fun selectedItemProducer(
    service: ItemService,
    itemId: String?,
    refreshKey: Int,
): Item? {
    return produceState<Item?>(initialValue = null, service, itemId, refreshKey) {
        value = service.item(itemId)
    }.value
}

@Composable
fun selectedArtifactProducer(
    service: ItemService,
    item: Item?,
    refreshKey: Int,
): Artifact? {
    return produceState<Artifact?>(initialValue = null, service, item, refreshKey) {
        value = service.artifactFor(item)
    }.value
}

private fun buildBrowserRows(
    items: List<Item>,
    artifactByItemId: Map<String, Artifact?>,
    expandedItemIds: Set<String>,
    selectedItemId: String?,
    filter: BrowserFilter,
): List<BrowserTreeRow> {
    val childrenByParent = items.groupBy { it.parentId }
    val rows = mutableListOf<BrowserTreeRow>()

    fun visit(parentId: String?, depth: Int) {
        childrenByParent[parentId].orEmpty().sortedBy { it.position }.forEach { item ->
            val children = childrenByParent[item.id].orEmpty()
            val expandable = item.kind.canHaveChildren() && children.isNotEmpty()
            val expanded = item.id in expandedItemIds
            val selected = item.id == selectedItemId

            val row =
                when (item.kind) {
                    ItemKind.Artifact ->
                        BrowserTreeRow.Artifact(
                            item = item,
                            artifact = artifactByItemId[item.id],
                            depth = depth,
                            selected = selected,
                        )
                    ItemKind.Folder ->
                        BrowserTreeRow.Folder(
                            item = item,
                            depth = depth,
                            expanded = expanded,
                            expandable = expandable,
                            selected = selected,
                        )
                    ItemKind.Page,
                    ItemKind.Group ->
                        BrowserTreeRow.Generic(
                            item = item,
                            depth = depth,
                            expanded = expanded,
                            expandable = expandable,
                            selected = selected,
                        )
                }

            if (row.matches(filter)) rows += row
            if (expanded && item.kind.canHaveChildren()) visit(item.id, depth + 1)
        }
    }

    visit(parentId = null, depth = 0)
    return rows
}

private fun BrowserTreeRow.matches(filter: BrowserFilter): Boolean {
    return when (filter) {
        All -> true
        Folders -> item.kind.isFolderLike()
        Artifacts -> item.kind == ItemKind.Artifact
    }
}
