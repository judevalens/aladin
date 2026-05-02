package com.jvp.aladin_compose.service

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.launch

interface PageDocumentSyncer {
    val updates: SharedFlow<PageDocumentSyncUpdate>

    fun open(pageId: String)

    fun close(pageId: String)

    fun localSnapshotChanged(pageId: String, markdown: String)

    fun retryLoad(pageId: String)

    fun retrySave(pageId: String, markdown: String)
}

data class PageDocumentSyncUpdate(
    val pageId: String,
    val revision: Long,
    val initialSnapshot: PageDocumentSnapshot?,
    val lastSavedAt: String?,
    val status: PageDocumentSyncStatus,
    val failure: PageDocumentSyncFailure?,
)

data class PageDocumentSnapshot(
    val markdown: String,
    val updatedAt: String?,
)

enum class PageDocumentSyncStatus {
    Loading,
    Ready,
    LocalChangesPending,
    Syncing,
    Synced,
    Failed,
}

sealed interface PageDocumentSyncFailure {
    data class LoadFailed(val message: String) : PageDocumentSyncFailure

    data class SaveFailed(val message: String) : PageDocumentSyncFailure
}

class DefaultPageDocumentSyncer(
    private val artifactService: ArtifactService,
    private val scope: CoroutineScope,
) : PageDocumentSyncer {
    private val runtimeByPageId = mutableMapOf<String, PageDocumentSyncRuntime>()
    private val commands =
        MutableSharedFlow<PageDocumentSyncCommand>(
            extraBufferCapacity = 64,
        )
    private val mutableUpdates =
        MutableSharedFlow<PageDocumentSyncUpdate>(
            replay = 64,
            extraBufferCapacity = 64,
        )

    override val updates: SharedFlow<PageDocumentSyncUpdate> = mutableUpdates.asSharedFlow()

    init {
        scope.launch {
            commands.collect(::processCommand)
        }
    }

    override fun open(pageId: String) {
        enqueue(PageDocumentSyncCommand.Open(pageId))
    }

    override fun close(pageId: String) {
        enqueue(PageDocumentSyncCommand.Close(pageId))
    }

    override fun localSnapshotChanged(pageId: String, markdown: String) {
        enqueue(PageDocumentSyncCommand.LocalSnapshotChanged(pageId, markdown))
    }

    override fun retryLoad(pageId: String) {
        enqueue(PageDocumentSyncCommand.RetryLoad(pageId))
    }

    override fun retrySave(pageId: String, markdown: String) {
        enqueue(PageDocumentSyncCommand.RetrySave(pageId, markdown))
    }

    private fun enqueue(command: PageDocumentSyncCommand) {
        if (!commands.tryEmit(command)) {
            scope.launch { commands.emit(command) }
        }
    }

    private fun processCommand(command: PageDocumentSyncCommand) {
        when (command) {
            is PageDocumentSyncCommand.Open -> handleOpen(command.pageId)
            is PageDocumentSyncCommand.Close -> handleClose(command.pageId)
            is PageDocumentSyncCommand.RetryLoad -> handleRetryLoad(command.pageId)
            is PageDocumentSyncCommand.LoadSucceeded ->
                handleLoadSucceeded(command.pageId, command.generation, command.snapshot)
            is PageDocumentSyncCommand.LoadFailed ->
                handleLoadFailed(command.pageId, command.generation, command.message)
            is PageDocumentSyncCommand.LocalSnapshotChanged ->
                handleLocalSnapshotChanged(command.pageId, command.markdown)
            is PageDocumentSyncCommand.RetrySave ->
                handleRetrySave(command.pageId, command.markdown)
            is PageDocumentSyncCommand.SaveSucceeded ->
                handleSaveSucceeded(command.pageId, command.generation, command.saved)
            is PageDocumentSyncCommand.SaveFailed ->
                handleSaveFailed(command.pageId, command.generation, command.message)
        }
    }

    private fun handleOpen(pageId: String) {
        if (pageId in runtimeByPageId) return

        val runtime =
            PageDocumentSyncRuntime(
                pageId = pageId,
                generation = nextGeneration(),
                revision = 1,
                state = PageDocumentSyncFsmState.Loading,
            )
        runtimeByPageId[pageId] = runtime
        emitRuntime(runtime)
        startLoad(runtime)
    }

    private fun handleClose(pageId: String) {
        runtimeByPageId.remove(pageId)?.cancel()
    }

    private fun handleRetryLoad(pageId: String) {
        val runtime = runtimeByPageId[pageId] ?: return handleOpen(pageId)
        runtime.cancel()
        val next = runtime.nextRevision(state = PageDocumentSyncFsmState.Loading)
        runtimeByPageId[pageId] = next
        emitRuntime(next)
        startLoad(next)
    }

    private fun handleLoadSucceeded(
        pageId: String,
        generation: Long,
        snapshot: PageDocumentSnapshot,
    ) {
        val runtime = runtimeByPageId.valid(pageId, generation) ?: return
        val next =
            runtime.nextRevision(
                state =
                    PageDocumentSyncFsmState.Synced(
                        lastSavedMarkdown = snapshot.markdown,
                        lastSavedAt = snapshot.updatedAt,
                    ),
                loadJob = null,
                initialSnapshotToEmit = snapshot,
            )
        runtimeByPageId[pageId] = next
        emitRuntime(next)
        runtimeByPageId[pageId] = next.copy(initialSnapshotToEmit = null)
    }

    private fun handleLoadFailed(
        pageId: String,
        generation: Long,
        message: String,
    ) {
        val runtime = runtimeByPageId.valid(pageId, generation) ?: return
        val next =
            runtime.nextRevision(
                state = PageDocumentSyncFsmState.LoadFailed(message),
                loadJob = null,
            )
        runtimeByPageId[pageId] = next
        emitRuntime(next)
    }

    private fun handleLocalSnapshotChanged(pageId: String, markdown: String) {
        val runtime = runtimeByPageId[pageId] ?: return
        if (runtime.state is PageDocumentSyncFsmState.Saving) {
            val next =
                runtime.nextRevision(
                    state = runtime.state.copy(latestMarkdown = markdown),
                )
            runtimeByPageId[pageId] = next
            emitRuntime(next)
            return
        }

        val dirty = runtime.state.toDirty(markdown) ?: return
        val next =
            runtime.nextRevision(
                state = dirty,
            )
        runtimeByPageId[pageId] = next
        startSave(next, dirty.latestMarkdown)
    }

    private fun handleRetrySave(pageId: String, markdown: String) {
        val runtime = runtimeByPageId[pageId] ?: return
        val dirty = runtime.state.toDirty(markdown) ?: return
        val next =
            runtime.nextRevision(
                state = dirty,
            )
        runtimeByPageId[pageId] = next
        startSave(next, markdown)
    }

    private fun handleSaveSucceeded(
        pageId: String,
        generation: Long,
        saved: PageDocumentSnapshot,
    ) {
        val runtime = runtimeByPageId.valid(pageId, generation) ?: return
        val saving = runtime.state as? PageDocumentSyncFsmState.Saving ?: return
        val latest = saving.latestMarkdown
        val nextState =
            if (latest != saved.markdown) {
                PageDocumentSyncFsmState.Saving(
                    savingMarkdown = latest,
                    latestMarkdown = latest,
                    lastSavedMarkdown = saved.markdown,
                    lastSavedAt = saved.updatedAt,
                )
            } else {
                PageDocumentSyncFsmState.Synced(
                    lastSavedMarkdown = saved.markdown,
                    lastSavedAt = saved.updatedAt,
                )
            }
        val next =
            runtime.nextRevision(
                state = nextState,
                saveJob = null,
            )
        runtimeByPageId[pageId] = next
        emitRuntime(next)

        if (nextState is PageDocumentSyncFsmState.Saving) {
            launchSave(next, latest)
        }
    }

    private fun handleSaveFailed(
        pageId: String,
        generation: Long,
        message: String,
    ) {
        val runtime = runtimeByPageId.valid(pageId, generation) ?: return
        val saving = runtime.state as? PageDocumentSyncFsmState.Saving ?: return
        val next =
            runtime.nextRevision(
                state =
                    PageDocumentSyncFsmState.SaveFailed(
                        latestMarkdown = saving.latestMarkdown,
                        lastSavedMarkdown = saving.lastSavedMarkdown,
                        lastSavedAt = saving.lastSavedAt,
                        message = message,
                    ),
                saveJob = null,
            )
        runtimeByPageId[pageId] = next
        emitRuntime(next)
    }

    private fun startLoad(runtime: PageDocumentSyncRuntime) {
        val job =
            scope.launch {
                try {
                    val page = artifactService.page(runtime.pageId)
                    if (page == null) {
                        commands.emit(
                            PageDocumentSyncCommand.LoadFailed(
                                pageId = runtime.pageId,
                                generation = runtime.generation,
                                message = "Page not found",
                            ),
                        )
                        return@launch
                    }
                    commands.emit(
                        PageDocumentSyncCommand.LoadSucceeded(
                            pageId = runtime.pageId,
                            generation = runtime.generation,
                            snapshot =
                                PageDocumentSnapshot(
                                    markdown = page.content,
                                    updatedAt = page.updatedAt,
                                ),
                        ),
                    )
                } catch (error: Throwable) {
                    commands.emit(
                        PageDocumentSyncCommand.LoadFailed(
                            pageId = runtime.pageId,
                            generation = runtime.generation,
                            message = error.message ?: "Failed to load page",
                        ),
                    )
                }
            }
        runtimeByPageId[runtime.pageId] = runtime.copy(loadJob = job)
    }

    private fun startSave(
        runtime: PageDocumentSyncRuntime,
        markdown: String,
    ) {
        if (runtime.saveJob != null) return

        val state = runtime.state
        val saveBaseline =
            when (state) {
                is PageDocumentSyncFsmState.Dirty -> state.lastSavedMarkdown to state.lastSavedAt
                is PageDocumentSyncFsmState.SaveFailed -> state.lastSavedMarkdown to state.lastSavedAt
                else -> return
            }
        val lastSaved = saveBaseline.first
        val lastSavedAt = saveBaseline.second
        if (markdown == lastSaved) return

        val next =
            runtime.nextRevision(
                state =
                    PageDocumentSyncFsmState.Saving(
                        savingMarkdown = markdown,
                        latestMarkdown = markdown,
                        lastSavedMarkdown = lastSaved,
                        lastSavedAt = lastSavedAt,
                    ),
            )
        runtimeByPageId[runtime.pageId] = next
        emitRuntime(next)
        launchSave(next, markdown)
    }

    private fun launchSave(
        runtime: PageDocumentSyncRuntime,
        markdown: String,
    ) {
        val job =
            scope.launch {
                try {
                    val saved = artifactService.savePage(runtime.pageId, markdown)
                    commands.emit(
                        PageDocumentSyncCommand.SaveSucceeded(
                            pageId = runtime.pageId,
                            generation = runtime.generation,
                            saved =
                                PageDocumentSnapshot(
                                    markdown = saved.content,
                                    updatedAt = saved.updatedAt,
                                ),
                        ),
                    )
                } catch (error: Throwable) {
                    commands.emit(
                        PageDocumentSyncCommand.SaveFailed(
                            pageId = runtime.pageId,
                            generation = runtime.generation,
                            message = error.message ?: "Failed to save page",
                        ),
                    )
                }
            }
        runtimeByPageId[runtime.pageId] = runtime.copy(saveJob = job)
    }

    private fun emitRuntime(runtime: PageDocumentSyncRuntime) {
        mutableUpdates.tryEmit(runtime.toUpdate())
    }

    private fun nextGeneration(): Long {
        generationCounter += 1
        return generationCounter
    }

    private var generationCounter = 0L
}

