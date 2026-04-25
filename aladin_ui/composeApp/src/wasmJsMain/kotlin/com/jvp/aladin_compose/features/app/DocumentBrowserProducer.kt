package com.jvp.aladin_compose.features.app

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.model.Item
import com.jvp.aladin_compose.model.ItemKind
import com.jvp.aladin_compose.model.canHaveChildren
import com.jvp.aladin_compose.service.ItemService
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch

interface DocumentBrowserProducer {
    @Composable fun produce(): DocumentBrowserProducerState
}

class DefaultDocumentBrowserProducer(
    private val service: ItemService,
    private val scope: CoroutineScope,
) : DocumentBrowserProducer {
    @Composable
    override fun produce(): DocumentBrowserProducerState {
        var selectedItemId by remember { mutableStateOf<String?>("item_rivian") }
        var expandedItemIds by remember { mutableStateOf(setOf("item_watch")) }
        var refreshKey by remember { mutableStateOf(0) }

        val selectedItem =
            produceState<Item?>(initialValue = null, service, selectedItemId, refreshKey) {
                    value = service.item(selectedItemId)
                }
                .value
        val selectedArtifact =
            produceState<Artifact?>(initialValue = null, service, selectedItem, refreshKey) {
                    value = service.artifactFor(selectedItem)
                }
                .value
        val breadcrumbs =
            produceState(initialValue = emptyList(), service, selectedItemId, refreshKey) {
                    value = service.breadcrumbs(selectedItemId)
                }
                .value
        val rows =
            produceState(
                    initialValue = emptyList(),
                    service,
                    expandedItemIds,
                    selectedItemId,
                    refreshKey,
                ) {
                    val items = service.items()
                    val artifactByItemId =
                        items
                            .filter { it.kind == ItemKind.Artifact }
                            .associate { item -> item.id to service.artifactFor(item) }

                    value =
                        buildBrowserRows(
                            items = items,
                            artifactByItemId = artifactByItemId,
                            expandedItemIds = expandedItemIds,
                            selectedItemId = selectedItemId,
                        )
                }
                .value

        return DocumentBrowserProducerState(
            browser =
                DocumentBrowserState(
                    breadcrumbs = breadcrumbs,
                    rows = rows,
                    eventSink = { event ->
                        when (event) {
                            is DocumentBrowserEvent.SelectItem -> {
                                selectedItemId = event.itemId
                                scope.launch {
                                    expandedItemIds =
                                        expandedItemIds + service.ancestorIds(event.itemId)
                                }
                            }
                            is DocumentBrowserEvent.ToggleItemExpanded -> {
                                expandedItemIds =
                                    if (event.itemId in expandedItemIds)
                                        expandedItemIds - event.itemId
                                    else expandedItemIds + event.itemId
                            }
                            is DocumentBrowserEvent.NavigateBreadcrumb -> {
                                selectedItemId = event.itemId
                                scope.launch {
                                    expandedItemIds =
                                        expandedItemIds + service.ancestorIds(event.itemId)
                                }
                            }
                            DocumentBrowserEvent.CreateFolder -> {
                                scope.launch {
                                    val parentId = service.nearestContainerId(selectedItemId)
                                    val created = service.createFolder(parentId)
                                    selectedItemId = created.id
                                    expandedItemIds =
                                        expandedItemIds + service.ancestorIds(created.id)
                                    refreshKey += 1
                                }
                            }
                            DocumentBrowserEvent.CreateArtifact -> {
                                scope.launch {
                                    val parentId = service.nearestContainerId(selectedItemId)
                                    val (createdItem, _) = service.createArtifactItem(parentId)
                                    selectedItemId = createdItem.id
                                    expandedItemIds =
                                        expandedItemIds + service.ancestorIds(createdItem.id)
                                    refreshKey += 1
                                }
                            }
                        }
                    },
                ),
            selectedItem = selectedItem,
            selectedArtifact = selectedArtifact,
            canCreateArtifact = selectedItemId != null,
        )
    }

    private fun buildBrowserRows(
        items: List<Item>,
        artifactByItemId: Map<String, Artifact?>,
        expandedItemIds: Set<String>,
        selectedItemId: String?,
    ): List<BrowserTreeRow> {
        val childrenByParent = items.groupBy { it.parentId }
        val rows = mutableListOf<BrowserTreeRow>()

        fun visit(parentId: String?, depth: Int) {
            childrenByParent[parentId]
                .orEmpty()
                .sortedBy { it.position }
                .forEach { item ->
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

                    rows += row
                    if (expanded && item.kind.canHaveChildren()) visit(item.id, depth + 1)
                }
        }

        visit(parentId = null, depth = 0)
        return rows
    }
}
