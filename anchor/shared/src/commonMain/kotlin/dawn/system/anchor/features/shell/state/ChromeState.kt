package dawn.system.anchor.features.shell.state

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import com.slack.circuit.runtime.CircuitUiState

/** Panels and overlays the user toggles. Nothing outside the shell can tell they exist. */
sealed interface ChromeEvent {
    data object ToggleSidebar : ChromeEvent
    data object ToggleCopilot : ChromeEvent
    data object ToggleFilter : ChromeEvent
    data object ToggleSwitcher : ChromeEvent
}

/**
 * What the shell is *showing* — as opposed to what it is showing it *about*.
 *
 * A value, like every other slice: the producer owns the mutable pieces and hands out an
 * immutable snapshot each frame, so nothing downstream can reach in and change it.
 *
 * [dismissOverlays] is a command rather than an event, because it is not something the *user*
 * does — it is what navigation asks of the chrome when you move somewhere. Keeping it off the
 * event hierarchy leaves that hierarchy meaning "something the user did".
 */
data class ChromeSlice(
    val sidebarOpen: Boolean,
    val copilotOpen: Boolean,
    val filterOpen: Boolean,
    val switcherOpen: Boolean,
    val dismissOverlays: () -> Unit,
    val handle: (ChromeEvent) -> Unit,
) : CircuitUiState

/**
 * The shell's own chrome. No dependencies — none of this is in the store, and nothing outside
 * the shell can tell whether the filter popover is open.
 *
 * Four booleans, where the previous shell had seven. The rest went with the navigation stack:
 * there is no path popover because place lives in the breadcrumb, and no column-browser
 * overlay because the columns are a destination now.
 */
@Composable
fun rememberChromeState(): ChromeSlice {
    var sidebarOpen by remember { mutableStateOf(true) }
    var copilotOpen by remember { mutableStateOf(false) }
    var filterOpen by remember { mutableStateOf(false) }
    var switcherOpen by remember { mutableStateOf(false) }

    return ChromeSlice(
        sidebarOpen = sidebarOpen,
        copilotOpen = copilotOpen,
        filterOpen = filterOpen,
        switcherOpen = switcherOpen,
        // Overlays close when you go somewhere; the sidebar and Copilot are not overlays and
        // deliberately survive — collapsing the sidebar is a statement about the whole shell,
        // not about the surface you happen to be on.
        dismissOverlays = {
            filterOpen = false
            switcherOpen = false
        },
    ) { event ->
        // Exhaustive, with no `else`. That is what a sealed hierarchy buys: a new chrome event
        // fails to compile here instead of silently routing nowhere.
        when (event) {
            ChromeEvent.ToggleSidebar -> sidebarOpen = !sidebarOpen
            ChromeEvent.ToggleCopilot -> copilotOpen = !copilotOpen
            ChromeEvent.ToggleFilter -> filterOpen = !filterOpen
            ChromeEvent.ToggleSwitcher -> switcherOpen = !switcherOpen
        }
    }
}
