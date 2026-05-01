package com.jvp.aladin_compose.features.app.artifactpane

import androidx.compose.runtime.Composable
import androidx.compose.runtime.produceState
import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.model.BreadcrumbItem
import com.jvp.aladin_compose.service.FolderService

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
    val eventSink: (ArtifactPaneEvent) -> Unit,
)

sealed interface ArtifactPaneEvent {
    data class ActivateArtifact(val artifactId: String) : ArtifactPaneEvent

    data class CloseArtifact(val artifactId: String) : ArtifactPaneEvent
}

class DefaultArtifactPaneProducer(
    private val service: FolderService,
) : ArtifactPaneProducer {
    @Composable
    override fun produce(
        activeArtifactId: String?,
        openArtifactIds: List<String>,
        onActivateArtifact: (String) -> Unit,
        onCloseArtifact: (String) -> Unit,
    ): ArtifactPaneState {
        val activeArtifact =
            produceState<Artifact?>(initialValue = null, service, activeArtifactId) {
                value =
                    try {
                        service.artifact(activeArtifactId)
                    } catch (_: Throwable) {
                        null
                    }
            }.value

        val openArtifacts =
            produceState(initialValue = emptyList(), service, openArtifactIds) {
                value =
                    openArtifactIds.mapNotNull { artifactId ->
                        try {
                            service.artifact(artifactId)
                        } catch (_: Throwable) {
                            null
                        }
                    }
            }.value

        val breadcrumbs =
            produceState(initialValue = emptyList(), service, activeArtifactId) {
                value =
                    try {
                        service.artifactBreadcrumbs(activeArtifactId)
                    } catch (_: Throwable) {
                        emptyList()
                    }
            }.value

        return ArtifactPaneState(
            activeArtifact = activeArtifact,
            openArtifacts = openArtifacts,
            breadcrumbs = breadcrumbs,
            eventSink = { event ->
                when (event) {
                    is ArtifactPaneEvent.ActivateArtifact -> onActivateArtifact(event.artifactId)
                    is ArtifactPaneEvent.CloseArtifact -> onCloseArtifact(event.artifactId)
                }
            },
        )
    }
}
