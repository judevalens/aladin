package com.jvp.aladin_compose.features.app

import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.model.BreadcrumbItem
import com.jvp.aladin_compose.model.FolderNode
import com.jvp.aladin_compose.model.FolderWorkspace
import com.jvp.aladin_compose.model.NavDestination
import com.slack.circuit.runtime.CircuitUiEvent
import com.slack.circuit.runtime.CircuitUiState

data class AppState(
    val destination: NavDestination,
    val breadcrumbs: List<BreadcrumbItem>,
    val currentParentId: String?,
    val browserFilter: BrowserFilter,
    val nodes: List<FolderNode>,
    val selectedFolderId: String?,
    val selectedArtifact: Artifact?,
    val workspace: FolderWorkspace?,
    val eventSink: (AppEvent) -> Unit,
) : CircuitUiState

enum class BrowserFilter {
    All,
    Folders,
    Artifacts,
}

sealed interface AppEvent : CircuitUiEvent {
    data class NavigateDestination(val destination: NavDestination) : AppEvent
    data class OpenNode(val nodeId: String) : AppEvent
    data class OpenArtifact(val artifactId: String) : AppEvent
    data class ChangeBrowserFilter(val filter: BrowserFilter) : AppEvent
    data class NavigateBreadcrumb(val parentId: String?) : AppEvent
    data object CreateFolder : AppEvent
    data object CreateArtifact : AppEvent
}
