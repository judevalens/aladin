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
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import dawn.system.anchor.features.shell.ShellScreen
import dawn.system.anchor.features.shell.state.ChromeEvent
import dawn.system.anchor.features.shell.state.NavEvent
import dawn.system.anchor.features.shell.state.OpenRow
import dawn.system.anchor.features.shell.state.openEvent
import dawn.system.anchor.features.shell.state.rowKey
import dawn.system.anchor.services.design.AnchorShape
import dawn.system.anchor.services.design.AnchorTheme
import dawn.system.anchor.services.design.CloseIcon
import dawn.system.anchor.services.design.ColumnsIcon
import dawn.system.anchor.services.design.MetaStyle
import dawn.system.anchor.services.design.SectionLabelStyle
import dawn.system.anchor.services.design.artifactKindIcon

/**
 * The Open list as an overlay — the browser dropdown's smaller sibling, deliberately.
 *
 * **Recent first here, append order in the sidebar**, and that is not an inconsistency. The
 * sidebar list is a set of targets you reach for by position, so it must not reorder under a
 * finger; this is a switcher, where "the one I was just in" is the whole point and the design
 * labels it so. One value serves both: `Nav.open` is membership and append order, `Nav.mru` is
 * recency, and neither is derived from the other.
 */
@Composable
internal fun BoxScope.OpenDropdown(state: ShellScreen.State) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    // Recency, reconciled against what is actually open — a stale entry can order wrongly but
    // can never render a phantom.
    val rows = remember(state.open, state.nav.nav) {
        val byTab = state.open.associateBy { it.tab }
        state.nav.nav.switcherOrder().mapNotNull(byTab::get)
    }

    AnchoredOverlay(
        visible = state.chrome.switcherOpen,
        anchor = if (state.chrome.sidebarOpen) {
            OverlayAnchor.Below(left = 14.dp, top = m.openDropdownTop)
        } else {
            OverlayAnchor.AboveDock(bottom = 96.dp)
        },
        onDismiss = { state.chrome.handle(ChromeEvent.ToggleSwitcher) },
        modifier = Modifier.width(m.openDropdownWidth).heightIn(max = m.openDropdownMaxHeight),
    ) {
        Column(Modifier.fillMaxWidth()) {
            Row(
                Modifier.fillMaxWidth().padding(start = 14.dp, end = 14.dp, top = 11.dp, bottom = 8.dp),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text("OPEN", style = SectionLabelStyle, color = c.ink4)
                Text("${rows.size}", style = SectionLabelStyle, color = c.ink4)
                Spacer(Modifier.weight(1f))
                Text("RECENT FIRST", style = SectionLabelStyle, color = c.ink4)
            }
            HorizontalHairline()

            LazyColumn(Modifier.weight(1f, fill = false)) {
                items(rows, key = { it.tab.rowKey() }) { row ->
                    SwitcherRow(row, state)
                }
            }

            HorizontalHairline()
            Text(
                "TAP TO OPEN · × CLOSES",
                style = SectionLabelStyle,
                color = c.ink4,
                modifier = Modifier.padding(horizontal = 14.dp, vertical = 8.dp),
            )
        }
    }
}

@Composable
private fun SwitcherRow(row: OpenRow, state: ShellScreen.State) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    Row(
        Modifier
            .fillMaxWidth()
            .height(m.openDropdownRow)
            .padding(horizontal = 8.dp, vertical = 2.dp)
            .clip(AnchorShape.menuRow)
            .background(if (row.active) c.sel else c.card.copy(alpha = 0f))
            .clickable {
                state.chrome.handle(ChromeEvent.ToggleSwitcher)
                state.nav.handle(row.tab.openEvent())
            }
            .padding(start = 11.dp, end = 4.dp),
        horizontalArrangement = Arrangement.spacedBy(11.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        // The browser tab has no kind, being a place rather than a row.
        if (row.kind == null && row.folder == null) {
            ColumnsIcon(tint = if (row.active) c.amber else c.ink3, size = 18.dp)
        } else {
            artifactKindIcon(
                artifactType = row.kind?.wire,
                isContainer = false,
                tint = if (row.active) c.amber else c.ink3,
                size = 18.dp,
            )
        }

        Column(Modifier.weight(1f)) {
            Text(
                row.title,
                style = MaterialTheme.typography.bodyMedium,
                color = if (row.active) c.ink else c.ink2,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            row.folder?.let {
                Text(it, style = MetaStyle, color = c.ink4, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
        }

        IconButton(size = 32.dp, onClick = { state.nav.handle(NavEvent.Close(row.tab)) }) {
            CloseIcon(tint = c.ink4, size = 14.dp)
        }
    }
}
