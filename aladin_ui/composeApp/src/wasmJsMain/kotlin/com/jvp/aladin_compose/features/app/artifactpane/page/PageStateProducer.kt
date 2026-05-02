package com.jvp.aladin_compose.features.app.artifactpane.page

import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.model.ArtifactKind
import com.jvp.aladin_compose.service.ArtifactService
import com.jvp.aladin_compose.service.PageDocumentSyncFailure
import com.jvp.aladin_compose.service.PageDocumentSyncStatus
import com.jvp.aladin_compose.service.PageDocumentSyncUpdate
import com.jvp.aladin_compose.service.PageDocumentSyncer
import com.jvp.aladin_compose.service.web.PageEditorBridge
import kotlinx.browser.window
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlin.time.Clock
import kotlin.time.Duration.Companion.seconds
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

class DefaultPageStateProducer(
    private val artifactService: ArtifactService,
    private val pageDocumentSyncer: PageDocumentSyncer,
    private val scope: CoroutineScope,
    private val pageEditorBridge: PageEditorBridge,
) : PageStateProducer {
    @Composable
    override fun produce(
        openArtifacts: List<Artifact>,
        activeArtifactId: String?,
    ): PageProducerState {
        val pageRuntimeById = remember { mutableStateMapOf<String, PageRuntime>() }
        val latestSyncByPageId = remember { mutableStateMapOf<String, PageDocumentSyncUpdate>() }
        var timestampTick by remember { mutableStateOf(Clock.System.now()) }
        val openPages = openArtifacts.filter { it.kind == ArtifactKind.Note }

        LaunchedEffect(Unit) {
            while (true) {
                delay(15.seconds)
                if (pageRuntimeById.isNotEmpty()) {
                    timestampTick = Clock.System.now()
                }
            }
        }

        LaunchedEffect(pageDocumentSyncer) {
            pageDocumentSyncer.updates.collect { update ->
                if (update.lastSavedAt != null) {
                    timestampTick = Clock.System.now()
                }
                latestSyncByPageId[update.pageId] = update
                updatePageRuntime(pageRuntimeById, update.pageId) { runtime ->
                    runtime.applySyncUpdate(update)
                }
            }
        }

        openPages.forEach { artifact ->
            if (artifact.id !in pageRuntimeById) {
                pageRuntimeById[artifact.id] =
                    PageRuntime(
                        pageId = artifact.id,
                        initialMarkdown = "",
                        draftMarkdown = "",
                        lastSavedMarkdown = "",
                        lastSavedAt = null,
                    )
            }

            DisposableEffect(artifact.id, pageDocumentSyncer) {
                pageDocumentSyncer.open(artifact.id)
                onDispose {
                    pageDocumentSyncer.close(artifact.id)
                    latestSyncByPageId.remove(artifact.id)
                }
            }

        }

        val openArtifactIds = openPages.map { it.id }.toSet()
        pageRuntimeById.keys.filterNot(openArtifactIds::contains).forEach { pageId ->
            pageRuntimeById.remove(pageId)
            latestSyncByPageId.remove(pageId)
        }

        val pagesById = openPages.associate { artifact ->
            artifact.id to
                (pageRuntimeById[artifact.id]?.toEditorState(timestampTick)
                    ?: PageRuntime.fromArtifact(artifact).toEditorState(timestampTick))
        }

        return PageProducerState(
            activePage = activeArtifactId?.let(pagesById::get),
            pagesById = pagesById,
            eventSink = { event ->
                when (event) {
                    is PageStateEvent.DocumentUpdated -> {
                        if (event.pageId in openArtifactIds) {
                            pageDocumentSyncer.localSnapshotChanged(event.pageId, event.markdown)
                            updatePageRuntime(pageRuntimeById, event.pageId) {
                                it.copy(
                                    draftMarkdown = event.markdown,
                                    saveError = null,
                                )
                            }
                        }
                    }
                    is PageStateEvent.RetrySave -> {
                        if (event.pageId in openArtifactIds) {
                            val markdown =
                                pageRuntimeById[event.pageId]?.draftMarkdown
                                    ?: return@PageProducerState
                            pageDocumentSyncer.retrySave(event.pageId, markdown)
                        }
                    }
                    is PageStateEvent.RetryLoad -> {
                        if (event.pageId in openArtifactIds) {
                            pageDocumentSyncer.retryLoad(event.pageId)
                        }
                    }
                    is PageStateEvent.ToggleEditable -> {
                        if (event.pageId in openArtifactIds) {
                            updatePageRuntime(pageRuntimeById, event.pageId) {
                                it.copy(isEditable = !it.isEditable)
                            }
                        }
                    }
                    is PageStateEvent.FileUploadRequested -> {
                        if (event.pageId in openArtifactIds) {
                            updatePageRuntime(pageRuntimeById, event.pageId) {
                                it.copy(uploadError = null, failedUpload = null)
                            }
                            requestUpload(
                                upload =
                                    PendingPageUpload(
                                        pageId = event.pageId,
                                        requestId = event.requestId,
                                        filename = event.filename,
                                        contentType = event.contentType,
                                        base64Data = event.base64Data,
                                    ),
                                artifactService = artifactService,
                                scope = scope,
                                pageEditorBridge = pageEditorBridge,
                                pageRuntimeById = pageRuntimeById,
                            )
                        }
                    }
                    is PageStateEvent.RetryUpload -> {
                        if (event.pageId in openArtifactIds) {
                            val upload =
                                pageRuntimeById[event.pageId]?.failedUpload
                                    ?: return@PageProducerState
                            updatePageRuntime(pageRuntimeById, event.pageId) {
                                it.copy(uploadError = null)
                            }
                            requestUpload(
                                upload = upload,
                                artifactService = artifactService,
                                scope = scope,
                                pageEditorBridge = pageEditorBridge,
                                pageRuntimeById = pageRuntimeById,
                            )
                        }
                    }
                }
            },
        )
    }

    private fun requestUpload(
        upload: PendingPageUpload,
        artifactService: ArtifactService,
        scope: CoroutineScope,
        pageEditorBridge: PageEditorBridge,
        pageRuntimeById: MutableMap<String, PageRuntime>,
    ) {
        val page = pageRuntimeById[upload.pageId] ?: return
        if (upload.requestId in page.uploadingRequestIds) return
        updatePageRuntime(pageRuntimeById, upload.pageId) {
            it.copy(uploadingRequestIds = it.uploadingRequestIds + upload.requestId)
        }
        scope.launch {
            try {
                val uploaded =
                    artifactService.uploadFile(
                        filename = upload.filename,
                        bytes = decodeBase64(upload.base64Data),
                        contentType = upload.contentType,
                    )
                updatePageRuntime(pageRuntimeById, upload.pageId) {
                    it.copy(failedUpload = null, uploadError = null)
                }
                pageEditorBridge.notifyFileUploadCompleted(
                    upload.pageId,
                    upload.requestId,
                    uploaded.url,
                )
            } catch (error: Throwable) {
                updatePageRuntime(pageRuntimeById, upload.pageId) {
                    it.copy(failedUpload = upload, uploadError = error.message ?: "Upload failed")
                }
            } finally {
                updatePageRuntime(pageRuntimeById, upload.pageId) {
                    it.copy(uploadingRequestIds = it.uploadingRequestIds - upload.requestId)
                }
            }
        }
    }

}

