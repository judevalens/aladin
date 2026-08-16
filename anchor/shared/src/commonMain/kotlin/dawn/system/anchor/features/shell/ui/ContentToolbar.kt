package dawn.system.anchor.features.shell.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import dawn.system.anchor.features.shell.ShellScreen
import dawn.system.anchor.features.shell.state.ChromeEvent
import dawn.system.anchor.features.shell.state.NavEvent
import dawn.system.anchor.services.design.AnchorShape
import dawn.system.anchor.services.design.AnchorTheme
import dawn.system.anchor.services.design.ChevronDirection
import dawn.system.anchor.services.design.ChevronIcon
import dawn.system.anchor.services.design.MoreIcon
import dawn.system.anchor.services.design.SidebarIcon

/**
 * The content toolbar: back/forward, the breadcrumb, and everything else behind ⋯.
 *
 * The breadcrumb is the shell's whole sense of place — always visible, never behind a tap.
 * That is the fix for the previous pass, where "where am I" lived in a popover and the
 * sidebar's meaning changed under you on every push.
 */
@Composable
internal fun ContentToolbar(state: ShellScreen.State, modifier: Modifier = Modifier) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    Row(
        modifier
            .fillMaxWidth()
            .height(m.toolbarHeight)
            .background(c.panel)
            .padding(start = 16.dp, end = 16.dp, top = m.toolbarSafeInset),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(2.dp),
    ) {
        // Only while the sidebar is away: it has its own hide button, and showing both would
        // put the same affordance on screen twice.
        if (!state.chrome.sidebarOpen) {
            IconButton(size = m.touchTarget, onClick = { state.chrome.handle(ChromeEvent.ToggleSidebar) }) {
                SidebarIcon(tint = c.ink3, size = 19.dp)
            }
        }

        NavArrow(ChevronDirection.Left, state.nav.nav.canBack) { state.nav.handle(NavEvent.Back) }
        NavArrow(ChevronDirection.Right, state.nav.nav.canForward) { state.nav.handle(NavEvent.Forward) }
        Spacer(Modifier.padding(start = 6.dp))

        state.crumbs.forEachIndexed { index, crumb ->
            val last = index == state.crumbs.lastIndex
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    text = crumb.label,
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = if (last) FontWeight.SemiBold else FontWeight.Medium,
                    color = if (last) c.ink else c.ink3,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier
                        .widthIn(max = 340.dp)
                        .clip(AnchorShape.chip)
                        .then(
                            crumb.target?.let { target ->
                                Modifier.clickable { state.nav.handle(target) }
                            } ?: Modifier,
                        )
                        .padding(horizontal = 9.dp, vertical = 7.dp),
                )
                if (!last) {
                    ChevronIcon(tint = c.ink4, size = 13.dp, direction = ChevronDirection.Right)
                }
            }
        }

        Spacer(Modifier.weight(1f))

        IconButton(size = m.touchTarget) { MoreIcon(tint = c.ink3, size = 19.dp) }
    }
}

/** Dimmed rather than hidden at the ends of the trail, so the affordance never moves. */
@Composable
private fun NavArrow(direction: ChevronDirection, enabled: Boolean, onClick: () -> Unit) {
    val c = AnchorTheme.colors
    Box(Modifier.height(AnchorTheme.metrics.touchTarget)) {
        IconButton(size = AnchorTheme.metrics.touchTarget, enabled = enabled, onClick = onClick) {
            ChevronIcon(tint = if (enabled) c.ink2 else c.ink4, size = 20.dp, direction = direction)
        }
    }
}
