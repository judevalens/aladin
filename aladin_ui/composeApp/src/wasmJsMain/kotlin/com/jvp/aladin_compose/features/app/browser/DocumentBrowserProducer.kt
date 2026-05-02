package com.jvp.aladin_compose.features.app.browser

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import com.jvp.aladin_compose.features.app.BrowserRowKind
import com.jvp.aladin_compose.features.app.BrowserRowMenuAction
import com.jvp.aladin_compose.features.app.BrowserRowMenuActionTone
import com.jvp.aladin_compose.features.app.BrowserRowMenuModel
import com.jvp.aladin_compose.features.app.BrowserRowMenuSection
import com.jvp.aladin_compose.model.BreadcrumbItem
import com.jvp.aladin_compose.model.ArtifactKind
import com.jvp.aladin_compose.model.BrowserNodeKind
import com.jvp.aladin_compose.model.BrowserTreeNode
import com.jvp.aladin_compose.model.FolderNode
import com.jvp.aladin_compose.service.ArtifactService
import com.jvp.aladin_compose.service.FolderService
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch

private const val DrillInDepth = 2

interface DocumentBrowserProducer {
    @Composable fun produce(activeArtifactId: String?, onOpenArtifact: (String) -> Unit): DocumentBrowserState
}

data class DocumentBrowserState(
    val breadcrumbs: List<BreadcrumbItem>,
    val scopeBreadcrumbs: List<BreadcrumbItem>,
    val canNavigateScopeBack: Boolean,
    val scopeBackTargetId: String?,
    val loading: Boolean,
    val errorMessage: String?,
    val rows: List<BrowserTreeRow>,
    val activeRename: BrowserRenameState?,
    val eventSink: (DocumentBrowserEvent) -> Unit,
)

data class BrowserRenameState(
    val rowKind: BrowserRowKind,
    val rowId: String,
    val originalTitle: String,
    val draftTitle: String,
    val saving: Boolean = false,
)

sealed interface DocumentBrowserEvent {
    data class FocusFolder(val folderId: String) : DocumentBrowserEvent
    data class OpenArtifact(val artifactId: String) : DocumentBrowserEvent
    data class ToggleFolderExpanded(val folderId: String, val depth: Int) : DocumentBrowserEvent
    data class NavigateScope(val folderId: String?) : DocumentBrowserEvent
    data class NavigateBreadcrumb(val folderId: String?) : DocumentBrowserEvent
    data class CreateInFolder(val folderId: String, val option: BrowserCreateOption) : DocumentBrowserEvent
    data class StartRename(val rowKind: BrowserRowKind, val rowId: String, val currentTitle: String) : DocumentBrowserEvent
    data class RenameDraftChanged(val rowId: String, val title: String) : DocumentBrowserEvent
    data class CommitRename(val rowId: String) : DocumentBrowserEvent
    data class CancelRename(val rowId: String) : DocumentBrowserEvent
    data object CreateFolder : DocumentBrowserEvent
    data object CreateArtifact : DocumentBrowserEvent
    data object RetryLoad : DocumentBrowserEvent
}

