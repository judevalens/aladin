package com.jvp.aladin_compose.features.app.artifactpane.link

import androidx.compose.runtime.Composable
import androidx.compose.runtime.produceState
import androidx.compose.runtime.getValue
import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.model.ArtifactKind
import com.jvp.aladin_compose.repo.ArtifactRepository
import com.jvp.aladin_compose.repo.LinkArtifactContent

interface LinkStateProducer {
    @Composable
    fun produce(openArtifacts: List<Artifact>, activeArtifactId: String?): LinkProducerState
}

data class LinkProducerState(
    val activeLink: LinkArtifactState?,
    val eventSink: (LinkArtifactEvent) -> Unit,
)

data class LinkArtifactState(
    val artifact: Artifact,
    val content: LinkArtifactContent?,
    val isLoading: Boolean,
    val errorMessage: String?,
)

sealed interface LinkArtifactEvent {
    data object RetryLoad : LinkArtifactEvent
}

class LinkStateProducerImpl(private val artifactRepository: ArtifactRepository) :
    LinkStateProducer {
    @Composable
    override fun produce(
        openArtifacts: List<Artifact>,
        activeArtifactId: String?,
    ): LinkProducerState {
        val activeLink =
            openArtifacts.firstOrNull { it.id == activeArtifactId && it.kind == ArtifactKind.Link }

        val content by
            produceState<LinkArtifactContent?>(null, activeLink?.id) {
                val artifact = activeLink ?: return@produceState
                artifactRepository.linkContent(artifact).collect { value = it }
            }

        return LinkProducerState(
            activeLink =
                activeLink?.let {
                    LinkArtifactState(
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
