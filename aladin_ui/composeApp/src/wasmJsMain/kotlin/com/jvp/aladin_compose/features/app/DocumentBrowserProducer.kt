package com.jvp.aladin_compose.features.app

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.model.FolderNode
import com.jvp.aladin_compose.service.FolderService
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch

private const val DrillInDepth = 2

interface DocumentBrowserProducer {
    @Composable fun produce(): DocumentBrowserProducerState
}

class DefaultDocumentBrowserProducer(
    private val service: FolderService,
    private val scope: CoroutineScope,
) : DocumentBrowserProducer {
    @Composable
    override fun produce(): DocumentBrowserProducerState {
        var selectedFolderId by remember { mutableStateOf<String?>(null) }
        var selectedArtifactId by remember { mutableStateOf<String?>(null) }
        var currentScopeId by remember { mutableStateOf<String?>(null) }
        var scopeBackStack by remember { mutableStateOf<List<String?>>(emptyList()) }
        var expandedFolderIds by remember { mutableStateOf(emptySet<String>()) }
        var refreshKey by remember { mutableStateOf(0) }
        var browserError by remember { mutableStateOf<String?>(null) }
        var hasLoadedRows by remember { mutableStateOf(false) }

        val selectedFolder =
            produceState<FolderNode?>(initialValue = null, service, selectedFolderId, refreshKey) {
                value =
                    try {
                        service.folder(selectedFolderId)
                    } catch (_: Throwable) {
                        null
                    }
            }.value
        val selectedArtifact =
            produceState<Artifact?>(initialValue = null, service, selectedArtifactId, refreshKey) {
                value =
                    try {
                        service.artifact(selectedArtifactId)
                    } catch (_: Throwable) {
                        null
                    }
            }.value
        val breadcrumbs =
            produceState(initialValue = emptyList(), service, selectedFolderId, selectedArtifactId, refreshKey) {
                value =
                    try {
                        if (selectedArtifactId != null) {
                            service.artifactBreadcrumbs(selectedArtifactId)
                        } else {
                            service.folderBreadcrumbs(selectedFolderId)
                        }
                    } catch (_: Throwable) {
                        emptyList()
                    }
            }.value
        val scopeBreadcrumbs =
            produceState(initialValue = emptyList(), service, currentScopeId, refreshKey) {
                value =
                    try {
                        service.folderBreadcrumbs(currentScopeId)
                    } catch (_: Throwable) {
                        emptyList()
                    }
            }.value
        val rows =
            produceState(
                initialValue = emptyList(),
                service,
                expandedFolderIds,
                selectedFolderId,
                selectedArtifactId,
                currentScopeId,
                refreshKey,
            ) {
                try {
                    browserError = null
                    service.prepareBrowser(
                        scopeId = currentScopeId,
                        expandedFolderIds = expandedFolderIds,
                        drillDepth = DrillInDepth,
                    )
                    value =
                        buildBrowserRows(
                            currentScopeId = currentScopeId,
                            expandedFolderIds = expandedFolderIds,
                            selectedFolderId = selectedFolderId,
                            selectedArtifactId = selectedArtifactId,
                        )
                    hasLoadedRows = true
                } catch (t: Throwable) {
                    browserError = t.message ?: "Failed to load folders"
                    hasLoadedRows = true
                    value = emptyList()
                }
            }.value

        return DocumentBrowserProducerState(
            browser =
                DocumentBrowserState(
                    breadcrumbs = breadcrumbs,
                    scopeBreadcrumbs = scopeBreadcrumbs,
                    canNavigateScopeBack = scopeBackStack.isNotEmpty(),
                    scopeBackTargetId = scopeBackStack.lastOrNull(),
                    loading = !hasLoadedRows,
                    errorMessage = browserError,
                    rows = rows,
                    eventSink = { event ->
                        when (event) {
                            is DocumentBrowserEvent.SelectFolder -> {
                                selectedFolderId = event.folderId
                                selectedArtifactId = null
                                scope.launch {
                                    expandedFolderIds = expandedFolderIds + service.ancestorFolderIds(event.folderId)
                                }
                            }
                            is DocumentBrowserEvent.SelectArtifact -> {
                                selectedArtifactId = event.artifactId
                                selectedFolderId = null
                                scope.launch {
                                    expandedFolderIds =
                                        expandedFolderIds + service.ancestorFolderIdsForArtifact(event.artifactId)
                                }
                            }
                            is DocumentBrowserEvent.ToggleFolderExpanded -> {
                                if (event.depth >= DrillInDepth) {
                                    scopeBackStack = scopeBackStack + currentScopeId
                                    currentScopeId = event.folderId
                                    selectedFolderId = event.folderId
                                    selectedArtifactId = null
                                    expandedFolderIds = expandedFolderIds + event.folderId
                                } else {
                                    expandedFolderIds =
                                        if (event.folderId in expandedFolderIds) {
                                            expandedFolderIds - event.folderId
                                        } else {
                                            expandedFolderIds + event.folderId
                                        }
                                }
                            }
                            is DocumentBrowserEvent.NavigateScope -> {
                                currentScopeId = event.folderId
                                scopeBackStack = scopeBackStack.dropLast(1)
                                selectedFolderId = event.folderId
                                selectedArtifactId = null
                            }
                            is DocumentBrowserEvent.NavigateBreadcrumb -> {
                                selectedFolderId = event.folderId
                                selectedArtifactId = null
                                currentScopeId = event.folderId
                                scopeBackStack = emptyList()
                                scope.launch {
                                    expandedFolderIds = expandedFolderIds + service.ancestorFolderIds(event.folderId)
                                }
                            }
                            DocumentBrowserEvent.CreateFolder -> {
                                scope.launch {
                                    try {
                                        val created = service.createFolder(currentScopeId)
                                        service.refreshBrowser(currentScopeId)
                                        browserError = null
                                        selectedFolderId = created.id
                                        selectedArtifactId = null
                                        expandedFolderIds =
                                            expandedFolderIds + service.ancestorFolderIds(created.id)
                                        refreshKey += 1
                                    } catch (t: Throwable) {
                                        browserError = t.message ?: "Failed to create folder"
                                    }
                                }
                            }
                            DocumentBrowserEvent.CreateArtifact -> {
                                scope.launch {
                                    try {
                                        val created = service.createArtifact(currentScopeId)
                                        service.refreshBrowser(currentScopeId)
                                        browserError = null
                                        selectedArtifactId = created.id
                                        selectedFolderId = null
                                        expandedFolderIds =
                                            expandedFolderIds + service.ancestorFolderIds(created.folderId)
                                        refreshKey += 1
                                    } catch (t: Throwable) {
                                        browserError = t.message ?: "Failed to create artifact"
                                    }
                                }
                            }
                            DocumentBrowserEvent.RetryLoad -> {
                                scope.launch {
                                    browserError = null
                                    hasLoadedRows = false
                                    service.refreshBrowser(currentScopeId)
                                    refreshKey += 1
                                }
                            }
                        }
                    },
                ),
            selectedFolder = selectedFolder,
            selectedArtifact = selectedArtifact,
            canCreateArtifact = true,
        )
    }

    private fun buildBrowserRows(
        currentScopeId: String?,
        expandedFolderIds: Set<String>,
        selectedFolderId: String?,
        selectedArtifactId: String?,
    ): List<BrowserTreeRow> {
        val rows = mutableListOf<BrowserTreeRow>()

        fun visit(parentId: String?, depth: Int, remainingDepth: Int) {
            val folders = service.cachedFolders(parentId).sortedBy { it.title.lowercase() }
            val artifacts = service.cachedArtifacts(parentId).sortedBy { it.title.lowercase() }

            folders.forEach { folder ->
                val expanded = folder.id in expandedFolderIds

                rows +=
                    BrowserTreeRow.Folder(
                        folder = folder,
                        depth = depth,
                        expanded = expanded,
                        expandable = true,
                        selected = folder.id == selectedFolderId,
                    )

                if (remainingDepth > 0 && expanded) {
                    visit(folder.id, depth + 1, remainingDepth - 1)
                }
            }

            artifacts.forEach { artifact ->
                rows +=
                    BrowserTreeRow.Artifact(
                        artifact = artifact,
                        depth = depth,
                        selected = artifact.id == selectedArtifactId,
                    )
            }
        }

        visit(parentId = currentScopeId, depth = 0, remainingDepth = DrillInDepth)
        return rows
    }
}
