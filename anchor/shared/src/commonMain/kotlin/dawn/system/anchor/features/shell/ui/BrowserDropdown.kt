package dawn.system.anchor.features.shell.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import dawn.system.anchor.domain.Surface
import dawn.system.anchor.features.shell.ShellScreen
import dawn.system.anchor.features.shell.state.ChromeEvent
import dawn.system.anchor.features.shell.state.NavEvent
import dawn.system.anchor.services.design.AnchorTheme
import dawn.system.anchor.services.design.CloseIcon
import dawn.system.anchor.services.design.ColumnsIcon
import dawn.system.anchor.services.design.MaximizeIcon
import dawn.system.anchor.services.design.PinIcon
import dawn.system.anchor.services.design.SectionLabelStyle

/**
 * The browser, over your content instead of instead of it.
 *
 * This is the v2 handoff's centrepiece and the reason the browser stopped being a destination:
 * picking a resource is constant and mid-task, so it must not cost a context switch. The
 * document behind stays exactly where it was, and dismissing lands you back in it.
 *
 * **Pinned means "survives opening an item"** — pin once, then work through several documents
 * in sequence without reopening. It deliberately does NOT mean "ignores a tap outside": the
 * design suppressed the catcher entirely when pinned, so the content behind stayed
 * interactive, but tap-outside-to-dismiss is the stronger convention and the owner's call.
 * The cost is stated rather than hidden: a pinned dropdown now also swallows taps meant for
 * the document behind it.
 */
@Composable
internal fun BoxScope.BrowserDropdown(state: ShellScreen.State) {
    val m = AnchorTheme.metrics
    val c = AnchorTheme.colors
    val pinned = state.chrome.browserPinned

    AnchoredOverlay(
        visible = state.chrome.browserOpen,
        anchor = if (state.chrome.sidebarOpen) {
            OverlayAnchor.Below(left = 14.dp, top = m.overlayTop)
        } else {
            OverlayAnchor.AboveDock(bottom = 96.dp)
        },
        onDismiss = { state.chrome.handle(ChromeEvent.CloseBrowser) },
        borderColor = if (pinned) c.amberLine else c.line,
        elevation = if (pinned) 16.dp else 24.dp,
        modifier = Modifier.width(m.browserDropdownWidth).height(m.browserDropdownHeight),
    ) {
        Column(Modifier.fillMaxSize()) {
            Header(state, pinned)
            HorizontalHairline()
            Box(Modifier.fillMaxWidth().weight(1f)) {
                BrowserRail(
                    slice = state.browser,
                    columnWidth = m.browserDropdownColumn,
                    // Passed so reopening on an already-deep path scrolls to the deepest
                    // columns rather than the shallowest.
                    visible = state.chrome.browserOpen,
                    onNav = { event ->
                        state.nav.handle(event)
                        // Opening an item dismisses — unless pinned, where picks land behind
                        // it. The coupling lives here rather than in either producer, because
                        // it is a fact about this overlay and not about navigation or chrome
                        // on their own.
                        if (event is NavEvent.OpenDoc && !pinned) {
                            state.chrome.handle(ChromeEvent.CloseBrowser)
                        }
                    },
                )
            }
            HorizontalHairline()
            BrowserFooter(
                slice = state.browser,
                depthHint = depthHint(state.browser.columns.size),
                onNav = { event ->
                    state.nav.handle(event)
                    if (event is NavEvent.OpenDoc && !pinned) {
                        state.chrome.handle(ChromeEvent.CloseBrowser)
                    }
                },
            )
        }
    }
}

/**
 * "depth 5 · scroll ⇢", and only once the rail is deeper than the three columns on screen.
 *
 * Without it, a rail scrolled to its end looks like a rail with nothing to its left — the one
 * thing horizontal columns cannot show about themselves.
 */
private fun depthHint(columns: Int): String? =
    if (columns > VISIBLE_COLUMNS) "DEPTH $columns · SCROLL" else null

private const val VISIBLE_COLUMNS = 3

/** Name, count, promote, pin, dismiss. */
@Composable
private fun Header(state: ShellScreen.State, pinned: Boolean) {
    val c = AnchorTheme.colors
    val rootCount = state.browser.columns.firstOrNull()?.rows?.size ?: 0

    Row(
        Modifier.fillMaxWidth().padding(start = 16.dp, end = 10.dp, top = 14.dp, bottom = 12.dp),
        horizontalArrangement = Arrangement.spacedBy(10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        ColumnsIcon(tint = c.ink3, size = 19.dp)
        Text(
            text = Surface.Browser.TITLE,
            style = MaterialTheme.typography.titleMedium,
            color = c.ink,
        )
        Text("$rootCount FOLDERS", style = SectionLabelStyle, color = c.ink4)
        Spacer(Modifier.weight(1f))

        // The discoverable path to a tab; long-press on the icon is the shortcut for people
        // who already know. Promoting closes this, because only one browser is ever live.
        IconButton(
            size = 40.dp,
            onClick = {
                state.chrome.handle(ChromeEvent.CloseBrowser)
                state.nav.handle(NavEvent.OpenBrowserTab)
            },
        ) {
            MaximizeIcon(tint = c.ink3, size = 16.dp)
        }

        // The pin is where the accent goes on this surface, and only when it is lit — the
        // design allows one amber element per surface and this spends it deliberately.
        IconButton(
            size = 40.dp,
            background = if (pinned) c.amberSoft else null,
            onClick = { state.chrome.handle(ChromeEvent.ToggleBrowserPinned) },
        ) {
            PinIcon(tint = if (pinned) c.amber else c.ink3, size = 17.dp)
        }
        IconButton(size = 40.dp, onClick = { state.chrome.handle(ChromeEvent.CloseBrowser) }) {
            CloseIcon(tint = c.ink3, size = 17.dp)
        }
    }
}
