package dawn.system.anchor.features.shell.state

import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.remember
import dawn.system.anchor.domain.Presence
import dawn.system.anchor.domain.WorkspaceNode
import dawn.system.anchor.services.data.NodeStore
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.flow.map

/**
 * Reading one node inside a composition.
 *
 * Both helpers wrap the subscription in `key(id)`, and that is **load-bearing rather than
 * tidy**. `collectAsState` is `produceState`, whose backing
 * `remember { mutableStateOf(initialValue) }` has *no keys* (`ProduceState.kt:140`): when the
 * flow changes, the effect restarts but the State keeps the previous flow's value, so
 * `initial` applies exactly once, at first composition.
 *
 * That is what shipped as "no folder can ever be entered". At section level there is no
 * folder, so the read was a genuine "absent"; drilling in swapped the flow but not the State,
 * and the absent read was attributed to the folder just entered. The correction rule saw
 * [Presence.Gone] and stepped back out, settling into the state that reproduces it.
 *
 * `key` gives the whole group a new identity, so the reset is real. Anything that reads a
 * node against a changing id must go through here.
 */

/** The row, or null while unread *or* once it is gone. Display cannot tell, and should not. */
@Composable
internal fun NodeStore.nodeOf(id: String?): WorkspaceNode? = key(id) {
    val row by remember { id?.let { node(it) } ?: flowOf(null) }.collectAsState(initial = null)
    row
}

/**
 * Whether the tree can vouch for [id] — the only question a correction rule may ask.
 *
 * [Presence.Unknown] is the *initial*, which is the point: "no emission yet" is precisely what
 * Unknown means, so the store never has to name a state it cannot observe. Only a positive
 * read of absence produces [Presence.Gone], and only that may move the user.
 */
@Composable
internal fun NodeStore.presenceOf(id: String?): Presence = key(id) {
    val presence by remember {
        (id?.let { node(it) } ?: flowOf(null))
            .map { row -> if (row == null) Presence.Gone else Presence.There }
    }.collectAsState(initial = Presence.Unknown)
    presence
}