private data class PageRuntime(
    val pageId: String,
    val initialMarkdown: String,
    val draftMarkdown: String,
    val isEditable: Boolean = true,
    val lastSavedMarkdown: String,
    val lastSavedAt: String?,
    val isLoaded: Boolean = false,
    val loadError: String? = null,
    val isSaving: Boolean = false,
    val saveError: String? = null,
    val showUnsavedStatus: Boolean = false,
    val uploadError: String? = null,
    val uploadingRequestIds: Set<String> = emptySet(),
    val failedUpload: PendingPageUpload? = null,
) {
    fun applySyncUpdate(update: PageDocumentSyncUpdate): PageRuntime {
        val snapshot = update.initialSnapshot
        val loadFailure = update.failure as? PageDocumentSyncFailure.LoadFailed
        val saveFailure = update.failure as? PageDocumentSyncFailure.SaveFailed
        val synced = update.status == PageDocumentSyncStatus.Synced
        val canApplyInitialSnapshot = snapshot != null && !isLoaded
        val nextDraft =
            when {
                canApplyInitialSnapshot -> snapshot.markdown
                else -> draftMarkdown
            }
        return copy(
            initialMarkdown = if (canApplyInitialSnapshot) snapshot.markdown else initialMarkdown,
            draftMarkdown = nextDraft,
            lastSavedMarkdown =
                when {
                    canApplyInitialSnapshot -> snapshot.markdown
                    synced -> nextDraft
                    else -> lastSavedMarkdown
                },
            lastSavedAt = update.lastSavedAt ?: lastSavedAt,
            isLoaded = isLoaded || canApplyInitialSnapshot,
            loadError = loadFailure?.message,
            isSaving = false,
            saveError = saveFailure?.message,
            showUnsavedStatus = false,
        )
    }

    fun toEditorState(timestampTick: Instant): PageEditorState =
        PageEditorState(
            pageId = pageId,
            initialMarkdown = initialMarkdown,
            draftMarkdown = draftMarkdown,
            isEditable = isEditable,
            isDirty = draftMarkdown != lastSavedMarkdown,
            showUnsavedStatus = showUnsavedStatus,
            isLoaded = isLoaded,
            loadError = loadError,
            isSaving = isSaving,
            saveError = saveError,
            isUploading = uploadingRequestIds.isNotEmpty(),
            uploadError = uploadError,
            lastSavedAt = lastSavedAt,
            timestampTick = timestampTick,
        )

    companion object {
        fun fromArtifact(artifact: Artifact): PageRuntime =
            PageRuntime(
                pageId = artifact.id,
                initialMarkdown = artifact.content,
                draftMarkdown = artifact.content,
                lastSavedMarkdown = artifact.content,
                lastSavedAt = artifact.updatedLabel,
            )
    }
}

private fun updatePageRuntime(
    pageRuntimeById: MutableMap<String, PageRuntime>,
    pageId: String,
    update: (PageRuntime) -> PageRuntime,
): Boolean {
    val current = pageRuntimeById[pageId] ?: return false
    pageRuntimeById[pageId] = update(current)
    return true
}

private data class PendingPageUpload(
    val pageId: String,
    val requestId: String,
    val filename: String,
    val contentType: String?,
    val base64Data: String,
)

private fun decodeBase64(encoded: String): ByteArray {
    val binary = window.atob(encoded)
    return ByteArray(binary.length) { index -> binary[index].code.toByte() }
}
