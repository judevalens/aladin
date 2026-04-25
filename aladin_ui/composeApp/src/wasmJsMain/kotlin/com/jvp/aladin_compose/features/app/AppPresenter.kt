package com.jvp.aladin_compose.features.app

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import com.jvp.aladin_compose.model.NavDestination
import com.jvp.aladin_compose.features.app.BrowserFilter.All
import com.jvp.aladin_compose.service.ItemService
import com.slack.circuit.runtime.CircuitContext
import com.slack.circuit.runtime.Navigator
import com.slack.circuit.runtime.presenter.Presenter
import com.slack.circuit.runtime.screen.Screen
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch

class AppPresenter(
    private val itemService: ItemService,
    private val scope: CoroutineScope,
) : Presenter<AppState> {

    @Composable
    override fun present(): AppState {
        var destination by remember { mutableStateOf(NavDestination.Home) }
        var selectedItemId by remember { mutableStateOf<String?>("item_rivian") }
        var expandedItemIds by remember { mutableStateOf(setOf("item_watch")) }
        var browserFilter by remember { mutableStateOf(All) }
        var refreshKey by remember { mutableStateOf(0) }

        val selectedItem = selectedItemProducer(itemService, selectedItemId, refreshKey)
        val selectedArtifact = selectedArtifactProducer(itemService, selectedItem, refreshKey)
        val breadcrumbs = itemBreadcrumbsProducer(itemService, selectedItemId, refreshKey)
        val browserRows =
            when (destination) {
                NavDestination.Home,
                NavDestination.Folders ->
                    browserRowsProducer(
                        service = itemService,
                        expandedItemIds = expandedItemIds,
                        selectedItemId = selectedItemId,
                        filter = browserFilter,
                        refreshKey = refreshKey,
                    )
                else -> emptyList()
            }

        return AppState(
            destination = destination,
            breadcrumbs = breadcrumbs,
            browserFilter = browserFilter,
            browserRows = browserRows,
            selectedItemId = selectedItemId,
            selectedItem = selectedItem,
            selectedArtifact = selectedArtifact,
            expandedItemIds = expandedItemIds,
            eventSink = { event ->
                when (event) {
                    is AppEvent.NavigateDestination -> destination = event.destination
                    is AppEvent.SelectItem -> {
                        selectedItemId = event.itemId
                        scope.launch {
                            expandedItemIds = expandedItemIds + itemService.ancestorIds(event.itemId)
                        }
                    }
                    is AppEvent.ToggleItemExpanded -> {
                        expandedItemIds =
                            if (event.itemId in expandedItemIds) expandedItemIds - event.itemId
                            else expandedItemIds + event.itemId
                    }
                    is AppEvent.NavigateBreadcrumb -> {
                        selectedItemId = event.itemId
                        scope.launch {
                            expandedItemIds = expandedItemIds + itemService.ancestorIds(event.itemId)
                        }
                    }
                    is AppEvent.ChangeBrowserFilter -> browserFilter = event.filter
                    AppEvent.CreateFolder -> {
                        scope.launch {
                            val parentId = itemService.nearestContainerId(selectedItemId)
                            val created = itemService.createFolder(parentId)
                            selectedItemId = created.id
                            expandedItemIds = expandedItemIds + itemService.ancestorIds(created.id)
                            refreshKey += 1
                        }
                    }
                    AppEvent.CreateArtifact -> {
                        scope.launch {
                            val parentId = itemService.nearestContainerId(selectedItemId)
                            val (createdItem, _) = itemService.createArtifactItem(parentId)
                            selectedItemId = createdItem.id
                            expandedItemIds = expandedItemIds + itemService.ancestorIds(createdItem.id)
                            refreshKey += 1
                        }
                    }
                }
            },
        )
    }

    class Factory(
        private val itemService: ItemService,
        private val scope: CoroutineScope,
    ) : Presenter.Factory {
        override fun create(screen: Screen, navigator: Navigator, context: CircuitContext): Presenter<*>? {
            return when (screen) {
                AppScreen -> AppPresenter(itemService, scope)
                else -> null
            }
        }
    }
}
