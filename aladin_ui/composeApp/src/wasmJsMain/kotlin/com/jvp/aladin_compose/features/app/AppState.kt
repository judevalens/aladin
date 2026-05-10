package com.jvp.aladin_compose.features.app

import com.jvp.aladin_compose.features.app.artifactpane.ArtifactPaneState
import com.jvp.aladin_compose.features.app.browser.DocumentBrowserState
import com.jvp.aladin_compose.features.app.sidebar.SidebarState
import com.jvp.aladin_compose.service.AuthenticatedUser
import com.slack.circuit.runtime.CircuitUiState

data class AppState(
    val user: AuthenticatedUser,
    val navigation: AppNavigationState,
    val sidebar: SidebarState,
    val browser: DocumentBrowserState,
    val artifactPane: ArtifactPaneState,
    val eventSink: (AppEvent) -> Unit,
) : CircuitUiState

sealed interface AppEvent {
    data object Logout : AppEvent
}
