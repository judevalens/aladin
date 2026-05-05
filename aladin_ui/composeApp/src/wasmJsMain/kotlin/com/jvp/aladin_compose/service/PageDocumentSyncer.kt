package com.jvp.aladin_compose.service

import com.jvp.aladin_compose.api.ApiException
import com.jvp.aladin_compose.repo.ArtifactRepository
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock

interface PageDocumentSyncer {
    val states: StateFlow<Map<String, PageDocumentSyncState>>

    suspend fun open(pageId: String)

    suspend fun close(pageId: String)

    suspend fun localSnapshotChanged(pageId: String, markdown: String)

    fun retryLoad(pageId: String)

    fun retrySave(pageId: String, markdown: String)
}

sealed interface PageDocumentSyncState {
    val pageId: String

    data class Loading(
        override val pageId: String,
        val loadError: String? = null,
    ) : PageDocumentSyncState

    data class Ready(
        override val pageId: String,
        val initialMarkdown: String,
        val lastSavedMarkdown: String,
        val revision: Long,
        val lastSavedAt: String?,
        val isSaving: Boolean,
        val loadError: String? = null,
        val saveError: String? = null,
    ) : PageDocumentSyncState
}

class PageDocumentSyncerImpl(
    private val artifactRepository: ArtifactRepository,
    private val scope: CoroutineScope,
) : PageDocumentSyncer {
    private val documentStates: MutableStateFlow<Map<String, PageDocumentSyncState>> =
        MutableStateFlow(emptyMap())
    private val pageLoadJobs = mutableMapOf<String, Job>()
    private val saveCounters = mutableMapOf<String, Long>()
    private val mutex = Mutex()

    override val states: StateFlow<Map<String, PageDocumentSyncState>> =
        documentStates.asStateFlow()

    override suspend fun open(pageId: String) {
        mutex.withLock {
            if (documentStates.value.containsKey(pageId)) return
            documentStates.update { it + (pageId to PageDocumentSyncState.Loading(pageId)) }
            startLoadLocked(pageId)
        }
    }

    override suspend fun close(pageId: String) {
        mutex.withLock {
            pageLoadJobs.remove(pageId)?.cancel()
            saveCounters.remove(pageId)
            documentStates.update { it - pageId }
        }
    }

    override suspend fun localSnapshotChanged(pageId: String, markdown: String) {
        val revision =
            mutex.withLock {
                val current = documentStates.value[pageId] as? PageDocumentSyncState.Ready
                    ?: return
                val nextRevision = maxOf(saveCounters[pageId] ?: current.revision, current.revision) + 1
                saveCounters[pageId] = nextRevision
                documentStates.update { states ->
                    states + (pageId to current.copy(isSaving = true, saveError = null))
                }
                nextRevision
            }

        scope.launch {
            try {
                val saved = artifactRepository.savePage(pageId, markdown, revision)
                mutex.withLock {
                    if (saveCounters[pageId] != revision) return@withLock
                    val current = documentStates.value[pageId] as? PageDocumentSyncState.Ready
                        ?: return@withLock
                    documentStates.update { states ->
                        states + (
                            pageId to current.copy(
                                initialMarkdown = saved.content,
                                lastSavedMarkdown = saved.content,
                                revision = saved.revision,
                                lastSavedAt = saved.updatedAt,
                                isSaving = false,
                                saveError = null,
                            )
                        )
                    }
                    saveCounters[pageId] = saved.revision
                }
            } catch (error: CancellationException) {
                throw error
            } catch (error: Throwable) {
                mutex.withLock {
                    if (saveCounters[pageId] != revision) return@withLock
                    val current = documentStates.value[pageId] as? PageDocumentSyncState.Ready
                        ?: return@withLock
                    documentStates.update { states ->
                        states + (
                            pageId to current.copy(
                                isSaving = false,
                                saveError =
                                    if (error.isRevisionConflict()) {
                                        "Page revision conflict"
                                    } else {
                                        error.message ?: "Failed to save page"
                                    },
                            )
                        )
                    }
                }
            }
        }
    }

    override fun retryLoad(pageId: String) {
        scope.launch {
            mutex.withLock {
                pageLoadJobs.remove(pageId)?.cancel()
                documentStates.update { it + (pageId to PageDocumentSyncState.Loading(pageId)) }
                startLoadLocked(pageId)
            }
        }
    }

    override fun retrySave(pageId: String, markdown: String) {
        scope.launch {
            localSnapshotChanged(pageId, markdown)
        }
    }

    private fun startLoadLocked(pageId: String) {
        pageLoadJobs[pageId]?.cancel()
        pageLoadJobs[pageId] =
            scope.launch {
                try {
                    val pageDocument = artifactRepository.page(pageId).first()
                    mutex.withLock {
                        saveCounters[pageId] = pageDocument.revision
                        documentStates.update { states ->
                            states + (
                                pageId to PageDocumentSyncState.Ready(
                                    pageId = pageId,
                                    initialMarkdown = pageDocument.content,
                                    lastSavedMarkdown = pageDocument.content,
                                    revision = pageDocument.revision,
                                    lastSavedAt = pageDocument.updatedAt,
                                    isSaving = false,
                                )
                            )
                        }
                    }
                } catch (error: CancellationException) {
                    throw error
                } catch (error: Throwable) {
                    mutex.withLock {
                        documentStates.update { states ->
                            states + (
                                pageId to PageDocumentSyncState.Loading(
                                    pageId = pageId,
                                    loadError = error.message ?: "Failed to load page",
                                )
                            )
                        }
                    }
                }
            }
    }
}

private fun Throwable.isRevisionConflict(): Boolean =
    this is ApiException && statusCode == 409
