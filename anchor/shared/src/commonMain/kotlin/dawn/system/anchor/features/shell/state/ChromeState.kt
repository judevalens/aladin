package dawn.system.anchor.features.shell.state

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import com.slack.circuit.runtime.CircuitUiState
import dawn.system.anchor.features.shell.ShellScreen

/**
 * What the shell is *showing* — panels, popovers, sheets — as opposed to what it is showing
 * them *about*.
 *
 * A value, like every other slice: the producer below owns the mutable pieces and hands out
 * an immutable snapshot each frame, so nothing downstream can reach in and change it.
 *
 * [dismissOverlays] and [unpinHero] are commands rather than events because they are not
 * things the *user* does — they are what navigation asks of the chrome when you move. Keeping
 * them off the event hierarchy leaves that hierarchy meaning "something the user did".
 */
data class ChromeSlice(
    val sidebarOpen: Boolean,
    val pathOpen: Boolean,
    val switcherOpen: Boolean,
    val browseOpen: Boolean,
    val createOpen: Boolean,
    val copilotOpen: Boolean,
    val heroExpanded: Boolean,
    /** Everything that overlays the shell closes when you go somewhere. */
    val dismissOverlays: () -> Unit,
    /** Back at root the hero follows the level again rather than a stale pin. */
    val unpinHero: () -> Unit,
    val handle: (ShellScreen.Event.Chrome) -> Unit,
) : CircuitUiState

/**
 * The shell's own chrome. No dependencies — none of this is in the store, and nothing outside
 * the shell can tell whether the path popover is open.
 *
 * The hero is the one with a rule rather than a toggle: full at root, mini once you are in a
 * list, unless you have pinned it either way.
 */
@Composable
fun rememberChromeState(isRoot: Boolean): ChromeSlice {
    var sidebarOpen by remember { mutableStateOf(true) }
    var pathOpen by remember { mutableStateOf(false) }
    var switcherOpen by remember { mutableStateOf(false) }
    var browseOpen by remember { mutableStateOf(false) }
    var createOpen by remember { mutableStateOf(false) }
    var copilotOpen by remember { mutableStateOf(false) }
    // Null means "follow the level" rather than a pin either way.
    var heroPin by remember { mutableStateOf<Boolean?>(null) }

    val heroExpanded = heroPin ?: isRoot

    return ChromeSlice(
        sidebarOpen = sidebarOpen,
        pathOpen = pathOpen,
        switcherOpen = switcherOpen,
        browseOpen = browseOpen,
        createOpen = createOpen,
        copilotOpen = copilotOpen,
        heroExpanded = heroExpanded,
        dismissOverlays = {
            pathOpen = false
            switcherOpen = false
            browseOpen = false
            createOpen = false
        },
        unpinHero = { heroPin = null },
    ) { event ->
        // Exhaustive, with no `else`. That is what the nested event hierarchy buys: a new
        // chrome event fails to compile here instead of silently routing nowhere.
        when (event) {
            ShellScreen.Event.Chrome.ToggleSidebar -> sidebarOpen = !sidebarOpen
            ShellScreen.Event.Chrome.TogglePath -> pathOpen = !pathOpen
            ShellScreen.Event.Chrome.ToggleSwitcher -> switcherOpen = !switcherOpen
            ShellScreen.Event.Chrome.ToggleCreate -> createOpen = !createOpen
            ShellScreen.Event.Chrome.ToggleHero -> heroPin = !heroExpanded
            ShellScreen.Event.Chrome.OpenCopilot -> copilotOpen = true
            ShellScreen.Event.Chrome.CloseCopilot -> copilotOpen = false

            ShellScreen.Event.Chrome.ToggleBrowse -> {
                browseOpen = !browseOpen
                pathOpen = false
            }
        }
    }
}
