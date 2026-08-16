package dawn.system.anchor.features.shell.state

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import com.slack.circuit.runtime.CircuitUiState
import dawn.system.anchor.domain.Destination
import dawn.system.anchor.domain.Nav
import dawn.system.anchor.domain.TabKey
import dawn.system.anchor.services.data.NodeStore

/** Something the user did to move. Nothing here is a command the shell issues to itself. */
sealed interface NavEvent {
    data class GoTo(val destination: Destination) : NavEvent
    data class GoToBrowser(val path: List<String>, val itemId: String?) : NavEvent
    data class OpenDoc(val key: TabKey) : NavEvent
    data class CloseDoc(val key: TabKey) : NavEvent
    data class SelectFolder(val column: Int, val folderId: String) : NavEvent
    data class SelectItem(val itemId: String) : NavEvent
    data object Back : NavEvent
    data object Forward : NavEvent
}

/**
 * Where you are, corrected. [nav] is the value everything else should read — including this
 * slice's own handler, which starts every transition from it.
 */
data class NavSlice(
    val nav: Nav,
    val handle: (NavEvent) -> Unit,
) : CircuitUiState

/**
 * The one owner of the navigation value.
 *
 * The producer owns the presence read *and* the value it corrects, deliberately. Keeping them
 * apart is what the previous shell got wrong: the value lived in the presenter, the read lived
 * in a different producer, and the correction lived in an effect between them — so the rule
 * could be forgotten at a call site, and was.
 *
 * ```
 * raw   ← written ONLY by events. No effect, no write-back, no second frame.
 * nav   ← raw.corrected(presence). A derivation, recomputed as part of composition.
 * ```
 *
 * Because `nav` is derived rather than stored, a row that reappears un-corrects on its own,
 * and because events start from `nav`, the correction settles into `raw` the moment the user
 * acts rather than drifting away from it.
 */
class NavStateProducer(private val nodes: NodeStore) {

    @Composable
    operator fun invoke(): NavSlice {
        var raw by remember { mutableStateOf(Nav()) }

        // Exactly what the correction rule may ask about: every open document, and the two
        // Browser ids of the position being rendered. Watching more would cost queries for
        // rows nothing decides on; watching less would let a deletion go unnoticed.
        val watched = remember(raw) {
            (raw.open.map { it.nodeId } + raw.here.path + listOfNotNull(raw.here.itemId))
                .distinct()
        }
        val nav = raw.corrected(nodes.presenceOf(watched))

        return NavSlice(nav) { event ->
            raw = when (event) {
                is NavEvent.GoTo -> nav.goTo(event.destination)
                is NavEvent.GoToBrowser -> nav.goToBrowser(event.path, event.itemId)
                is NavEvent.OpenDoc -> nav.openDoc(event.key)
                is NavEvent.CloseDoc -> nav.closeDoc(event.key)
                is NavEvent.SelectFolder -> nav.selectFolder(event.column, event.folderId)
                is NavEvent.SelectItem -> nav.selectItem(event.itemId)
                NavEvent.Back -> nav.step(-1)
                NavEvent.Forward -> nav.step(1)
            }
        }
    }
}
