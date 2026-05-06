package com.jvp.aladin_compose.features.app.browser

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import com.jvp.aladin_compose.features.app.BrowserRowKind
import com.jvp.aladin_compose.features.app.BrowserRowMenuAction
import com.jvp.aladin_compose.features.app.BrowserRowMenuModel
import com.jvp.aladin_compose.features.app.BrowserRowMenuSection
import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.model.ArtifactKind
import com.jvp.aladin_compose.model.ArtifactPreview
import com.jvp.aladin_compose.model.BreadcrumbItem
import com.jvp.aladin_compose.model.BrowserNodeKind
import com.jvp.aladin_compose.model.BrowserTreeNode
import com.jvp.aladin_compose.model.FolderNode
import com.jvp.aladin_compose.repo.ArtifactRepository
import com.jvp.aladin_compose.repo.FolderRepository
import com.jvp.aladin_compose.service.VoicePlaybackHandle
import com.jvp.aladin_compose.service.VoiceCaptureResult
import com.jvp.aladin_compose.service.VoiceCaptureService
import com.jvp.aladin_compose.service.VoiceCaptureSession
import com.jvp.aladin_compose.service.createVoicePlaybackHandle
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlin.random.Random

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
    val activeVoiceCapture: BrowserVoiceCaptureState?,
    val eventSink: (DocumentBrowserEvent) -> Unit,
)

data class BrowserRenameState(
    val rowKind: BrowserRowKind,
    val rowId: String,
    val originalTitle: String,
    val draftTitle: String,
    val saving: Boolean = false,
)

enum class BrowserVoiceCapturePhase {
    Ready,
    Starting,
    Recording,
    Review,
    Saving,
    Failed,
}

data class BrowserVoiceCaptureState(
    val title: String,
    val description: String,
    val phase: BrowserVoiceCapturePhase,
    val elapsedMs: Long,
    val levels: List<Float>,
    val playbackUrl: String?,
    val isPlaying: Boolean,
    val playbackPositionMs: Long,
    val playbackDurationMs: Long,
    val errorMessage: String?,
)

sealed interface DocumentBrowserEvent {
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
    data object CreateVoiceArtifact : DocumentBrowserEvent
    data class StartVoiceCapture(val folderId: String?) : DocumentBrowserEvent
    data object BeginVoiceRecording : DocumentBrowserEvent
    data object StopVoiceRecording : DocumentBrowserEvent
    data object ToggleVoicePlayback : DocumentBrowserEvent
    data class VoicePlaybackPositionChanged(val positionMs: Long) : DocumentBrowserEvent
    data class SeekVoicePlayback(val positionMs: Long) : DocumentBrowserEvent
    data object VoicePlaybackEnded : DocumentBrowserEvent
    data class VoiceCaptureTitleChanged(val title: String) : DocumentBrowserEvent
    data class VoiceCaptureDescriptionChanged(val description: String) : DocumentBrowserEvent
    data object SaveVoiceCapture : DocumentBrowserEvent
    data object CancelVoiceCapture : DocumentBrowserEvent
    data object RetryLoad : DocumentBrowserEvent
}

