package dawn.system.anchor.features.shell.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import dawn.system.anchor.domain.ResearchView
import dawn.system.anchor.domain.TabKey
import dawn.system.anchor.features.shell.ShellScreen
import dawn.system.anchor.features.shell.state.NavEvent
import dawn.system.anchor.features.shell.state.TreeEvent
import dawn.system.anchor.features.shell.state.TreeRow
import dawn.system.anchor.services.design.AnchorShape
import dawn.system.anchor.services.design.AnchorTheme
import dawn.system.anchor.services.design.ChevronDirection
import dawn.system.anchor.services.design.ChevronIcon
import dawn.system.anchor.services.design.MaximizeIcon
import dawn.system.anchor.services.design.SectionLabelStyle
import dawn.system.anchor.services.design.artifactKindIcon

/**
 * The sidebar's folder tree — the everyday browser.
 *
 * One tap does the obvious thing: an artifact opens, a folder expands or collapses. The tree
 * is where you *live*; the Browser destination remains the wide, columned power view for
 * organising. It is the only zone of the sidebar that scrolls.
 */
@Composable
internal fun TreeZone(state: ShellScreen.State, modifier: Modifier = Modifier) {
    val c = AnchorTheme.colors

    Column(modifier.fillMaxWidth()) {
        Row(
            Modifier.fillMaxWidth().padding(start = 20.dp, end = 10.dp, top = 14.dp, bottom = 4.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text("WORKSPACE", style = SectionLabelStyle, color = c.ink4)
        }

        if (state.tree.rows.isEmpty()) {
            Text(
                if (state.chrome.filter.isNarrowing) "Nothing matches the filter." else "Nothing here yet.",
                style = MaterialTheme.typography.bodyMedium,
                color = c.ink4,
                modifier = Modifier.padding(horizontal = 20.dp, vertical = 22.dp),
            )
        } else {
            LazyColumn(
                Modifier.fillMaxWidth(),
                contentPadding = PaddingValues(start = 10.dp, end = 10.dp, bottom = 8.dp),
            ) {
                items(state.tree.rows, key = { it.id }) { row ->
                    TreeRowView(row, state)
                }
            }
        }
    }
}

@Composable
private fun TreeRowView(row: TreeRow, state: ShellScreen.State) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    val openOverview = { state.nav.handle(NavEvent.OpenDoc(TabKey.Research(row.id, ResearchView.Overview))) }
    val onClick = {
        if (row.isContainer) {
            state.tree.handle(TreeEvent.ToggleFolder(row.id))
        } else {
            // Opening a document also makes its folder the place new things land.
            state.tree.handle(TreeEvent.FocusFolder(row.parentId))
            state.nav.handle(NavEvent.OpenDoc(TabKey.Artifact(row.id)))
        }
    }

    Row(
        Modifier
            .fillMaxWidth()
            .height(m.treeRow)
            .clip(AnchorShape.treeRow)
            .background(if (row.active) c.sel else c.panel.copy(alpha = 0f))
            .combinedClickable(onClick = onClick, onLongClick = if (row.isResearch) openOverview else null)
            .padding(start = m.treeIndentBase + m.treeIndent * row.depth, end = 4.dp),
        horizontalArrangement = Arrangement.spacedBy(10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        // The disclosure. Leaves keep the slot, so glyphs line up down the tree.
        if (row.isContainer) {
            ChevronIcon(
                tint = if (row.hasChildren) c.ink4 else c.ink4.copy(alpha = 0.45f),
                size = 14.dp,
                direction = if (row.expanded) ChevronDirection.Down else ChevronDirection.Right,
            )
        } else {
            Spacer(Modifier.width(14.dp))
        }
        // Amber is spent on the Copilot hero in the resting sidebar; research is the one
        // exception, because its folder IS a document you can open.
        artifactKindIcon(
            artifactType = row.kind?.wire,
            isContainer = row.isContainer,
            tint = when {
                row.isResearch -> c.amber
                row.active -> c.ink
                else -> c.ink3
            },
            size = 18.dp,
        )
        Text(
            row.title,
            style = MaterialTheme.typography.bodyMedium,
            fontWeight = FontWeight.Medium,
            color = if (row.active) c.ink else c.ink2,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.weight(1f),
        )
        row.count?.takeIf { it > 0 }?.let { Text("$it", style = SectionLabelStyle, color = c.ink4) }
        if (row.isResearch) {
            // Its Overview, in a 28pt glyph inside the 44pt row — the same allowance the Open
            // row's close control had.
            IconButton(size = 28.dp, onClick = openOverview) {
                MaximizeIcon(tint = c.amber, size = 14.dp)
            }
        }
    }
    Spacer(Modifier.size(2.dp))
}