class DefaultDocumentBrowserProducer(
    private val service: FolderService,
    private val artifactService: ArtifactService,
    private val scope: CoroutineScope,
) : DocumentBrowserProducer {
    @Composable
    override fun produce(
        activeArtifactId: String?,
        onOpenArtifact: (String) -> Unit,
    ): DocumentBrowserState {
        var focusedFolderId by remember { mutableStateOf<String?>(null) }
        var currentScopeId by remember { mutableStateOf<String?>(null) }
        var scopeBackStack by remember { mutableStateOf<List<String?>>(emptyList()) }
        var expandedFolderIds by remember { mutableStateOf(emptySet<String>()) }
        var refreshKey by remember { mutableStateOf(0) }
        var browserError by remember { mutableStateOf<String?>(null) }
        var hasLoadedRows by remember { mutableStateOf(false) }
        var activeRename by remember { mutableStateOf<BrowserRenameState?>(null) }

        val focusedFolder =
            produceState<FolderNode?>(initialValue = null, service, focusedFolderId, refreshKey) {
                value =
                    try {
                        service.folder(focusedFolderId)
                    } catch (_: Throwable) {
                        null
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
                activeArtifactId,
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
                            focusedArtifactId = activeArtifactId,
                        )
                    hasLoadedRows = true
                } catch (t: Throwable) {
                    browserError = t.message ?: "Failed to load folders"
                    hasLoadedRows = true
                    value = emptyList()
                }
            }.value

        return DocumentBrowserState(
            breadcrumbs = breadcrumbs,
            scopeBreadcrumbs = scopeBreadcrumbs,
            canNavigateScopeBack = scopeBackStack.isNotEmpty(),
            scopeBackTargetId = scopeBackStack.lastOrNull(),
            loading = !hasLoadedRows,
            errorMessage = browserError,
            rows = rows,
            activeRename = activeRename,
            eventSink = { event ->
                when (event) {
                    is DocumentBrowserEvent.FocusFolder -> {
                        focusedFolderId = event.folderId
                        scope.launch {
                            expandedFolderIds = expandedFolderIds + service.ancestorFolderIds(event.folderId)
                        }
                    }
                    is DocumentBrowserEvent.OpenArtifact -> {
                        focusedFolderId = null
                        onOpenArtifact(event.artifactId)
                        scope.launch {
                            expandedFolderIds =
                                expandedFolderIds + service.ancestorFolderIdsForArtifact(event.artifactId)
                        }
                    }
                    is DocumentBrowserEvent.ToggleFolderExpanded -> {
                        if (event.depth >= DrillInDepth) {
                            scopeBackStack = scopeBackStack + currentScopeId
                            currentScopeId = event.folderId
                            focusedFolderId = event.folderId
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
                    }
                    is DocumentBrowserEvent.NavigateBreadcrumb -> {
                        focusedFolderId = event.folderId
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
                                        focusedFolderId = null
                                        onOpenArtifact(created.id)
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
                    is DocumentBrowserEvent.StartRename -> {
                        activeRename =
                            BrowserRenameState(
                                rowKind = event.rowKind,
                                rowId = event.rowId,
                                originalTitle = event.currentTitle,
                                draftTitle = event.currentTitle,
                            )
                        browserError = null
                    }
                    is DocumentBrowserEvent.RenameDraftChanged -> {
                        activeRename =
                            activeRename?.takeIf { it.rowId == event.rowId && !it.saving }?.copy(draftTitle = event.title)
                                ?: activeRename
                    }
                    is DocumentBrowserEvent.CommitRename -> {
                        val rename = activeRename?.takeIf { it.rowId == event.rowId } ?: return@DocumentBrowserState
                        commitRename(
                            rename = rename,
                            setActiveRename = { activeRename = it },
                            setBrowserError = { browserError = it },
                            setFocusedFolderId = { focusedFolderId = it },
                            refresh = { refreshKey += 1 },
                        )
                    }
                    is DocumentBrowserEvent.CancelRename -> {
                        activeRename = activeRename?.takeUnless { it.rowId == event.rowId }
                    }
                    DocumentBrowserEvent.CreateFolder -> {
                        scope.launch {
                            try {
                                val targetFolderId = focusedFolderId ?: currentScopeId
                                val created = service.createFolder(targetFolderId)
                                service.refreshBrowser()
                                browserError = null
                                focusedFolderId = created.id
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
                                focusedFolderId = null
                                onOpenArtifact(created.id)
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
                        title = "Manage",
                        actions =
                            listOf(
                                BrowserRowMenuAction(id = RenameActionId, label = "Rename"),
                            ),
                    ),
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
            sections =
                listOf(
                    BrowserRowMenuSection(
                        title = "Manage",
                        actions =
                            listOf(
                                BrowserRowMenuAction(id = RenameActionId, label = "Rename"),
                            ),
                    ),
                ),
        )
    }

    private fun createActionId(option: BrowserCreateOption): String = "create:${option.name.lowercase()}"

    private fun commitRename(
        rename: BrowserRenameState,
        setActiveRename: (BrowserRenameState?) -> Unit,
        setBrowserError: (String?) -> Unit,
        setFocusedFolderId: (String?) -> Unit,
        refresh: () -> Unit,
    ) {
        if (rename.saving) return
        val nextTitle = rename.draftTitle.trim()
        if (nextTitle.isEmpty() || nextTitle == rename.originalTitle) {
            setActiveRename(null)
            return
        }
        setActiveRename(rename.copy(draftTitle = nextTitle, saving = true))
        scope.launch {
            try {
                when (rename.rowKind) {
                    BrowserRowKind.Folder -> {
                        val renamed = service.renameFolder(rename.rowId, nextTitle)
                        setFocusedFolderId(renamed.id)
                    }
                    BrowserRowKind.Artifact -> {
                        artifactService.renameArtifact(rename.rowId, nextTitle)
                        service.refreshBrowser()
                    }
                }
                setBrowserError(null)
                setActiveRename(null)
                refresh()
            } catch (t: Throwable) {
                setBrowserError(t.message ?: "Failed to rename")
                setActiveRename(rename.copy(draftTitle = nextTitle, saving = false))
            }
        }
    }

    private companion object {
        const val RenameActionId = "rename"
    }
}
