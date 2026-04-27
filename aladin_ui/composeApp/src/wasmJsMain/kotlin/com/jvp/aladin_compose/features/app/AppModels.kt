package com.jvp.aladin_compose.features.app

import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.model.ArtifactPreview
import com.jvp.aladin_compose.model.BreadcrumbItem
import com.jvp.aladin_compose.model.FolderNode
import com.jvp.aladin_compose.model.NavDestination
import com.slack.circuit.runtime.CircuitUiEvent
import com.slack.circuit.runtime.CircuitUiState

data class AppState(
    val destination: NavDestination,
    val browser: DocumentBrowserState,
    val selectedFolder: FolderNode?,
    val selectedArtifact: Artifact?,
    val canCreateArtifact: Boolean,
    val eventSink: (AppEvent) -> Unit,
) : CircuitUiState

data class DocumentBrowserProducerState(
    val browser: DocumentBrowserState,
    val selectedFolder: FolderNode?,
    val selectedArtifact: Artifact?,
    val canCreateArtifact: Boolean,
)

data class DocumentBrowserState(
    val breadcrumbs: List<BreadcrumbItem>,
    val scopeBreadcrumbs: List<BreadcrumbItem>,
    val canNavigateScopeBack: Boolean,
    val scopeBackTargetId: String?,
    val loading: Boolean,
    val errorMessage: String?,
    val rows: List<BrowserTreeRow>,
    val eventSink: (DocumentBrowserEvent) -> Unit,
)

sealed interface BrowserTreeRow {
    val key: String
    val depth: Int
    val selected: Boolean

    data class Folder(
        val folder: FolderNode,
        override val depth: Int,
        val expanded: Boolean,
        val expandable: Boolean,
        override val selected: Boolean,
    ) : BrowserTreeRow {
        override val key: String = "folder_${folder.id}"
    }

    data class Artifact(
        val artifact: ArtifactPreview,
        override val depth: Int,
        override val selected: Boolean,
    ) : BrowserTreeRow {
        override val key: String = "artifact_${artifact.id}"
    }
}

sealed interface AppEvent : CircuitUiEvent {
    data class NavigateDestination(val destination: NavDestination) : AppEvent
}

sealed interface DocumentBrowserEvent {
    data class SelectFolder(val folderId: String) : DocumentBrowserEvent
    data class SelectArtifact(val artifactId: String) : DocumentBrowserEvent
    data class ToggleFolderExpanded(val folderId: String, val depth: Int) : DocumentBrowserEvent
    data class NavigateScope(val folderId: String?) : DocumentBrowserEvent
    data class NavigateBreadcrumb(val folderId: String?) : DocumentBrowserEvent
    data object CreateFolder : DocumentBrowserEvent
    data object CreateArtifact : DocumentBrowserEvent
    data object RetryLoad : DocumentBrowserEvent
}
