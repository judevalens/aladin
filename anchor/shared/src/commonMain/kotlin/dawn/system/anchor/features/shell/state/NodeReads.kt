package dawn.system.anchor.features.shell.state

import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.remember
import dawn.system.anchor.domain.Presence
import dawn.system.anchor.domain.WorkspaceNode
import dawn.system.anchor.services.data.NodeStore
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.flow.map

/**
 * Reading nodes inside a composition.
 *
 * Every helper here wraps its subscription in `key(...)`, and that is **load-bearing rather
 * than tidy**. `collectAsState` is `produceState`, whose backing
 * `remember { mutableStateOf(initialValue) }` has *no keys* (`ProduceState.kt:140`): when the
 * flow changes, the effect restarts but the State keeps the previous flow's value, so
 * `initial` applies exactly once, at first composition.
 *
 * That is what shipped as "no folder can ever be entered". At section level there was no
 * folder, so the read was a genuine "absent"; drilling in swapped the flow but not the State,
 * and the absent read was attributed to the folder just entered. The correction rule saw
 * [Presence.Gone] and stepped back out, settling into the state that reproduces it.
 *
 * `key` gives the whole group a new identity, so the reset is real. Anything reading a node
 * against a changing id must come through here.
 */

/**
 * The row, or null while unread *or* once it is gone. Display cannot tell, and should not.
 *
 * **Seeded from the stream's replay cache**, which is the other half of the `key` above and
 * just as load-bearing. `key` correctly gives each id a fresh group, so `collectAsState` starts
 * at its `initial` — and a null initial is indistinguishable from "there is no such row". For
 * one frame after every switch the title read "Untitled", the kind read null, and the shell
 * concluded the open document was not a PDF and covered the reader with a placeholder.
 *
 * The value was already in hand the whole time: the stream is shared with `replay = 1`, so a
 * document read even once has its row sitting in `replayCache`. Reading that synchronously is
 * what makes a switch paint the right thing on its first frame rather than its second.
 *
 * A genuinely unread id still seeds null, which is correct — that one is not a flicker.
 */
@Composable
internal fun NodeStore.nodeOf(id: String?): WorkspaceNode? = key(id) {
    // No early return: a `return@key` out of a composable lambda breaks the compiler plugin's
    // group bookkeeping and produces a ClassFormatError at runtime rather than a build error.
    val stream = remember { id?.let { node(it) } }
    val seed = remember(stream) { stream?.replayCache?.firstOrNull() }
    val flow = remember(stream) { stream ?: flowOf(null) }
    val row by flow.collectAsState(initial = seed)
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
    val stream = remember { id?.let { node(it) } }
    // Same replay-cache seed as [nodeOf]: an id already read answers on the first frame rather
    // than passing back through Unknown on its way to the truth it already holds.
    val seed = remember(stream) {
        when {
            stream == null -> Presence.Gone
            stream.replayCache.isEmpty() -> Presence.Unknown
            stream.replayCache.first() == null -> Presence.Gone
            else -> Presence.There
        }
    }
    val flow = remember(stream) {
        stream?.map { row -> if (row == null) Presence.Gone else Presence.There }
            ?: flowOf(Presence.Gone)
    }
    val presence by flow.collectAsState(initial = seed)
    presence
}

/**
 * Presence for a whole set, in the shape [dawn.system.anchor.domain.Nav.corrected] takes.
 *
 * Answers [Presence.Unknown] for anything outside [ids], so a correction rule can only act on
 * something deliberately watched. `combine` withholds its first emission until *every* stream
 * has produced one, which leaves the whole set Unknown while any member is unread —
 * conservative in the only direction that is safe, since Unknown moves nothing.
 */
@Composable
internal fun NodeStore.presenceOf(ids: List<String>): (String) -> Presence = key(ids) {
    val empty: Flow<List<WorkspaceNode?>> = flowOf(emptyList())
    val rows: List<WorkspaceNode?>? by remember {
        if (ids.isEmpty()) empty else combine(ids.map { node(it) }) { it.toList() }
    }.collectAsState(initial = null)

    val read = rows
    remember(ids, read) { presenceLookup(ids, read) }
}

/**
 * The Miller path that lands you *on* [nodeId] — its ancestor folders, root-most first.
 *
 * Walks up one read per level. The loop is a fixed [MAX_TREE_DEPTH] iterations rather than
 * "until the root", because a composable's call structure has to be stable across frames; the
 * unused levels read null and cost nothing beyond a map lookup on an already-shared stream.
 *
 * Bounded deliberately: a cycle in `parentId` — which the tree should never contain but a bad
 * frame could — would otherwise spin forever inside composition.
 */
@Composable
internal fun NodeStore.ancestorPathOf(nodeId: String?): List<String> {
    val chain = ArrayList<String>(MAX_TREE_DEPTH)
    var cursor = nodeId
    repeat(MAX_TREE_DEPTH) {
        val parent = nodeOf(cursor)?.parentId
        if (parent != null && parent !in chain) chain.add(parent)
        cursor = parent
    }
    return chain.reversed()
}

private const val MAX_TREE_DEPTH = 8

/** Pure, so the composable above stays a subscription and nothing else. */
private fun presenceLookup(
    ids: List<String>,
    rows: List<WorkspaceNode?>?,
): (String) -> Presence {
    if (rows == null) return { Presence.Unknown }
    val watched: Map<String, WorkspaceNode?> = ids.zip(rows).toMap()
    return { id ->
        when {
            !watched.containsKey(id) -> Presence.Unknown
            watched.getValue(id) == null -> Presence.Gone
            else -> Presence.There
        }
    }
}
