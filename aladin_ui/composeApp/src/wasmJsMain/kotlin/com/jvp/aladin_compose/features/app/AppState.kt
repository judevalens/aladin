package com.jvp.aladin_compose.features.app

import com.jvp.aladin_compose.features.app.artifactpane.ArtifactPaneState
import com.jvp.aladin_compose.features.app.browser.DocumentBrowserState
import com.jvp.aladin_compose.features.app.sidebar.SidebarState
import com.slack.circuit.runtime.CircuitUiState

data class AppState(
    val navigation: AppNavigationState,
    val sidebar: SidebarState,
    val browser: DocumentBrowserState,
    val artifactPane: ArtifactPaneState,
) : CircuitUiState
