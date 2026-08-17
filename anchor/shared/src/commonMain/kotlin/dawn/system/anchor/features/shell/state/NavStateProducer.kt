package dawn.system.anchor.features.shell.state

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import com.slack.circuit.runtime.CircuitUiState
import dawn.system.anchor.domain.Destination
import dawn.system.anchor.domain.Nav
import dawn.system.anchor.domain.OpenTab
import dawn.system.anchor.domain.TabKey
import dawn.system.anchor.services.data.NodeStore

/** Something the user did to move. Nothing here is a command the shell issues to itself. */
sealed interface NavEvent {
    data class GoTo(val destination: Destination) : NavEvent

    /** A breadcrumb jump into the columns. Shows the browser; does not open a tab. */
    data class GoToBrowser(val path: List<String>) : NavEvent
    data class OpenDoc(val key: TabKey) : NavEvent

    /** Promote the browser to a tab, or return to the one already open. */
    data object OpenBrowserTab : NavEvent
    data class Close(val tab: OpenTab) : NavEvent

    /** Pick a row in a column — folder or leaf, the same move. */
    data class Select(val column: Int, val id: String) : NavEvent
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

        // Exactly what the correction rule may ask about: every open DOCUMENT, and the path
        // of the position being rendered. Watching more would cost queries for rows nothing
        // decides on; watching less would let a deletion go unnoticed.
        //
        // The browser tab is not watched because it names no row — see `Nav.corrected`, where
        // asking about it anyway is the mistake that would close it.
        val watched = remember(raw) {
            (raw.open.filterIsInstance<OpenTab.Doc>().map { it.key.nodeId } + raw.here.path)
                .distinct()
        }
        val nav = raw.corrected(nodes.presenceOf(watched))

        return NavSlice(nav) { event ->
            raw = when (event) {
                is NavEvent.GoTo -> nav.goTo(event.destination)
                is NavEvent.GoToBrowser -> nav.goToBrowser(event.path)
                is NavEvent.OpenDoc -> nav.openDoc(event.key)
                NavEvent.OpenBrowserTab -> nav.openBrowserTab()
                is NavEvent.Close -> nav.close(event.tab)
                is NavEvent.Select -> nav.select(event.column, event.id)
                NavEvent.Back -> nav.step(-1)
                NavEvent.Forward -> nav.step(1)
            }
        }
    }
}
