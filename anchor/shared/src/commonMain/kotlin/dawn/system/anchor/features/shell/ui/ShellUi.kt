package dawn.system.anchor.features.shell.ui

import androidx.compose.animation.core.CubicBezierEasing
import androidx.compose.animation.core.animateDpAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.requiredWidth
import androidx.compose.foundation.layout.width
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import dawn.system.anchor.features.shell.ShellScreen
import dawn.system.anchor.services.design.AladinMetrics
import dawn.system.anchor.services.design.AnchorTheme

/**
 * The shell's frame: sidebar · content (toolbar, tab strip, surface) · copilot. With the
 * sidebar away, the toolbar's pane toggle is the one affordance — there is no second chrome.
 *
 * Stateless. It renders [ShellScreen.State] and calls the handlers the slices carry, which is
 * what lets the same layout be driven by a test, a preview, or the real presenter.
 *
 * Every column runs the full height so its background reaches the physical edges, under the
 * status bar and the home indicator; only *content* is inset. A chrome-coloured band across
 * the top would read as a title bar, and this design does not have one.
 */
@Composable
fun ShellUi(state: ShellScreen.State, modifier: Modifier = Modifier) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    Box(modifier.fillMaxSize().background(c.bg)) {
        Row(Modifier.fillMaxSize()) {
            // Collapsing animates the wrapper's width while the panel keeps its natural width,
            // so nothing reflows mid-transition.
            val sidebarWidth by animateDpAsState(
                targetValue = if (state.chrome.sidebarOpen) m.sidebarWidth else 0.dp,
                animationSpec = tween(
                    durationMillis = AladinMetrics.COLUMN_COLLAPSE_MILLIS,
                    easing = CubicBezierEasing(0.2f, 0.8f, 0.2f, 1f),
                ),
                label = "sidebar-collapse",
            )
            Box(Modifier.width(sidebarWidth).fillMaxHeight()) {
                Sidebar(state, Modifier.requiredWidth(m.sidebarWidth))
            }
            if (state.chrome.sidebarOpen) VerticalHairline()

            Column(Modifier.weight(1f).fillMaxHeight()) {
                ContentToolbar(state)
                // The open documents, as tabs. Only while something is open: with nothing
                // open there is nothing to switch between, and the band would be dead space.
                if (state.open.isNotEmpty()) TabStrip(state)
                SurfaceHost(state, Modifier.weight(1f).fillMaxWidth())
            }

            val copilotWidth by animateDpAsState(
                targetValue = if (state.chrome.copilotOpen) m.copilotPaneWidth else 0.dp,
                animationSpec = tween(
                    durationMillis = AladinMetrics.COLUMN_COLLAPSE_MILLIS,
                    easing = CubicBezierEasing(0.2f, 0.8f, 0.2f, 1f),
                ),
                label = "copilot-collapse",
            )
            if (copilotWidth > 0.dp) {
                VerticalHairline()
                Box(Modifier.width(copilotWidth).fillMaxHeight()) {
                    CopilotSidebar(state, Modifier.requiredWidth(m.copilotPaneWidth))
                }
            }
        }

        // Overlays hang HERE, at the shell's own box, never inside the pager: a parked page is
        // placed two window-widths away and would take its overlay with it.
        BrowserDropdown(state)
        BrowserMenu(state)
        OpenDropdown(state)
        FilterPopover(state)
        CreateMenu(state)
        WriteSheets(state)
    }
}

/** A one-pixel row rule — the horizontal twin, for stacked zones. */
@Composable
internal fun HorizontalHairline() {
    Box(Modifier.fillMaxWidth().height(1.dp).background(AnchorTheme.colors.line2))
}

/** A one-pixel column rule. The shell's only divider. */
@Composable
internal fun VerticalHairline() {
    Box(Modifier.fillMaxHeight().width(1.dp).background(AnchorTheme.colors.line))
}
