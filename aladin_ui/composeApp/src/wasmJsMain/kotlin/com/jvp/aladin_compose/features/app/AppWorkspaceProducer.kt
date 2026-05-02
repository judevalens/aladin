package com.jvp.aladin_compose.features.app

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import com.jvp.aladin_compose.features.app.artifactpane.ArtifactPaneProducer
import com.jvp.aladin_compose.features.app.artifactpane.defaultArtifactPaneProducer
import com.jvp.aladin_compose.features.app.browser.DefaultDocumentBrowserProducer
import com.jvp.aladin_compose.features.app.browser.DocumentBrowserEvent
import com.jvp.aladin_compose.features.app.browser.DocumentBrowserProducer
import com.jvp.aladin_compose.features.app.sidebar.DefaultSidebarProducer
import com.jvp.aladin_compose.features.app.sidebar.SidebarProducer
import com.jvp.aladin_compose.model.NavDestination
import com.jvp.aladin_compose.repo.ApiArtifactRepository
import com.jvp.aladin_compose.service.FolderService
import com.jvp.aladin_compose.service.ArtifactService
import com.jvp.aladin_compose.service.DefaultPageDocumentSyncer
import kotlinx.coroutines.CoroutineScope

interface AppWorkspaceProducer {
    @Composable fun produce(): AppState
}

class DefaultAppWorkspaceProducer(
    private val sidebarProducer: SidebarProducer,
    private val documentBrowserProducer: DocumentBrowserProducer,
    private val artifactPaneProducer: ArtifactPaneProducer,
) : AppWorkspaceProducer {
    @Composable
    override fun produce(): AppState {
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

        return AppState(
            sidebar = sidebar,
            browser = browser,
            artifactPane = artifactPane,
        )
    }
}

fun defaultAppWorkspaceProducer(
    folderService: FolderService,
    scope: CoroutineScope,
): AppWorkspaceProducer {
    val artifactService = ArtifactService(ApiArtifactRepository())
    val pageDocumentSyncer = DefaultPageDocumentSyncer(artifactService, scope)
    return DefaultAppWorkspaceProducer(
        sidebarProducer = DefaultSidebarProducer(),
        documentBrowserProducer = DefaultDocumentBrowserProducer(folderService, artifactService, scope),
        artifactPaneProducer =
            defaultArtifactPaneProducer(
                folderService = folderService,
                artifactService = artifactService,
                pageDocumentSyncer = pageDocumentSyncer,
                scope = scope,
            ),
    )
}
