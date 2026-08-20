package dawn.system.anchor.features.shell.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.unit.dp
import dawn.system.anchor.features.shell.ShellScreen
import dawn.system.anchor.features.shell.state.ChromeEvent
import dawn.system.anchor.features.shell.state.NavEvent
import dawn.system.anchor.services.design.AnchorShape
import dawn.system.anchor.services.design.AnchorTheme
import dawn.system.anchor.services.design.ColumnsIcon
import dawn.system.anchor.services.design.MaximizeIcon
import dawn.system.anchor.services.design.SectionLabelStyle

/**
 * Long-press on the browser icon: **a menu, not a direct action**.
 *
 * A long press that promoted straight to a tab would be a gesture you could only discover by
 * firing it accidentally. A menu is cancellable, and it does the teaching — the footer line
 * says where a tab lives and that it closes like anything else, which is the one thing about
 * this model that is not obvious from looking at it.
 *
 * The maximize glyph in the dropdown header reaches the same promotion. This is the shortcut
 * for people who already know; that is the discoverable path.
 */
@Composable
internal fun BoxScope.BrowserMenu(state: ShellScreen.State) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    AnchoredOverlay(
        visible = state.chrome.browserMenuOpen,
        anchor = if (state.chrome.sidebarOpen) {
            OverlayAnchor.Below(left = 58.dp, top = m.overlayTop + 4.dp)
        } else {
            OverlayAnchor.AboveDock(bottom = 96.dp)
        },
        onDismiss = { state.chrome.handle(ChromeEvent.CloseBrowserMenu) },
        shape = AnchorShape.card,
        modifier = Modifier.width(m.contextMenuWidth),
    ) {
        Column(Modifier.fillMaxWidth().padding(6.dp)) {
            MenuRow(
                label = "Open browser here",
                icon = { ColumnsIcon(tint = c.ink3, size = 18.dp) },
                onClick = {
                    state.chrome.handle(ChromeEvent.CloseBrowserMenu)
                    state.chrome.handle(ChromeEvent.ToggleBrowser)
                },
            )
            MenuRow(
                label = "Open in a tab",
                icon = { MaximizeIcon(tint = c.ink3, size = 16.dp) },
                onClick = {
                    state.chrome.handle(ChromeEvent.CloseBrowserMenu)
                    state.nav.handle(NavEvent.OpenBrowserTab)
                },
            )
            HorizontalHairline()
            Text(
                "A TAB LIVES IN OPEN · CLOSABLE",
                style = SectionLabelStyle,
                color = c.ink4,
                modifier = Modifier.padding(start = 11.dp, top = 10.dp, bottom = 6.dp),
            )
        }
    }
}

@Composable
private fun MenuRow(label: String, icon: @Composable () -> Unit, onClick: () -> Unit) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    Row(
        Modifier
            .fillMaxWidth()
            .height(m.touchTarget)
            .clip(AnchorShape.control)
            .clickable(onClick = onClick)
            .padding(horizontal = 11.dp),
        horizontalArrangement = Arrangement.spacedBy(11.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        icon()
        Text(label, style = MaterialTheme.typography.bodyMedium, color = c.ink)
        Spacer(Modifier.weight(1f))
    }
}