class DocumentBrowserProducerImpl(
    private val folderRepository: FolderRepository,
    private val artifactRepository: ArtifactRepository,
    private val voiceCaptureService: VoiceCaptureService,
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
        var browserError by remember { mutableStateOf<String?>(null) }
        var hasLoadedRows by remember { mutableStateOf(false) }
        var activeRename by remember { mutableStateOf<BrowserRenameState?>(null) }
        var activeVoiceCapture by remember { mutableStateOf<BrowserVoiceCaptureState?>(null) }
        var voiceCaptureTargetFolderId by remember { mutableStateOf<String?>(null) }
        var voiceCaptureSession by remember { mutableStateOf<VoiceCaptureSession?>(null) }
        var voiceCaptureResult by remember { mutableStateOf<VoiceCaptureResult?>(null) }
        var voicePlaybackHandle by remember { mutableStateOf<VoicePlaybackHandle?>(null) }
        var voiceCaptureTimerJob by remember { mutableStateOf<Job?>(null) }
        var tree by remember { mutableStateOf(emptyList<BrowserTreeNode>()) }
        val breadcrumbs by artifactRepository.observeArtifactBreadcrumbs(activeArtifactId).collectAsState(initial = emptyList())
        val scopeBreadcrumbs = buildFolderBreadcrumbs(tree, currentScopeId)

        LaunchedEffect(folderRepository) {
            try {
                browserError = null
                tree = folderRepository.browserTree()
                hasLoadedRows = true
            } catch (t: Throwable) {
                browserError = t.message ?: "Failed to load folders"
                hasLoadedRows = true
            }
        }

        val rows =
            buildBrowserRows(
                tree = tree,
                currentScopeId = currentScopeId,
                expandedFolderIds = expandedFolderIds,
                focusedArtifactId = activeArtifactId,
            )

        return DocumentBrowserState(
            breadcrumbs = breadcrumbs,
            scopeBreadcrumbs = scopeBreadcrumbs,
            canNavigateScopeBack = scopeBackStack.isNotEmpty(),
            scopeBackTargetId = scopeBackStack.lastOrNull(),
            loading = !hasLoadedRows,
            errorMessage = browserError,
            rows = rows,
            activeRename = activeRename,
            activeVoiceCapture = activeVoiceCapture,
            eventSink = { event ->
                when (event) {
                    is DocumentBrowserEvent.OpenArtifact -> {
                        focusedFolderId = null
                        onOpenArtifact(event.artifactId)
                        expandedFolderIds = expandedFolderIds + ancestorFolderIdsForArtifact(tree, event.artifactId)
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
                        expandedFolderIds = expandedFolderIds + ancestorFolderIds(tree, event.folderId)
                    }
                    is DocumentBrowserEvent.CreateInFolder -> {
                        scope.launch {
                            when (event.option) {
                                BrowserCreateOption.Folder -> {
                                    try {
                                        val created =
                                            folderRepository.createFolder(
                                                event.folderId,
                                                nextFolderTitle(tree, event.folderId),
                                            )
                                        tree = folderRepository.browserTree()
                                        browserError = null
                                        focusedFolderId = created.id
                                        expandedFolderIds = expandedFolderIds + ancestorFolderIds(tree, created.id)
                                    } catch (t: Throwable) {
                                        browserError = t.message ?: "Failed to create folder"
                                    }
                                }
                                BrowserCreateOption.Note,
                                BrowserCreateOption.Link -> {
                                    try {
                                        val kind = event.option.toArtifactKind()
                                        val title = nextArtifactTitle(tree, event.folderId, kind)
                                        val created =
                                            folderRepository.createArtifact(
                                                event.folderId,
                                                title,
                                                kind,
                                            )
                                        tree =
                                            folderRepository.browserTree()
                                        browserError = null
                                        focusedFolderId = null
                                        onOpenArtifact(created.id)
                                        expandedFolderIds = expandedFolderIds + ancestorFolderIds(tree, created.folderId)
                                    } catch (t: Throwable) {
                                        browserError = t.message ?: "Failed to create artifact"
                                    }
                                }
                                BrowserCreateOption.Upload -> Unit
                                BrowserCreateOption.Voice -> {
                                    voiceCaptureTargetFolderId = event.folderId
                                    activeVoiceCapture = newVoiceCaptureState(tree, event.folderId)
                                    voiceCaptureResult = null
                                }
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
                            setTree = { tree = it },
                        )
                    }
                    is DocumentBrowserEvent.CancelRename -> {
                        activeRename = activeRename?.takeUnless { it.rowId == event.rowId }
                    }
                    DocumentBrowserEvent.CreateFolder -> {
                        scope.launch {
                            try {
                                val targetFolderId = focusedFolderId ?: currentScopeId
                                val created =
                                    folderRepository.createFolder(
                                        targetFolderId,
                                        nextFolderTitle(tree, targetFolderId),
                                    )
                                tree = folderRepository.browserTree()
                                browserError = null
                                focusedFolderId = created.id
                                expandedFolderIds = expandedFolderIds + ancestorFolderIds(tree, created.id)
                            } catch (t: Throwable) {
                                browserError = t.message ?: "Failed to create folder"
                            }
                        }
                    }
                    DocumentBrowserEvent.CreateArtifact -> {
                        scope.launch {
                            try {
                                val targetFolderId = focusedFolderId ?: currentScopeId
                                val created =
                                    folderRepository.createArtifact(
                                        targetFolderId,
                                        nextArtifactTitle(tree, targetFolderId),
                                    )
                                tree = folderRepository.browserTree()
                                browserError = null
                                focusedFolderId = null
                                onOpenArtifact(created.id)
                                expandedFolderIds = expandedFolderIds + ancestorFolderIds(tree, created.folderId)
                            } catch (t: Throwable) {
                                browserError = t.message ?: "Failed to create artifact"
                            }
                        }
                    }
                    DocumentBrowserEvent.CreateVoiceArtifact -> {
                        val targetFolderId = focusedFolderId ?: currentScopeId
                        voiceCaptureTargetFolderId = targetFolderId
                        activeVoiceCapture = newVoiceCaptureState(tree, targetFolderId)
                        voiceCaptureResult = null
                    }
                    is DocumentBrowserEvent.StartVoiceCapture -> {
                        voiceCaptureTargetFolderId = event.folderId
                        activeVoiceCapture = newVoiceCaptureState(tree, event.folderId)
                        voiceCaptureResult = null
                    }
                    DocumentBrowserEvent.BeginVoiceRecording -> {
                        val current = activeVoiceCapture ?: return@DocumentBrowserState
                        voicePlaybackHandle?.dispose()
                        voicePlaybackHandle = null
                        activeVoiceCapture =
                            current.copy(
                                phase = BrowserVoiceCapturePhase.Starting,
                                elapsedMs = 0,
                                levels = emptyList(),
                                playbackUrl = null,
                                isPlaying = false,
                                playbackPositionMs = 0,
                                playbackDurationMs = 0,
                                errorMessage = null,
                            )
                        voiceCaptureResult = null
                        voiceCaptureSession =
                            voiceCaptureService.start(
                                onLevel = { level ->
                                    scope.launch {
                                        activeVoiceCapture =
                                            activeVoiceCapture?.let {
                                                if (it.phase != BrowserVoiceCapturePhase.Recording) it
                                                else it.copy(levels = (it.levels + level).takeLast(42))
                                            }
                                    }
                                },
                                onStarted = {
                                    scope.launch {
                                        activeVoiceCapture =
                                            activeVoiceCapture?.copy(
                                                phase = BrowserVoiceCapturePhase.Recording,
                                                errorMessage = null,
                                            )
                                        voiceCaptureTimerJob?.cancel()
                                        voiceCaptureTimerJob =
                                            launch {
                                                var elapsed = 0L
                                                while (true) {
                                                    delay(250)
                                                    elapsed += 250
                                                    activeVoiceCapture =
                                                        activeVoiceCapture?.let {
                                                            if (it.phase == BrowserVoiceCapturePhase.Recording)
                                                                it.copy(elapsedMs = elapsed)
                                                            else it
                                                        }
                                                }
                                            }
                                    }
                                },
                                onError = { message ->
                                    scope.launch {
                                        activeVoiceCapture =
                                            activeVoiceCapture?.copy(
                                                phase = BrowserVoiceCapturePhase.Failed,
                                                errorMessage = message,
                                            )
                                    }
                                },
                            )
                    }
                    DocumentBrowserEvent.StopVoiceRecording -> {
                        voiceCaptureTimerJob?.cancel()
                        voiceCaptureTimerJob = null
                        val session = voiceCaptureSession ?: return@DocumentBrowserState
                        session.stop(
                            onComplete = { result ->
                                scope.launch {
                                    val playbackUrl = result.toPlaybackDataUrl()
                                    voiceCaptureResult = result
                                    voicePlaybackHandle?.dispose()
                                    voicePlaybackHandle =
                                        createVoicePlaybackHandle(
                                            url = playbackUrl,
                                            onPositionChanged = { positionMs ->
                                                scope.launch {
                                                    activeVoiceCapture =
                                                        activeVoiceCapture?.copy(
                                                            playbackPositionMs = positionMs,
                                                        )
                                                }
                                            },
                                            onEnded = {
                                                scope.launch {
                                                    activeVoiceCapture =
                                                        activeVoiceCapture?.copy(
                                                            isPlaying = false,
                                                            playbackPositionMs = result.durationMs,
                                                        )
                                                }
                                            },
                                            onError = { message ->
                                                scope.launch {
                                                    activeVoiceCapture =
                                                        activeVoiceCapture?.copy(
                                                            isPlaying = false,
                                                            errorMessage = message,
                                                        )
                                                }
                                            },
                                        )
                                    activeVoiceCapture =
                                        activeVoiceCapture?.copy(
                                            phase = BrowserVoiceCapturePhase.Review,
                                            elapsedMs = result.durationMs,
                                            playbackUrl = playbackUrl,
                                            isPlaying = false,
                                            playbackPositionMs = 0,
                                            playbackDurationMs = result.durationMs,
                                            errorMessage = null,
                                        )
                                }
                            },
                            onError = { message ->
                                scope.launch {
                                    activeVoiceCapture =
                                        activeVoiceCapture?.copy(
                                            phase = BrowserVoiceCapturePhase.Failed,
                                            errorMessage = message,
                                        )
                                }
                            },
                        )
                    }
                    DocumentBrowserEvent.ToggleVoicePlayback -> {
                        val capture = activeVoiceCapture ?: return@DocumentBrowserState
                        val playback = voicePlaybackHandle ?: return@DocumentBrowserState
                        if (capture.phase != BrowserVoiceCapturePhase.Review) return@DocumentBrowserState
                        if (capture.isPlaying) {
                            playback.pause()
                            activeVoiceCapture = capture.copy(isPlaying = false)
                        } else {
                            playback.play()
                            activeVoiceCapture = capture.copy(isPlaying = true, errorMessage = null)
                        }
                    }
                    is DocumentBrowserEvent.VoicePlaybackPositionChanged -> {
                        activeVoiceCapture =
                            activeVoiceCapture?.copy(
                                playbackPositionMs = event.positionMs.coerceAtLeast(0),
                            )
                    }
                    is DocumentBrowserEvent.SeekVoicePlayback -> {
                        val capture = activeVoiceCapture ?: return@DocumentBrowserState
                        val playback = voicePlaybackHandle ?: return@DocumentBrowserState
                        val nextPosition =
                            event.positionMs.coerceIn(0, capture.playbackDurationMs.coerceAtLeast(0))
                        playback.seek(nextPosition)
                        activeVoiceCapture =
                            capture.copy(
                                playbackPositionMs = nextPosition,
                                errorMessage = null,
                            )
                    }
                    DocumentBrowserEvent.VoicePlaybackEnded -> {
                        activeVoiceCapture =
                            activeVoiceCapture?.copy(
                                isPlaying = false,
                                playbackPositionMs = activeVoiceCapture?.playbackDurationMs ?: 0,
                            )
                    }
                    is DocumentBrowserEvent.VoiceCaptureTitleChanged -> {
                        activeVoiceCapture = activeVoiceCapture?.copy(title = event.title)
                    }
                    is DocumentBrowserEvent.VoiceCaptureDescriptionChanged -> {
                        activeVoiceCapture =
                            activeVoiceCapture?.copy(description = event.description)
                    }
                    DocumentBrowserEvent.SaveVoiceCapture -> {
                        val capture = activeVoiceCapture ?: return@DocumentBrowserState
                        voicePlaybackHandle?.pause()
                        val targetFolderId = voiceCaptureTargetFolderId
                        val title =
                            capture.title.trim().ifBlank {
                                nextArtifactTitle(tree, targetFolderId, ArtifactKind.Voice)
                            }
                        activeVoiceCapture = capture.copy(phase = BrowserVoiceCapturePhase.Saving)
                        scope.launch {
                            val created =
                                createLocalVoiceArtifact(
                                    folderId = targetFolderId,
                                    title = title,
                                    summary = capture.description.trim().ifBlank { null },
                                )
                            tree = tree.addLocalArtifactNode(created)
                            browserError = null
                            focusedFolderId = null
                            activeVoiceCapture = null
                            voiceCaptureResult = null
                            voiceCaptureTargetFolderId = null
                            voiceCaptureSession = null
                            voicePlaybackHandle?.dispose()
                            voicePlaybackHandle = null
                            onOpenArtifact(created.id)
                            expandedFolderIds =
                                expandedFolderIds + ancestorFolderIds(tree, created.folderId)
                        }
                    }
                    DocumentBrowserEvent.CancelVoiceCapture -> {
                        voiceCaptureTimerJob?.cancel()
                        voiceCaptureTimerJob = null
                        voiceCaptureSession?.cancel()
                        voiceCaptureSession = null
                        voicePlaybackHandle?.dispose()
                        voicePlaybackHandle = null
                        voiceCaptureResult = null
                        voiceCaptureTargetFolderId = null
                        activeVoiceCapture = null
                    }
                    DocumentBrowserEvent.RetryLoad -> {
                        scope.launch {
                            browserError = null
                            hasLoadedRows = false
                            try {
                                tree = folderRepository.browserTree()
                                browserError = null
                            } catch (t: Throwable) {
                                browserError = t.message ?: "Failed to load folders"
                            } finally {
                                hasLoadedRows = true
                            }
                        }
                    }
                }
            },
        )
    }

    private fun buildBrowserRows(
        tree: List<BrowserTreeNode>,
        currentScopeId: String?,
        expandedFolderIds: Set<String>,
        focusedArtifactId: String?,
    ): List<BrowserTreeRow> {
        val rows = mutableListOf<BrowserTreeRow>()

        fun visit(parentId: String?, depth: Int, remainingDepth: Int) {
            val nodes = tree.childrenOf(parentId)
            nodes.forEach { node ->
                when (node.kind) {
                    BrowserNodeKind.Folder -> appendFolderNode(rows, node, depth, remainingDepth, expandedFolderIds, ::visit)
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
                selected = false,
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

    private fun createLocalVoiceArtifact(
        folderId: String?,
        title: String,
        summary: String?,
    ): Artifact {
        val artifact =
            Artifact(
                id = "local-voice-${Random.nextInt(100_000, 999_999)}",
                folderId = folderId,
                title = title,
                content = "",
                summary = summary ?: "Recording capture will be wired in a backend pass.",
                kind = ArtifactKind.Voice,
                updatedLabel = "Just now",
                resourceUrl = null,
            )
        artifactRepository.registerLocalArtifact(artifact)
        return artifact
    }

    private fun commitRename(
        rename: BrowserRenameState,
        setActiveRename: (BrowserRenameState?) -> Unit,
        setBrowserError: (String?) -> Unit,
        setFocusedFolderId: (String?) -> Unit,
        setTree: (List<BrowserTreeNode>) -> Unit,
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
                        val renamed = folderRepository.renameFolder(rename.rowId, nextTitle)
                        setFocusedFolderId(renamed.id)
                        setTree(folderRepository.browserTree())
                    }
                    BrowserRowKind.Artifact -> {
                        artifactRepository.renameArtifact(rename.rowId, nextTitle)
                        setTree(folderRepository.browserTree())
                    }
                }
                setBrowserError(null)
                setActiveRename(null)
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

private fun ancestorFolderIds(tree: List<BrowserTreeNode>, id: String?): Set<String> {
    val ids = linkedSetOf<String>()
    var currentId = id
    while (currentId != null) {
        val current = tree.findNode { it.id == currentId } ?: break
        if (current.kind == BrowserNodeKind.Folder) {
            ids += current.id
        }
        currentId = current.parentId
    }
    return ids
}

private fun ancestorFolderIdsForArtifact(tree: List<BrowserTreeNode>, artifactId: String?): Set<String> {
    val id = artifactId ?: return emptySet()
    val node = tree.findNode { it.artifactPreview?.id == id || it.id == id } ?: return emptySet()
    return ancestorFolderIds(tree, node.parentId)
}

private fun buildFolderBreadcrumbs(
    tree: List<BrowserTreeNode>,
    folderId: String?,
): List<BreadcrumbItem> {
    if (folderId == null) return listOf(BreadcrumbItem(id = null, label = "Folders"))
    val folders = mutableListOf<BrowserTreeNode>()
    var currentId: String? = folderId
    while (currentId != null) {
        val folder = tree.findNode { it.kind == BrowserNodeKind.Folder && it.id == currentId } ?: break
        folders += folder
        currentId = folder.parentId
    }
    return buildList {
        add(BreadcrumbItem(id = null, label = "Folders"))
        folders.asReversed().forEach { folder ->
            add(BreadcrumbItem(id = folder.id, label = folder.title))
        }
    }
}

private fun nextFolderTitle(tree: List<BrowserTreeNode>, parentId: String?): String {
    val count = tree.childrenOf(parentId).count { it.kind == BrowserNodeKind.Folder } + 1
    return "New Folder $count"
}

private fun nextArtifactTitle(tree: List<BrowserTreeNode>, parentId: String?): String {
    val count = tree.childrenOf(parentId).count { it.kind == BrowserNodeKind.Artifact } + 1
    return "New Artifact $count"
}

private fun nextArtifactTitle(
    tree: List<BrowserTreeNode>,
    parentId: String?,
    kind: ArtifactKind,
): String {
    val count =
        tree.childrenOf(parentId).count {
            it.kind == BrowserNodeKind.Artifact && it.artifactPreview?.kind == kind
        } + 1
    return when (kind) {
        ArtifactKind.Note -> "New Note $count"
        ArtifactKind.Link -> "New Link $count"
        ArtifactKind.Voice -> "New Voice $count"
        ArtifactKind.File -> "New File $count"
    }
}

private fun newVoiceCaptureState(
    tree: List<BrowserTreeNode>,
    folderId: String?,
): BrowserVoiceCaptureState =
    BrowserVoiceCaptureState(
        title = nextArtifactTitle(tree, folderId, ArtifactKind.Voice),
        description = "",
        phase = BrowserVoiceCapturePhase.Ready,
        elapsedMs = 0,
        levels = emptyList(),
        playbackUrl = null,
        isPlaying = false,
        playbackPositionMs = 0,
        playbackDurationMs = 0,
        errorMessage = null,
    )

private fun VoiceCaptureResult.toPlaybackDataUrl(): String {
    val mediaType = contentType ?: "audio/webm"
    return "data:$mediaType;base64,$base64Data"
}

private fun BrowserCreateOption.toArtifactKind(): ArtifactKind =
    when (this) {
        BrowserCreateOption.Link -> ArtifactKind.Link
        BrowserCreateOption.Voice -> ArtifactKind.Voice
        else -> ArtifactKind.Note
    }

private fun List<BrowserTreeNode>.addLocalArtifactNode(artifact: Artifact): List<BrowserTreeNode> {
    val node =
        BrowserTreeNode(
            id = artifact.id,
            parentId = artifact.folderId,
            kind = BrowserNodeKind.Artifact,
            title = artifact.title,
            artifactId = artifact.id,
            artifactPreview =
                ArtifactPreview(
                    id = artifact.id,
                    title = artifact.title,
                    kind = artifact.kind,
                    updatedLabel = artifact.updatedLabel,
                ),
        )
    if (artifact.folderId == null) return this + node
    return map { current ->
        if (current.kind == BrowserNodeKind.Folder && current.id == artifact.folderId) {
            current.copy(children = current.children + node)
        } else {
            current.copy(children = current.children.addLocalArtifactNode(artifact))
        }
    }
}

private fun List<BrowserTreeNode>.childrenOf(parentId: String?): List<BrowserTreeNode> {
    if (parentId == null) return this
    return findNode { it.kind == BrowserNodeKind.Folder && it.id == parentId }?.children.orEmpty()
}

private fun List<BrowserTreeNode>.findNode(predicate: (BrowserTreeNode) -> Boolean): BrowserTreeNode? {
    fun visit(node: BrowserTreeNode): BrowserTreeNode? {
        if (predicate(node)) return node
        return node.children.firstNotNullOfOrNull(::visit)
    }
    return firstNotNullOfOrNull(::visit)
}
