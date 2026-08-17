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

    /** The browser icon. Opening the dropdown is chrome, never navigation. */
    data object ToggleBrowser : ChromeEvent
    data object CloseBrowser : ChromeEvent

    /** Pin: the dropdown stays through item opens. */
    data object ToggleBrowserPinned : ChromeEvent

    /** Long-press on the browser icon: a menu, never a direct action. */
    data object OpenBrowserMenu : ChromeEvent
    data object CloseBrowserMenu : ChromeEvent
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
    val browserOpen: Boolean,
    /**
     * The long-press menu. **A menu rather than a direct promotion**, because a long press that
     * did something immediately would be a gesture you can only learn by accidentally firing
     * it — and this one teaches that a tab exists.
     */
    val browserMenuOpen: Boolean,
    /**
     * Pinned: the dropdown survives opening an item, and renders no click-catcher at all, so
     * the document behind stays interactive. A working preference rather than a transient —
     * the design has it persist across relaunches, which is still to build.
     */
    val browserPinned: Boolean,
    val dismissOverlays: () -> Unit,
    val handle: (ChromeEvent) -> Unit,
) : CircuitUiState

/**
 * The shell's own chrome. No dependencies — none of this is in the store, and nothing outside
 * the shell can tell whether the filter popover is open.
 *
 * Six booleans, where the previous shell had seven — and unlike that one, none of them is a
 * second opinion about where you are. There is no path popover, because place lives in the
 * breadcrumb.
 */
@Composable
fun rememberChromeState(): ChromeSlice {
    var sidebarOpen by remember { mutableStateOf(true) }
    var copilotOpen by remember { mutableStateOf(false) }
    var filterOpen by remember { mutableStateOf(false) }
    var switcherOpen by remember { mutableStateOf(false) }
    var browserOpen by remember { mutableStateOf(false) }
    var browserPinned by remember { mutableStateOf(false) }
    var browserMenuOpen by remember { mutableStateOf(false) }

    return ChromeSlice(
        sidebarOpen = sidebarOpen,
        copilotOpen = copilotOpen,
        filterOpen = filterOpen,
        switcherOpen = switcherOpen,
        browserOpen = browserOpen,
        browserMenuOpen = browserMenuOpen,
        browserPinned = browserPinned,
        // Overlays close when you go somewhere; the sidebar and Copilot are not overlays and
        // deliberately survive — collapsing the sidebar is a statement about the whole shell,
        // not about the surface you happen to be on.
        dismissOverlays = {
            filterOpen = false
            switcherOpen = false
            // The pinned browser is the exception, and the whole point of pinning: picks land
            // behind it and it stays put.
            if (!browserPinned) browserOpen = false
            browserMenuOpen = false
        },
    ) { event ->
        // Exhaustive, with no `else`. That is what a sealed hierarchy buys: a new chrome event
        // fails to compile here instead of silently routing nowhere.
        when (event) {
            ChromeEvent.ToggleSidebar -> sidebarOpen = !sidebarOpen
            ChromeEvent.ToggleCopilot -> copilotOpen = !copilotOpen
            ChromeEvent.ToggleFilter -> filterOpen = !filterOpen
            ChromeEvent.ToggleSwitcher -> switcherOpen = !switcherOpen
            ChromeEvent.ToggleBrowser -> browserOpen = !browserOpen
            ChromeEvent.CloseBrowser -> browserOpen = false
            ChromeEvent.ToggleBrowserPinned -> browserPinned = !browserPinned
            ChromeEvent.OpenBrowserMenu -> {
                browserMenuOpen = true
                // Never both: the menu is a choice ABOUT the browser, not a thing beside it.
                browserOpen = false
            }
            ChromeEvent.CloseBrowserMenu -> browserMenuOpen = false
        }
    }
}
