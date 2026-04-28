package com.jvp.aladin_compose.features.app

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.model.ArtifactKind
import com.jvp.aladin_compose.model.BrowserNodeKind
import com.jvp.aladin_compose.model.BrowserTreeNode
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
        var focusedFolderId by remember { mutableStateOf<String?>(null) }
        var focusedArtifactId by remember { mutableStateOf<String?>(null) }
        var openArtifactIds by remember { mutableStateOf<List<String>>(emptyList()) }
        var activeArtifactId by remember { mutableStateOf<String?>(null) }
        var currentScopeId by remember { mutableStateOf<String?>(null) }
        var scopeBackStack by remember { mutableStateOf<List<String?>>(emptyList()) }
        var expandedFolderIds by remember { mutableStateOf(emptySet<String>()) }
        var refreshKey by remember { mutableStateOf(0) }
        var browserError by remember { mutableStateOf<String?>(null) }
        var hasLoadedRows by remember { mutableStateOf(false) }

        val focusedFolder =
            produceState<FolderNode?>(initialValue = null, service, focusedFolderId, refreshKey) {
                value =
                    try {
                        service.folder(focusedFolderId)
                    } catch (_: Throwable) {
                        null
                    }
            }.value
        val activeArtifact =
            produceState<Artifact?>(initialValue = null, service, activeArtifactId, refreshKey) {
                value =
                    try {
                        service.artifact(activeArtifactId)
                    } catch (_: Throwable) {
                        null
                    }
            }.value
        val openArtifacts =
            produceState(
                initialValue = emptyList(),
                service,
                openArtifactIds,
                refreshKey,
            ) {
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
            produceState(initialValue = emptyList(), service, activeArtifactId, refreshKey) {
                value =
                    try {
                        if (activeArtifactId != null) {
                            service.artifactBreadcrumbs(activeArtifactId)
                        } else {
                            emptyList()
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
                focusedFolderId,
                focusedArtifactId,
                currentScopeId,
                refreshKey,
            ) {
                try {
                    browserError = null
                    service.prepareBrowser()
                    value =
                        buildBrowserRows(
                            currentScopeId = currentScopeId,
                            expandedFolderIds = expandedFolderIds,
                            focusedFolderId = focusedFolderId,
                            focusedArtifactId = focusedArtifactId,
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
                            is DocumentBrowserEvent.FocusFolder -> {
                                focusedFolderId = event.folderId
                                focusedArtifactId = null
                                scope.launch {
                                    expandedFolderIds = expandedFolderIds + service.ancestorFolderIds(event.folderId)
                                }
                            }
                            is DocumentBrowserEvent.OpenArtifact -> {
                                focusedArtifactId = event.artifactId
                                focusedFolderId = null
                                if (event.artifactId !in openArtifactIds) {
                                    openArtifactIds = openArtifactIds + event.artifactId
                                }
                                activeArtifactId = event.artifactId
                                scope.launch {
                                    expandedFolderIds =
                                        expandedFolderIds + service.ancestorFolderIdsForArtifact(event.artifactId)
                                }
                            }
                            is DocumentBrowserEvent.ActivateArtifactTab -> {
                                if (event.artifactId in openArtifactIds) {
                                    activeArtifactId = event.artifactId
                                    focusedArtifactId = event.artifactId
                                    focusedFolderId = null
                                }
                            }
                            is DocumentBrowserEvent.ToggleFolderExpanded -> {
                                if (event.depth >= DrillInDepth) {
                                    scopeBackStack = scopeBackStack + currentScopeId
                                    currentScopeId = event.folderId
                                    focusedFolderId = event.folderId
                                    focusedArtifactId = null
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
                                focusedFolderId = event.folderId
                                focusedArtifactId = null
                            }
                            is DocumentBrowserEvent.NavigateBreadcrumb -> {
                                focusedFolderId = event.folderId
                                focusedArtifactId = null
                                currentScopeId = event.folderId
                                scopeBackStack = emptyList()
                                scope.launch {
                                    expandedFolderIds = expandedFolderIds + service.ancestorFolderIds(event.folderId)
                                }
                            }
                            is DocumentBrowserEvent.CreateInFolder -> {
                                scope.launch {
                                    when (event.option) {
                                        BrowserCreateOption.Folder -> {
                                            try {
                                                val created = service.createFolder(event.folderId)
                                                service.refreshBrowser()
                                                browserError = null
                                                focusedFolderId = created.id
                                                focusedArtifactId = null
                                                expandedFolderIds =
                                                    expandedFolderIds + service.ancestorFolderIds(created.id)
                                                refreshKey += 1
                                            } catch (t: Throwable) {
                                                browserError = t.message ?: "Failed to create folder"
                                            }
                                        }
                                        BrowserCreateOption.Note,
                                        BrowserCreateOption.Link -> {
                                            try {
                                                val kind =
                                                    when (event.option) {
                                                        BrowserCreateOption.Link -> ArtifactKind.Link
                                                        else -> ArtifactKind.Note
                                                    }
                                                val created = service.createArtifact(event.folderId, kind)
                                                service.refreshBrowser()
                                                browserError = null
                                                focusedArtifactId = created.id
                                                focusedFolderId = null
                                                if (created.id !in openArtifactIds) {
                                                    openArtifactIds = openArtifactIds + created.id
                                                }
                                                activeArtifactId = created.id
                                                expandedFolderIds =
                                                    expandedFolderIds + service.ancestorFolderIds(created.folderId)
                                                refreshKey += 1
                                            } catch (t: Throwable) {
                                                browserError = t.message ?: "Failed to create artifact"
                                            }
                                        }
                                        BrowserCreateOption.Voice,
                                        BrowserCreateOption.Upload -> Unit
                                    }
                                }
                            }
                            DocumentBrowserEvent.CreateFolder -> {
                                scope.launch {
                                    try {
                                        val targetFolderId = focusedFolderId ?: currentScopeId
                                        val created = service.createFolder(targetFolderId)
                                        service.refreshBrowser()
                                        browserError = null
                                        focusedFolderId = created.id
                                        focusedArtifactId = null
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
                                        val targetFolderId = focusedFolderId ?: currentScopeId
                                        val created = service.createArtifact(targetFolderId)
                                        service.refreshBrowser()
                                        browserError = null
                                        focusedArtifactId = created.id
                                        focusedFolderId = null
                                        if (created.id !in openArtifactIds) {
                                            openArtifactIds = openArtifactIds + created.id
                                        }
                                        activeArtifactId = created.id
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
                                    service.refreshBrowser()
                                    refreshKey += 1
                                }
                            }
                        }
                    },
                ),
            focusedFolder = focusedFolder,
            activeArtifact = activeArtifact,
            openArtifacts = openArtifacts,
            canCreateArtifact = true,
        )
    }

    private fun buildBrowserRows(
        currentScopeId: String?,
        expandedFolderIds: Set<String>,
        focusedFolderId: String?,
        focusedArtifactId: String?,
    ): List<BrowserTreeRow> {
        val rows = mutableListOf<BrowserTreeRow>()

        fun visit(parentId: String?, depth: Int, remainingDepth: Int) {
            val nodes = service.treeChildren(parentId)
            nodes.forEach { node ->
                when (node.kind) {
                    BrowserNodeKind.Folder -> appendFolderNode(rows, node, depth, remainingDepth, expandedFolderIds, focusedFolderId, ::visit)
                    BrowserNodeKind.Artifact -> {
                        val preview = node.artifactPreview ?: return@forEach
                        rows +=
                            BrowserTreeRow.Artifact(
                                artifact = preview,
                                depth = depth,
                                selected = preview.id == focusedArtifactId,
                                menu = artifactMenu(preview.id),
                            )
                    }
                }
            }
        }

        visit(parentId = currentScopeId, depth = 0, remainingDepth = DrillInDepth)
        return rows
    }

    private fun appendFolderNode(
        rows: MutableList<BrowserTreeRow>,
        node: BrowserTreeNode,
        depth: Int,
        remainingDepth: Int,
        expandedFolderIds: Set<String>,
        focusedFolderId: String?,
        visit: (String?, Int, Int) -> Unit,
    ) {
        val folder = FolderNode(id = node.id, parentId = node.parentId, title = node.title)
        val expanded = node.id in expandedFolderIds
        val expandable = node.children.isNotEmpty()

        rows +=
            BrowserTreeRow.Folder(
                folder = folder,
                depth = depth,
                expanded = expanded,
                expandable = expandable,
                selected = node.id == focusedFolderId,
                menu = folderMenu(node.id),
            )

        if (remainingDepth > 0 && expanded) {
            visit(node.id, depth + 1, remainingDepth - 1)
        }
    }

    private fun folderMenu(folderId: String): BrowserRowMenuModel {
        return BrowserRowMenuModel(
            rowId = folderId,
            rowKind = BrowserRowKind.Folder,
            sections =
                listOf(
                    BrowserRowMenuSection(
                        title = "Create",
                        actions =
                            listOf(
                                BrowserRowMenuAction(id = createActionId(BrowserCreateOption.Folder), label = "New folder"),
                                BrowserRowMenuAction(id = createActionId(BrowserCreateOption.Note), label = "New note"),
                                BrowserRowMenuAction(id = createActionId(BrowserCreateOption.Link), label = "New link"),
                                BrowserRowMenuAction(
                                    id = createActionId(BrowserCreateOption.Voice),
                                    label = "New voice",
                                    enabled = false,
                                ),
                                BrowserRowMenuAction(
                                    id = createActionId(BrowserCreateOption.Upload),
                                    label = "New upload",
                                    enabled = false,
                                ),
                            ),
                    )
                ),
        )
    }

    private fun artifactMenu(artifactId: String): BrowserRowMenuModel {
        return BrowserRowMenuModel(
            rowId = artifactId,
            rowKind = BrowserRowKind.Artifact,
            sections = emptyList(),
        )
    }

    private fun createActionId(option: BrowserCreateOption): String = "create:${option.name.lowercase()}"
}
