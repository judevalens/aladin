package com.jvp.aladin_compose.features.app

import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.model.BreadcrumbItem
import com.jvp.aladin_compose.model.Item
import com.jvp.aladin_compose.model.NavDestination
import com.slack.circuit.runtime.CircuitUiEvent
import com.slack.circuit.runtime.CircuitUiState

data class AppState(
    val destination: NavDestination,
    val browser: DocumentBrowserState,
    val selectedItem: Item?,
    val selectedArtifact: Artifact?,
    val canCreateArtifact: Boolean,
    val eventSink: (AppEvent) -> Unit,
) : CircuitUiState

data class DocumentBrowserProducerState(
    val browser: DocumentBrowserState,
    val selectedItem: Item?,
    val selectedArtifact: Artifact?,
    val canCreateArtifact: Boolean,
)

data class DocumentBrowserState(
    val breadcrumbs: List<BreadcrumbItem>,
    val rows: List<BrowserTreeRow>,
    val eventSink: (DocumentBrowserEvent) -> Unit,
)

sealed interface BrowserTreeRow {
    val key: String
    val item: Item
    val depth: Int
    val selected: Boolean

    data class Folder(
        override val item: Item,
        override val depth: Int,
        val expanded: Boolean,
        val expandable: Boolean,
        override val selected: Boolean,
    ) : BrowserTreeRow {
        override val key: String = item.id
    }

    data class Artifact(
        override val item: Item,
        val artifact: com.jvp.aladin_compose.model.Artifact?,
        override val depth: Int,
        override val selected: Boolean,
    ) : BrowserTreeRow {
        override val key: String = item.id
    }

    data class Generic(
        override val item: Item,
        override val depth: Int,
        val expanded: Boolean,
        val expandable: Boolean,
        override val selected: Boolean,
    ) : BrowserTreeRow {
        override val key: String = item.id
    }
}

sealed interface AppEvent : CircuitUiEvent {
    data class NavigateDestination(val destination: NavDestination) : AppEvent
}

sealed interface DocumentBrowserEvent {
    data class SelectItem(val itemId: String) : DocumentBrowserEvent
    data class ToggleItemExpanded(val itemId: String) : DocumentBrowserEvent
    data class NavigateBreadcrumb(val itemId: String?) : DocumentBrowserEvent
    data object CreateFolder : DocumentBrowserEvent
    data object CreateArtifact : DocumentBrowserEvent
}
