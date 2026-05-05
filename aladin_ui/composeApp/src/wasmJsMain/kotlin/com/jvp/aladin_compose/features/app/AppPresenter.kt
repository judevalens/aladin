package com.jvp.aladin_compose.features.app

import androidx.compose.runtime.*
import com.jvp.aladin_compose.features.app.artifactpane.WorkPaneProducer
import com.jvp.aladin_compose.features.app.browser.DocumentBrowserEvent
import com.jvp.aladin_compose.features.app.browser.DocumentBrowserProducer
import com.jvp.aladin_compose.features.app.sidebar.SidebarProducer
import com.jvp.aladin_compose.model.NavDestination
import com.slack.circuit.runtime.CircuitContext
import com.slack.circuit.runtime.Navigator
import com.slack.circuit.runtime.presenter.Presenter
import com.slack.circuit.runtime.screen.Screen

class AppPresenter(
    private val sidebarProducer: SidebarProducer,
    private val documentBrowserProducer: DocumentBrowserProducer,
    private val artifactPaneProducer: WorkPaneProducer,
) : Presenter<AppState> {

    @Composable
    override fun present(): AppState {

        var destination by remember { mutableStateOf(NavDestination.Home) }
        var activeArtifactId by remember { mutableStateOf<String?>(null) }
        var openArtifactIds by remember { mutableStateOf<List<String>>(emptyList()) }

        val browser =
            documentBrowserProducer.produce(activeArtifactId = activeArtifactId) { artifactId ->
                if (artifactId !in openArtifactIds) {
                    openArtifactIds = openArtifactIds + artifactId
                }
                activeArtifactId = artifactId
            }

        val sidebar =
            sidebarProducer.produce(
                selectedDestination = destination,
                canCreateArtifact = true,
                onNavigate = { destination = it },
                onCreateFolder = { browser.eventSink(DocumentBrowserEvent.CreateFolder) },
                onCreateArtifact = { browser.eventSink(DocumentBrowserEvent.CreateArtifact) },
            )

        val artifactPane =
            artifactPaneProducer.produce(
                activeArtifactId = activeArtifactId,
                openArtifactIds = openArtifactIds,
                onActivateArtifact = { activeArtifactId = it },
                onCloseArtifact = { artifactId ->
                    val remaining = openArtifactIds.filterNot { it == artifactId }
                    openArtifactIds = remaining
                    if (activeArtifactId == artifactId) {
                        activeArtifactId = remaining.lastOrNull()
                    }
                },
            )

        return AppState(sidebar = sidebar, browser = browser, artifactPane = artifactPane)
    }

    class Factory(
        private val sidebarProducer: SidebarProducer,
        private val documentBrowserProducer: DocumentBrowserProducer,
        private val artifactPaneProducer: WorkPaneProducer,
    ) : Presenter.Factory {
        override fun create(screen: Screen, navigator: Navigator, context: CircuitContext): Presenter<*>? {
            return when (screen) {
                AppScreen ->
                    AppPresenter(
                        sidebarProducer = sidebarProducer,
                        documentBrowserProducer = documentBrowserProducer,
                        artifactPaneProducer = artifactPaneProducer,
                    )
                else -> null
            }
        }
    }
}