private sealed interface PageDocumentSyncCommand {
    data class Open(val pageId: String) : PageDocumentSyncCommand

    data class Close(val pageId: String) : PageDocumentSyncCommand

    data class RetryLoad(val pageId: String) : PageDocumentSyncCommand

    data class LoadSucceeded(
        val pageId: String,
        val generation: Long,
        val snapshot: PageDocumentSnapshot,
    ) : PageDocumentSyncCommand

    data class LoadFailed(
        val pageId: String,
        val generation: Long,
        val message: String,
    ) : PageDocumentSyncCommand

    data class LocalSnapshotChanged(
        val pageId: String,
        val markdown: String,
    ) : PageDocumentSyncCommand

    data class RetrySave(
        val pageId: String,
        val markdown: String,
    ) : PageDocumentSyncCommand

    data class SaveSucceeded(
        val pageId: String,
        val generation: Long,
        val saved: PageDocumentSnapshot,
    ) : PageDocumentSyncCommand

    data class SaveFailed(
        val pageId: String,
        val generation: Long,
        val message: String,
    ) : PageDocumentSyncCommand
}

private data class PageDocumentSyncRuntime(
    val pageId: String,
    val generation: Long,
    val revision: Long,
    val state: PageDocumentSyncFsmState,
    val initialSnapshotToEmit: PageDocumentSnapshot? = null,
    val loadJob: Job? = null,
    val saveJob: Job? = null,
) {
    fun nextRevision(
        state: PageDocumentSyncFsmState,
        initialSnapshotToEmit: PageDocumentSnapshot? = null,
        loadJob: Job? = this.loadJob,
        saveJob: Job? = this.saveJob,
    ): PageDocumentSyncRuntime =
        copy(
            revision = revision + 1,
            state = state,
            initialSnapshotToEmit = initialSnapshotToEmit,
            loadJob = loadJob,
            saveJob = saveJob,
        )

    fun toUpdate(): PageDocumentSyncUpdate {
        val failure =
            when (state) {
                is PageDocumentSyncFsmState.LoadFailed ->
                    PageDocumentSyncFailure.LoadFailed(state.message)
                is PageDocumentSyncFsmState.SaveFailed ->
                    PageDocumentSyncFailure.SaveFailed(state.message)
                else -> null
            }
        return PageDocumentSyncUpdate(
            pageId = pageId,
            revision = revision,
            initialSnapshot = initialSnapshotToEmit,
            lastSavedAt = state.lastSavedAt,
            status = state.status,
            failure = failure,
        )
    }

    fun cancel() {
        loadJob?.cancel()
        saveJob?.cancel()
    }
}

