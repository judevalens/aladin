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
    val focusedFolder: FolderNode?,
    val activeArtifact: Artifact?,
    val openArtifacts: List<Artifact>,
    val canCreateArtifact: Boolean,
    val eventSink: (AppEvent) -> Unit,
) : CircuitUiState

data class DocumentBrowserProducerState(
    val browser: DocumentBrowserState,
    val focusedFolder: FolderNode?,
    val activeArtifact: Artifact?,
    val openArtifacts: List<Artifact>,
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

enum class BrowserCreateOption {
    Folder,
    Note,
    Link,
    Voice,
    Upload,
}

enum class BrowserRowKind {
    Folder,
    Artifact,
}

enum class BrowserRowMenuActionTone {
    Normal,
    Secondary,
    Destructive,
}

data class BrowserRowMenuAction(
    val id: String,
    val label: String,
    val enabled: Boolean = true,
    val tone: BrowserRowMenuActionTone = BrowserRowMenuActionTone.Normal,
)

data class BrowserRowMenuSection(
    val title: String,
    val actions: List<BrowserRowMenuAction>,
)

data class BrowserRowMenuModel(
    val rowId: String,
    val rowKind: BrowserRowKind,
    val sections: List<BrowserRowMenuSection>,
)

enum class BrowserMenuPlacement {
    ContextualRight,
    DropdownBelow,
}

data class BrowserRowMenuRequest(
    val menu: BrowserRowMenuModel,
    val anchorLeftPx: Float,
    val anchorRightPx: Float,
    val anchorBottomPx: Float,
    val placement: BrowserMenuPlacement = BrowserMenuPlacement.ContextualRight,
    val matchAnchorWidth: Boolean = false,
    val elevated: Boolean = false,
    val onActionSelected: (String) -> Unit,
)

sealed interface BrowserTreeRow {
    val key: String
    val depth: Int
    val selected: Boolean
    val menu: BrowserRowMenuModel

    data class Folder(
        val folder: FolderNode,
        override val depth: Int,
        val expanded: Boolean,
        val expandable: Boolean,
        override val selected: Boolean,
        override val menu: BrowserRowMenuModel,
    ) : BrowserTreeRow {
        override val key: String = "folder_${folder.id}"
    }

    data class Artifact(
        val artifact: ArtifactPreview,
        override val depth: Int,
        override val selected: Boolean,
        override val menu: BrowserRowMenuModel,
    ) : BrowserTreeRow {
        override val key: String = "artifact_${artifact.id}"
    }
}

sealed interface AppEvent : CircuitUiEvent {
    data class NavigateDestination(val destination: NavDestination) : AppEvent
}

sealed interface DocumentBrowserEvent {
    data class FocusFolder(val folderId: String) : DocumentBrowserEvent
    data class OpenArtifact(val artifactId: String) : DocumentBrowserEvent
    data class ActivateArtifactTab(val artifactId: String) : DocumentBrowserEvent
    data class ToggleFolderExpanded(val folderId: String, val depth: Int) : DocumentBrowserEvent
    data class NavigateScope(val folderId: String?) : DocumentBrowserEvent
    data class NavigateBreadcrumb(val folderId: String?) : DocumentBrowserEvent
    data class CreateInFolder(val folderId: String, val option: BrowserCreateOption) : DocumentBrowserEvent
    data object CreateFolder : DocumentBrowserEvent
    data object CreateArtifact : DocumentBrowserEvent
    data object RetryLoad : DocumentBrowserEvent
}
