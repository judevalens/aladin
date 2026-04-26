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

private const val DrillInDepth = 2

interface DocumentBrowserProducer {
    @Composable fun produce(): DocumentBrowserProducerState
}

class DefaultDocumentBrowserProducer(
    private val service: ItemService,
    private val scope: CoroutineScope,
) : DocumentBrowserProducer {
    @Composable
    override fun produce(): DocumentBrowserProducerState {
        var selectedItemId by remember { mutableStateOf<String?>(null) }
        var currentScopeId by remember { mutableStateOf<String?>(null) }
        var scopeBackStack by remember { mutableStateOf<List<String?>>(emptyList()) }
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
                    currentScopeId,
                    scopeBackStack,
                    refreshKey,
                ) {
                    val items = service.items()
                    val artifactByItemId =
                        items
                            .filter { it.kind == ItemKind.Artifact }
                            .associate { item -> item.id to service.artifactFor(item) }
                    val currentScope = currentScopeId?.let { id -> items.firstOrNull { it.id == id } }

                    value =
                        buildBrowserRows(
                            items = items,
                            artifactByItemId = artifactByItemId,
                            expandedItemIds = expandedItemIds,
                            selectedItemId = selectedItemId,
                            currentScope = currentScope,
                            backScopeId = scopeBackStack.lastOrNull(),
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
                                if (event.depth >= DrillInDepth) {
                                    scopeBackStack = scopeBackStack + currentScopeId
                                    currentScopeId = event.itemId
                                    selectedItemId = event.itemId
                                    expandedItemIds = expandedItemIds + event.itemId
                                } else {
                                    expandedItemIds =
                                        if (event.itemId in expandedItemIds)
                                            expandedItemIds - event.itemId
                                        else expandedItemIds + event.itemId
                                }
                            }
                            is DocumentBrowserEvent.NavigateScope -> {
                                currentScopeId = scopeBackStack.lastOrNull() ?: event.itemId
                                scopeBackStack = scopeBackStack.dropLast(1)
                                selectedItemId = event.itemId ?: selectedItemId
                            }
                            is DocumentBrowserEvent.NavigateBreadcrumb -> {
                                selectedItemId = event.itemId
                                scope.launch {
                                    currentScopeId = service.nearestContainerId(event.itemId)
                                    scopeBackStack = emptyList()
                                    expandedItemIds =
                                        expandedItemIds + service.ancestorIds(event.itemId)
                                }
                            }
                            DocumentBrowserEvent.CreateFolder -> {
                                scope.launch {
                                    val parentId = service.nearestContainerId(selectedItemId)
                                    val created = service.createFolder(parentId)
                                    selectedItemId = created.id
                                    currentScopeId = parentId
                                    scopeBackStack = emptyList()
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
                                    currentScopeId = parentId
                                    scopeBackStack = emptyList()
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
        currentScope: Item?,
        backScopeId: String?,
    ): List<BrowserTreeRow> {
        val childrenByParent = items.groupBy { it.parentId }
        val itemById = items.associateBy { it.id }
        val rows = mutableListOf<BrowserTreeRow>()

        fun addItemRow(item: Item, depth: Int) {
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
        }

        fun visit(parentId: String?, depth: Int, remainingDepth: Int) {
            childrenByParent[parentId]
                .orEmpty()
                .sortedBy { it.position }
                .forEach { item ->
                    addItemRow(item = item, depth = depth)
                    if (remainingDepth > 0 && item.id in expandedItemIds && item.kind.canHaveChildren()) {
                        visit(item.id, depth + 1, remainingDepth - 1)
                    }
                }
        }

        if (currentScope != null) {
            rows += BrowserTreeRow.ScopeBack(
                targetScopeId = backScopeId,
                label = currentScope.title,
            )
        }

        visit(parentId = currentScope?.id, depth = 0, remainingDepth = DrillInDepth)
        return rows
    }
}
