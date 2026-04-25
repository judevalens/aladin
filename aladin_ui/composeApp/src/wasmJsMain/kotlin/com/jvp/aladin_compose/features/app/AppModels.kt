package com.jvp.aladin_compose.features.app

import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.model.BreadcrumbItem
import com.jvp.aladin_compose.model.Item
import com.jvp.aladin_compose.model.ItemKind
import com.jvp.aladin_compose.model.NavDestination
import com.slack.circuit.runtime.CircuitUiEvent
import com.slack.circuit.runtime.CircuitUiState

data class AppState(
    val destination: NavDestination,
    val breadcrumbs: List<BreadcrumbItem>,
    val browserFilter: BrowserFilter,
    val browserRows: List<BrowserTreeRow>,
    val selectedItemId: String?,
    val selectedItem: Item?,
    val selectedArtifact: Artifact?,
    val expandedItemIds: Set<String>,
    val eventSink: (AppEvent) -> Unit,
) : CircuitUiState

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

enum class BrowserFilter {
    All,
    Folders,
    Artifacts,
}

sealed interface AppEvent : CircuitUiEvent {
    data class NavigateDestination(val destination: NavDestination) : AppEvent
    data class SelectItem(val itemId: String) : AppEvent
    data class ToggleItemExpanded(val itemId: String) : AppEvent
    data class ChangeBrowserFilter(val filter: BrowserFilter) : AppEvent
    data class NavigateBreadcrumb(val itemId: String?) : AppEvent
    data object CreateFolder : AppEvent
    data object CreateArtifact : AppEvent
}

fun ItemKind.isFolderLike(): Boolean {
    return when (this) {
        ItemKind.Folder,
        ItemKind.Page,
        ItemKind.Group -> true
        ItemKind.Artifact -> false
    }
}
