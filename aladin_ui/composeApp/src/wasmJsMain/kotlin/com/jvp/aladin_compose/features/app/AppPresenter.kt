package com.jvp.aladin_compose.features.app

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import com.jvp.aladin_compose.model.NavDestination
import com.jvp.aladin_compose.features.app.BrowserFilter.All
import com.jvp.aladin_compose.service.FolderService
import com.slack.circuit.runtime.CircuitContext
import com.slack.circuit.runtime.Navigator
import com.slack.circuit.runtime.presenter.Presenter
import com.slack.circuit.runtime.screen.Screen
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch

class AppPresenter(
    private val folderService: FolderService,
    private val scope: CoroutineScope,
) : Presenter<AppState> {

    @Composable
    override fun present(): AppState {
        var destination by remember { mutableStateOf(NavDestination.Home) }
        var currentParentId by remember { mutableStateOf<String?>(null) }
        var selectedFolderId by remember { mutableStateOf("folder_rivian") }
        var selectedArtifactId by remember { mutableStateOf<String?>(null) }
        var browserFilter by remember { mutableStateOf(All) }
        var refreshKey by remember { mutableStateOf(0) }

        val breadcrumbs = folderBreadcrumbsProducer(folderService, currentParentId, refreshKey)
        val nodes = folderChildrenProducer(folderService, currentParentId, refreshKey)
        val selectedWorkspace = folderWorkspaceProducer(folderService, selectedFolderId, refreshKey)

        return AppState(
            destination = destination,
            breadcrumbs = breadcrumbs,
            currentParentId = currentParentId,
            browserFilter = browserFilter,
            nodes = when (destination) {
                NavDestination.Home, NavDestination.Folders -> nodes
                else -> emptyList()
            },
            selectedFolderId = selectedFolderId,
            selectedArtifact = selectedWorkspace?.artifacts?.firstOrNull { it.id == selectedArtifactId },
            workspace = selectedWorkspace,
            eventSink = { event ->
                when (event) {
                    is AppEvent.NavigateDestination -> destination = event.destination
                    is AppEvent.OpenNode -> {
                        val node = nodes.firstOrNull { it.id == event.nodeId }
                        if (node != null) {
                            currentParentId = node.id
                            selectedFolderId = node.id
                            selectedArtifactId = null
                        }
                    }
                    is AppEvent.NavigateBreadcrumb -> currentParentId = event.parentId
                    is AppEvent.OpenArtifact -> selectedArtifactId = event.artifactId
                    is AppEvent.ChangeBrowserFilter -> browserFilter = event.filter
                    AppEvent.CreateFolder -> {
                        scope.launch {
                            val created = folderService.createFolder(currentParentId)
                            selectedFolderId = created.id
                            selectedArtifactId = null
                            refreshKey += 1
                        }
                    }
                    AppEvent.CreateArtifact -> {
                        scope.launch {
                            val created = folderService.createArtifact(selectedFolderId)
                            selectedArtifactId = created.id
                            refreshKey += 1
                        }
                    }
                }
            },
        )
    }

    class Factory(
        private val folderService: FolderService,
        private val scope: CoroutineScope,
    ) : Presenter.Factory {
        override fun create(screen: Screen, navigator: Navigator, context: CircuitContext): Presenter<*>? {
            return when (screen) {
                AppScreen -> AppPresenter(folderService, scope)
                else -> null
            }
        }
    }
}
