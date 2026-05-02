package com.jvp.aladin_compose.features.app.artifactpane

import androidx.compose.runtime.Composable
import androidx.compose.runtime.produceState
import com.jvp.aladin_compose.features.app.artifactpane.page.DefaultPageStateProducer
import com.jvp.aladin_compose.features.app.artifactpane.page.PageProducerState
import com.jvp.aladin_compose.features.app.artifactpane.page.PageStateProducer
import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.model.BreadcrumbItem
import com.jvp.aladin_compose.service.ArtifactService
import com.jvp.aladin_compose.service.FolderService
import com.jvp.aladin_compose.service.PageDocumentSyncer
import com.jvp.aladin_compose.service.web.DefaultPageEditorBridge
import com.jvp.aladin_compose.service.web.PageEditorBridge
import kotlinx.coroutines.CoroutineScope

interface ArtifactPaneProducer {
    @Composable
    fun produce(
        activeArtifactId: String?,
        openArtifactIds: List<String>,
        onActivateArtifact: (String) -> Unit,
        onCloseArtifact: (String) -> Unit,
    ): ArtifactPaneState
}

data class ArtifactPaneState(
    val activeArtifact: Artifact?,
    val openArtifacts: List<Artifact>,
    val breadcrumbs: List<BreadcrumbItem>,
    val pages: PageProducerState,
    val pageEditorBridge: PageEditorBridge,
    val eventSink: (ArtifactPaneEvent) -> Unit,
)

sealed interface ArtifactPaneEvent {
    data class ActivateArtifact(val artifactId: String) : ArtifactPaneEvent

    data class CloseArtifact(val artifactId: String) : ArtifactPaneEvent
}

class DefaultArtifactPaneProducer(
    private val folderService: FolderService,
    private val artifactService: ArtifactService,
    private val pageStateProducer: PageStateProducer,
    private val pageEditorBridge: PageEditorBridge,
) : ArtifactPaneProducer {
    @Composable
    override fun produce(
        activeArtifactId: String?,
        openArtifactIds: List<String>,
        onActivateArtifact: (String) -> Unit,
        onCloseArtifact: (String) -> Unit,
    ): ArtifactPaneState {
        val activeArtifact =
            produceState<Artifact?>(initialValue = null, artifactService, activeArtifactId) {
                value =
                    try {
                        artifactService.artifact(activeArtifactId)
                    } catch (_: Throwable) {
                        null
                    }
            }.value

        val openArtifacts =
            produceState(initialValue = emptyList(), artifactService, openArtifactIds) {
                value =
                    openArtifactIds.mapNotNull { artifactId ->
                        try {
                            artifactService.artifact(artifactId)
                        } catch (_: Throwable) {
                            null
                        }
                    }
            }.value

        val breadcrumbs =
            produceState(initialValue = emptyList(), folderService, activeArtifactId) {
                value =
                    try {
                        folderService.artifactBreadcrumbs(activeArtifactId)
                    } catch (_: Throwable) {
                        emptyList()
                    }
            }.value

        val pages =
            pageStateProducer.produce(
                openArtifacts = openArtifacts,
                activeArtifactId = activeArtifactId,
            )

        return ArtifactPaneState(
            activeArtifact = activeArtifact,
            openArtifacts = openArtifacts,
            breadcrumbs = breadcrumbs,
            pages = pages,
            pageEditorBridge = pageEditorBridge,
            eventSink = { event ->
                when (event) {
                    is ArtifactPaneEvent.ActivateArtifact -> onActivateArtifact(event.artifactId)
                    is ArtifactPaneEvent.CloseArtifact -> onCloseArtifact(event.artifactId)
                }
            },
        )
    }
}

fun defaultArtifactPaneProducer(
    folderService: FolderService,
    artifactService: ArtifactService,
    pageDocumentSyncer: PageDocumentSyncer,
    scope: CoroutineScope,
): ArtifactPaneProducer {
    val pageEditorBridge = DefaultPageEditorBridge()
    return DefaultArtifactPaneProducer(
        folderService = folderService,
        artifactService = artifactService,
        pageStateProducer =
            DefaultPageStateProducer(
                artifactService = artifactService,
                pageDocumentSyncer = pageDocumentSyncer,
                scope = scope,
                pageEditorBridge = pageEditorBridge,
            ),
        pageEditorBridge = pageEditorBridge,
    )
}
