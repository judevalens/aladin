package com.jvp.aladin_compose.features.app.artifactpane.voice

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.produceState
import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.model.ArtifactKind
import com.jvp.aladin_compose.repo.ArtifactRepository
import com.jvp.aladin_compose.repo.VoiceArtifactContent

interface VoiceStateProducer {
    @Composable
    fun produce(openArtifacts: List<Artifact>, activeArtifactId: String?): VoiceProducerState
}

data class VoiceProducerState(
    val activeVoice: VoiceArtifactState?,
    val eventSink: (VoiceArtifactEvent) -> Unit,
)

data class VoiceArtifactState(
    val artifact: Artifact,
    val content: VoiceArtifactContent?,
    val isLoading: Boolean,
    val errorMessage: String?,
)

sealed interface VoiceArtifactEvent {
    data object RetryLoad : VoiceArtifactEvent
}

class VoiceStateProducerImpl(private val artifactRepository: ArtifactRepository) :
    VoiceStateProducer {
    @Composable
    override fun produce(
        openArtifacts: List<Artifact>,
        activeArtifactId: String?,
    ): VoiceProducerState {
        val activeVoice =
            openArtifacts.firstOrNull { it.id == activeArtifactId && it.kind == ArtifactKind.Voice }

        val content by
            produceState<VoiceArtifactContent?>(null, activeVoice?.id) {
                val artifact = activeVoice ?: return@produceState
                artifactRepository.voiceContent(artifact).collect { value = it }
            }

        return VoiceProducerState(
            activeVoice =
                activeVoice?.let {
                    VoiceArtifactState(
                        artifact = it,
                        content = content,
                        isLoading = content == null,
                        errorMessage = null,
                    )
                },
            eventSink = {},
        )
    }
}