private sealed interface PageDocumentSyncFsmState {
    val status: PageDocumentSyncStatus
    val lastSavedAt: String?

    data object Loading : PageDocumentSyncFsmState {
        override val status: PageDocumentSyncStatus = PageDocumentSyncStatus.Loading
        override val lastSavedAt: String? = null
    }

    data class LoadFailed(
        val message: String,
    ) : PageDocumentSyncFsmState {
        override val status: PageDocumentSyncStatus = PageDocumentSyncStatus.Failed
        override val lastSavedAt: String? = null
    }

    data class Synced(
        val lastSavedMarkdown: String,
        override val lastSavedAt: String?,
    ) : PageDocumentSyncFsmState {
        override val status: PageDocumentSyncStatus = PageDocumentSyncStatus.Synced
    }

    data class Dirty(
        val latestMarkdown: String,
        val lastSavedMarkdown: String,
        override val lastSavedAt: String?,
    ) : PageDocumentSyncFsmState {
        override val status: PageDocumentSyncStatus = PageDocumentSyncStatus.Ready
    }

    data class Saving(
        val savingMarkdown: String,
        val latestMarkdown: String,
        val lastSavedMarkdown: String,
        override val lastSavedAt: String?,
    ) : PageDocumentSyncFsmState {
        override val status: PageDocumentSyncStatus = PageDocumentSyncStatus.Syncing
    }

