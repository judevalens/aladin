package com.jvp.aladin_compose.features.app.artifactpane.page

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.model.ArtifactKind
import com.jvp.aladin_compose.repo.ArtifactRepository
import com.jvp.aladin_compose.service.PageDocumentSyncState
import com.jvp.aladin_compose.service.PageDocumentSyncer
import com.jvp.aladin_compose.service.web.PageEditorBridge
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch
import kotlin.time.Clock
import kotlin.time.Instant

interface PageStateProducer {
    @Composable
    fun produce(openArtifacts: List<Artifact>, activeArtifactId: String?): PageProducerState
}

data class PageProducerState(
    val activePage: PageEditorState?,
    val pagesById: Map<String, PageEditorState>,
    val eventSink: (PageStateEvent) -> Unit,
)

data class PageEditorState(
    val pageId: String,
    val initialMarkdown: String,
    val draftMarkdown: String,
    val isEditable: Boolean,
    val isDirty: Boolean,
    val showUnsavedStatus: Boolean,
    val isLoaded: Boolean,
    val loadError: String?,
    val isSaving: Boolean,
    val saveError: String?,
    val isUploading: Boolean,
    val uploadError: String?,
    val lastSavedAt: String?,
    val timestampTick: Instant,
)

sealed interface PageStateEvent {
    data class DocumentUpdated(val pageId: String, val markdown: String) : PageStateEvent

    data class RetrySave(val pageId: String) : PageStateEvent

    data class RetryLoad(val pageId: String) : PageStateEvent

    data class ToggleEditable(val pageId: String) : PageStateEvent

    data class FileUploadRequested(
        val pageId: String,
        val requestId: String,
        val filename: String,
        val contentType: String?,
        val base64Data: String,
    ) : PageStateEvent

    data class RetryUpload(val pageId: String) : PageStateEvent
}

class PageStateProducerImpl(
    private val artifactRepository: ArtifactRepository,
    private val pageDocumentSyncer: PageDocumentSyncer,
    private val scope: CoroutineScope,
    private val pageEditorBridge: PageEditorBridge,
) : PageStateProducer {
    @Composable
    override fun produce(
        openArtifacts: List<Artifact>,
        activeArtifactId: String?,
    ): PageProducerState {
        val openPages = openArtifacts.filter { it.kind == ArtifactKind.Note }
        val openPageIds = openPages.map { it.id }.toSet()
        val syncStates by pageDocumentSyncer.states.collectAsState()
        var knownOpenPageIds by remember { mutableStateOf(emptySet<String>()) }

        LaunchedEffect(openPageIds) {
            (openPageIds - knownOpenPageIds).forEach { pageDocumentSyncer.open(it) }
            (knownOpenPageIds - openPageIds).forEach { pageDocumentSyncer.close(it) }
            knownOpenPageIds = openPageIds
        }

        val pagesById =
            openPages.associate { artifact ->
                val syncState =
                    syncStates[artifact.id] ?: PageDocumentSyncState.Loading(artifact.id)
                artifact.id to syncState.toEditorState()
            }

        return PageProducerState(
            activePage = activeArtifactId?.let(pagesById::get),
            pagesById = pagesById,
            eventSink = { event ->
                when (event) {
                    is PageStateEvent.DocumentUpdated -> {
                        if (event.pageId in openPageIds) {
                            scope.launch {
                                pageDocumentSyncer.localSnapshotChanged(
                                    event.pageId,
                                    event.markdown,
                                )
                            }
                        }
                    }
                    is PageStateEvent.FileUploadRequested -> TODO()
                    is PageStateEvent.RetryLoad -> pageDocumentSyncer.retryLoad(event.pageId)
                    is PageStateEvent.RetrySave -> {
                        val markdown = pagesById[event.pageId]?.draftMarkdown
                            ?: return@PageProducerState
                        pageDocumentSyncer.retrySave(event.pageId, markdown)
                    }
                    is PageStateEvent.RetryUpload -> TODO()
                    is PageStateEvent.ToggleEditable -> TODO()
                }
            },
        )
    }
}

private fun PageDocumentSyncState.toEditorState(): PageEditorState =
    when (this) {
        is PageDocumentSyncState.Loading ->
            PageEditorState(
                pageId = pageId,
                initialMarkdown = "",
                draftMarkdown = "",
                isEditable = true,
                isDirty = false,
                showUnsavedStatus = false,
                isLoaded = false,
                loadError = loadError,
                isSaving = false,
                saveError = null,
                isUploading = false,
                uploadError = null,
                lastSavedAt = null,
                timestampTick = Clock.System.now(),
            )
        is PageDocumentSyncState.Ready ->
            PageEditorState(
                pageId = pageId,
                initialMarkdown = initialMarkdown,
                draftMarkdown = lastSavedMarkdown,
                isEditable = true,
                isDirty = false,
                showUnsavedStatus = false,
                isLoaded = true,
                loadError = loadError,
                isSaving = isSaving,
                saveError = saveError,
                isUploading = false,
                uploadError = null,
                lastSavedAt = lastSavedAt,
                timestampTick = Clock.System.now(),
            )
    }

private val PageStateEvent.targetPageId: String
    get() =
        when (this) {
            is PageStateEvent.DocumentUpdated -> this.pageId
            is PageStateEvent.RetrySave -> this.pageId
            is PageStateEvent.RetryLoad -> this.pageId
            is PageStateEvent.ToggleEditable -> this.pageId
            is PageStateEvent.FileUploadRequested -> this.pageId
            is PageStateEvent.RetryUpload -> this.pageId
        }
