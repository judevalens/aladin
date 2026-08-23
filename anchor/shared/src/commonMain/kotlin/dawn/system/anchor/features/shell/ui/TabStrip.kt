package dawn.system.anchor.features.shell.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import dawn.system.anchor.domain.OpenTab
import dawn.system.anchor.domain.TabKey
import dawn.system.anchor.features.shell.ShellScreen
import dawn.system.anchor.features.shell.state.ChromeEvent
import dawn.system.anchor.features.shell.state.NavEvent
import dawn.system.anchor.features.shell.state.OpenRow
import dawn.system.anchor.features.shell.state.openEvent
import dawn.system.anchor.features.shell.state.rowKey
import dawn.system.anchor.services.design.AnchorShape
import dawn.system.anchor.services.design.AnchorTheme
import dawn.system.anchor.services.design.ChevronDirection
import dawn.system.anchor.services.design.ChevronIcon
import dawn.system.anchor.services.design.CloseIcon
import dawn.system.anchor.services.design.ColumnsIcon
import dawn.system.anchor.services.design.artifactKindIcon

/**
 * The open documents, as a row of tabs over the content.
 *
 * **Append order, the stable one.** A tab is a target you reach for by position; recency
 * would reorder the row under the reader's finger. Recency belongs to the switcher this
 * strip's trailing button opens.
 *
 * The active tab takes the content's own background and runs into it with no hairline
 * between — the tab is the document's handle, not a label above it.
 */
@Composable
internal fun TabStrip(state: ShellScreen.State, modifier: Modifier = Modifier) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    // DERIVED, never stored: the active tab's index is a fact about the list, and scrolling
    // to it only when it leaves the viewport means the strip never fights a finger.
    val listState = rememberLazyListState()
    val activeIndex = state.open.indexOfFirst { it.active }
    LaunchedEffect(activeIndex) {
        if (activeIndex < 0) return@LaunchedEffect
        val info = listState.layoutInfo
        val visible = info.visibleItemsInfo.firstOrNull { it.index == activeIndex }
        val fullyVisible = visible != null &&
            visible.offset >= info.viewportStartOffset &&
            visible.offset + visible.size <= info.viewportEndOffset
        if (!fullyVisible) listState.animateScrollToItem(activeIndex)
    }

    Row(
        modifier
            .fillMaxWidth()
            .height(m.tabStripHeight)
            .background(c.panel)
            .padding(horizontal = 10.dp, vertical = 4.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        LazyRow(
            state = listState,
            modifier = Modifier.weight(1f),
            horizontalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            items(state.open, key = { it.tab.rowKey() }) { row -> TabView(row, state) }
        }
        // The switcher: recency, for when the strip is longer than the screen.
        IconButton(size = m.touchTarget, onClick = { state.chrome.handle(ChromeEvent.ToggleSwitcher) }) {
            ChevronIcon(tint = c.ink3, size = 16.dp, direction = ChevronDirection.Down)
        }
    }
}

@Composable
private fun TabView(row: OpenRow, state: ShellScreen.State) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    Row(
        Modifier
            .height(m.tabHeight)
            .widthIn(max = m.tabMaxWidth)
            .clip(AnchorShape.tab)
            .background(if (row.active) c.bg else c.panel)
            .clickable { state.nav.handle(row.tab.openEvent()) }
            .padding(start = 12.dp, end = 4.dp),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        val tint = if (row.active) c.ink else c.ink3
        when (val tab = row.tab) {
            OpenTab.Browser -> ColumnsIcon(tint = tint, size = 18.dp)
            is OpenTab.Doc -> when (tab.key) {
                // A research folder's Overview: the folder glyph, in the research amber.
                is TabKey.Research -> artifactKindIcon(null, isContainer = true, tint = c.amber, size = 18.dp)
                is TabKey.Artifact -> artifactKindIcon(row.kind?.wire, isContainer = false, tint = tint, size = 18.dp)
            }
        }
        Text(
            row.title,
            style = MaterialTheme.typography.bodyMedium,
            fontWeight = FontWeight.Medium,
            color = if (row.active) c.ink else c.ink3,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.weight(1f, fill = false),
        )
        // 28pt glyph inside a 44pt row is fine; a 28pt *button* would not be.
        IconButton(size = 28.dp, onClick = { state.nav.handle(NavEvent.Close(row.tab)) }) {
            CloseIcon(tint = c.ink4, size = 14.dp)
        }
    }
}
