package dawn.system.anchor.features.shell.state

import androidx.compose.runtime.Composable
import com.slack.circuit.runtime.CircuitUiState
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import dawn.system.anchor.domain.WorkspaceNode
import dawn.system.anchor.features.shell.ShellScreen
import dawn.system.anchor.services.data.NodeState
import dawn.system.anchor.services.data.NodeStore
import dawn.system.anchor.services.data.WorkspaceWriter
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.launch

/** Create, rename and delete, plus the sheets that drive them. */
data class WriteSlice(
    val actionsFor: WorkspaceNode?,
    val rename: ShellScreen.RenameDraft?,
    val pendingDelete: WorkspaceNode?,
    val writeError: String?,
    val handle: (ShellScreen.Event.Write) -> Unit,
) : CircuitUiState

/** What the user is typing, and which node it is for. */
private data class RenameInput(val nodeId: String, val title: String, val saving: Boolean = false)

/**
 * The workspace's writes.
 *
 * **Ids in, nodes resolved.** A sheet that is up while its subject is renamed or deleted
 * elsewhere has to follow the store, not the copy it was opened with — so nothing here holds
 * a `WorkspaceNode`, only the id of one, and the node is read from its own stream every
 * frame. The one genuinely local value is the rename *text*, because that is what the user
 * is typing and it exists nowhere else.
 *
 * Writes reach beyond this slice — creating a page selects it, deleting closes its tabs — so
 * those are callbacks rather than reads into someone else's state. That keeps the direction
 * of the dependency visible at the call site instead of buried here.
 */
class WriteStateProducer(
    private val nodes: NodeStore,
    private val writer: WorkspaceWriter,
) {

    @Composable
    operator fun invoke(
        /** Where a new item lands: the folder you are inside, or the section root. */
        parentId: String?,
        /** A page that was just created and should be opened. */
        onCreated: (WorkspaceNode) -> Unit,
        /** A node that is on its way out; anything pointing at it should stop. */
        onDeleted: (WorkspaceNode) -> Unit,
        /** Closes the "new" menu once a choice has been made. */
        onCreateChosen: () -> Unit,
    ): WriteSlice {
        val scope = rememberCoroutineScope()

        var actionsForId by remember { mutableStateOf<String?>(null) }
        var draft by remember { mutableStateOf<RenameInput?>(null) }
        var pendingDeleteId by remember { mutableStateOf<String?>(null) }
        var writeError by remember { mutableStateOf<String?>(null) }

        val actionsFor by remember(actionsForId) {
            actionsForId?.let(nodes::node) ?: flowOf(NodeState.Missing)
        }.collectAsState(initial = NodeState.Loading)

        val pendingDelete by remember(pendingDeleteId) {
            pendingDeleteId?.let(nodes::node) ?: flowOf(NodeState.Missing)
        }.collectAsState(initial = NodeState.Loading)

        val renameTarget by remember(draft?.nodeId) {
            draft?.nodeId?.let(nodes::node) ?: flowOf(NodeState.Missing)
        }.collectAsState(initial = NodeState.Loading)

        return WriteSlice(
            actionsFor = actionsFor.node,
            rename = draft?.let { input ->
                renameTarget.node?.let { ShellScreen.RenameDraft(it, input.title, input.saving) }
            },
            pendingDelete = pendingDelete.node,
            writeError = writeError,
        ) { event ->
            when (event) {
                is ShellScreen.Event.Write.ActionsRequested -> actionsForId = event.node.id
                ShellScreen.Event.Write.ActionsDismissed -> actionsForId = null

                ShellScreen.Event.Write.RenameStarted -> {
                    draft = actionsFor.node?.let { RenameInput(it.id, it.title) }
                    actionsForId = null
                }

                is ShellScreen.Event.Write.RenameEdited -> {
                    draft = draft?.copy(title = event.title)
                }

                ShellScreen.Event.Write.RenameCancelled -> draft = null

                ShellScreen.Event.Write.RenameCommitted -> {
                    val input = draft
                    val node = renameTarget.node
                    val title = input?.title?.trim()
                    if (input == null || node == null || title.isNullOrEmpty() || title == node.title) {
                        draft = null
                    } else {
                        draft = input.copy(saving = true)
                        scope.launch {
                            // No optimistic patch: the row changes when the server says it has,
                            // and the live frame that follows meets the seq guard.
                            runCatching { writer.rename(node, title) }
                                .onFailure { writeError = it.failureText("rename") }
                            draft = null
                        }
                    }
                }

                ShellScreen.Event.Write.DeleteRequested -> {
                    pendingDeleteId = actionsForId
                    actionsForId = null
                }

                ShellScreen.Event.Write.DeleteCancelled -> pendingDeleteId = null

                ShellScreen.Event.Write.DeleteConfirmed -> {
                    pendingDelete.node?.let { target ->
                        pendingDeleteId = null
                        scope.launch {
                            runCatching { writer.delete(target) }
                                .onFailure { writeError = it.failureText("delete") }
                        }
                        onDeleted(target)
                    }
                }

                ShellScreen.Event.Write.CreateFolder -> {
                    onCreateChosen()
                    scope.launch {
                        runCatching { writer.createFolder(parentId, "New folder") }
                            .onSuccess { id ->
                                // Straight into a rename: an untitled folder is never what you
                                // wanted, and naming it is the next thing you would do.
                                nodes.byId(id)?.let { draft = RenameInput(it.id, it.title) }
                            }
                            .onFailure { writeError = it.failureText("new folder") }
                    }
                }

                ShellScreen.Event.Write.CreatePage -> {
                    onCreateChosen()
                    scope.launch {
                        runCatching { writer.createPage(parentId, "Untitled page") }
                            .onSuccess { id -> nodes.byId(id)?.let(onCreated) }
                            .onFailure { writeError = it.failureText("new page") }
                    }
                }

                ShellScreen.Event.Write.WriteErrorDismissed -> writeError = null
            }
        }
    }
}

/**
 * What to tell the user when a write is refused. The server's own message when there is one,
 * because "couldn't rename" alone leaves nothing to act on.
 */
private fun Throwable.failureText(action: String): String =
    message?.takeIf { it.isNotBlank() } ?: "the $action didn't go through"
