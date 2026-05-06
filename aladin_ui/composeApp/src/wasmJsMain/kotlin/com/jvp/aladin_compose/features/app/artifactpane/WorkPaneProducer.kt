package com.jvp.aladin_compose.features.app.artifactpane

import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import com.jvp.aladin_compose.features.app.artifactpane.link.LinkProducerState
import com.jvp.aladin_compose.features.app.artifactpane.link.LinkStateProducer
import com.jvp.aladin_compose.features.app.artifactpane.page.PageProducerState
import com.jvp.aladin_compose.features.app.artifactpane.page.PageStateProducer
import com.jvp.aladin_compose.features.app.artifactpane.voice.VoiceProducerState
import com.jvp.aladin_compose.features.app.artifactpane.voice.VoiceStateProducer
import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.model.ArtifactKind
import com.jvp.aladin_compose.model.BreadcrumbItem
import com.jvp.aladin_compose.repo.ArtifactInsight
import com.jvp.aladin_compose.repo.ArtifactRepository
import com.jvp.aladin_compose.service.web.PageEditorBridge
import kotlinx.coroutines.flow.flowOf

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
    val links: LinkProducerState,
    val voices: VoiceProducerState,
    val activeInsight: ArtifactInsight?,
    val inspectorOpen: Boolean,
    val pageEditorBridge: PageEditorBridge,
    val eventSink: (ArtifactPaneEvent) -> Unit,
)

sealed interface ArtifactPaneEvent {
    data class ActivateArtifact(val artifactId: String) : ArtifactPaneEvent

    data class CloseArtifact(val artifactId: String) : ArtifactPaneEvent

    data object ToggleInspector : ArtifactPaneEvent
}

class WorkPaneProducerImpl(
    private val artifactRepository: ArtifactRepository,
    private val pageStateProducer: PageStateProducer,
    private val linkStateProducer: LinkStateProducer,
    private val voiceStateProducer: VoiceStateProducer,
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

        var inspectorOverrides by remember { mutableStateOf(emptyMap<String, Boolean>()) }
        val inspectorOpen =
            activeArtifact?.let { artifact ->
                inspectorOverrides[artifact.id] ?: (artifact.kind == ArtifactKind.Link || artifact.kind == ArtifactKind.Voice)
            } ?: false

        val activeInsightFlow =
            activeArtifact?.let(artifactRepository::artifactInsight) ?: flowOf(null)
        val activeInsight by activeInsightFlow.collectAsState(initial = null)

        val pages =
            pageStateProducer.produce(
                openArtifacts = openArtifacts,
                activeArtifactId = activeArtifactId,
            )
        val links =
            linkStateProducer.produce(
                openArtifacts = openArtifacts,
                activeArtifactId = activeArtifactId,
            )
        val voices =
            voiceStateProducer.produce(
                openArtifacts = openArtifacts,
                activeArtifactId = activeArtifactId,
            )

        return ArtifactPaneState(
            activeArtifact = activeArtifact,
            openArtifacts = openArtifacts,
            breadcrumbs = breadcrumbs,
            pages = pages,
            links = links,
            voices = voices,
            activeInsight = activeInsight,
            inspectorOpen = inspectorOpen,
            pageEditorBridge = pageEditorBridge,
            eventSink = { event ->
                when (event) {
                    is ArtifactPaneEvent.ActivateArtifact -> onActivateArtifact(event.artifactId)
                    is ArtifactPaneEvent.CloseArtifact -> onCloseArtifact(event.artifactId)
                    ArtifactPaneEvent.ToggleInspector -> {
                        val artifact = activeArtifact ?: return@ArtifactPaneState
                        inspectorOverrides =
                            inspectorOverrides + (artifact.id to !inspectorOpen)
                    }
                }
            },
        )
    }
}