    data class SaveFailed(
        val latestMarkdown: String,
        val lastSavedMarkdown: String,
        override val lastSavedAt: String?,
        val message: String,
    ) : PageDocumentSyncFsmState {
        override val status: PageDocumentSyncStatus = PageDocumentSyncStatus.Failed
    }
}

private fun PageDocumentSyncFsmState.toDirty(markdown: String): PageDocumentSyncFsmState.Dirty? =
    when (this) {
        is PageDocumentSyncFsmState.Synced ->
            PageDocumentSyncFsmState.Dirty(
                latestMarkdown = markdown,
                lastSavedMarkdown = lastSavedMarkdown,
                lastSavedAt = lastSavedAt,
            )
        is PageDocumentSyncFsmState.Dirty ->
            copy(latestMarkdown = markdown)
        is PageDocumentSyncFsmState.Saving ->
            copy(latestMarkdown = markdown).let {
                PageDocumentSyncFsmState.Dirty(
                    latestMarkdown = it.latestMarkdown,
                    lastSavedMarkdown = it.lastSavedMarkdown,
                    lastSavedAt = it.lastSavedAt,
                )
            }
        is PageDocumentSyncFsmState.SaveFailed ->
            PageDocumentSyncFsmState.Dirty(
                latestMarkdown = markdown,
                lastSavedMarkdown = lastSavedMarkdown,
                lastSavedAt = lastSavedAt,
            )
        PageDocumentSyncFsmState.Loading,
        is PageDocumentSyncFsmState.LoadFailed -> null
    }

private fun Map<String, PageDocumentSyncRuntime>.valid(
    pageId: String,
    generation: Long,
): PageDocumentSyncRuntime? {
    val runtime = this[pageId] ?: return null
    return runtime.takeIf { it.generation == generation }
}
