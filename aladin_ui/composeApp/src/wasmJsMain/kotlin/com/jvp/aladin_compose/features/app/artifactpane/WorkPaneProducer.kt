package com.jvp.aladin_compose.features.app.artifactpane

import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.produceState
import com.jvp.aladin_compose.features.app.artifactpane.page.PageProducerState
import com.jvp.aladin_compose.features.app.artifactpane.page.PageStateProducer
import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.model.BreadcrumbItem
import com.jvp.aladin_compose.repo.ArtifactRepository
import com.jvp.aladin_compose.service.web.PageEditorBridge

interface WorkPaneProducer {
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

class WorkPaneProducerImpl(
    private val artifactRepository: ArtifactRepository,
    private val pageStateProducer: PageStateProducer,
    private val pageEditorBridge: PageEditorBridge,
) : WorkPaneProducer {
    @Composable
    override fun produce(
        activeArtifactId: String?,
        openArtifactIds: List<String>,
        onActivateArtifact: (String) -> Unit,
        onCloseArtifact: (String) -> Unit,
    ): ArtifactPaneState {
        val activeArtifact by
            produceState(null as Artifact?, activeArtifactId) {
                activeArtifactId?.let {
                    artifactRepository.artifact(activeArtifactId).collect { value = it }
                }
            }

        val openArtifacts by
            artifactRepository.artifacts(openArtifactIds).collectAsState(initial = emptyList())

        val breadcrumbs by
            artifactRepository
                .observeArtifactBreadcrumbs(activeArtifactId)
                .collectAsState(initial = emptyList())

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
