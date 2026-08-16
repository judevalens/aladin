package dawn.system.anchor.features.shell.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import dawn.system.anchor.domain.FolderPurpose
import dawn.system.anchor.features.shell.state.BrowserRow
import dawn.system.anchor.features.shell.state.BrowserSlice
import dawn.system.anchor.features.shell.state.NavEvent
import dawn.system.anchor.services.design.AnchorShape
import dawn.system.anchor.services.design.AnchorTheme
import dawn.system.anchor.services.design.ChevronDirection
import dawn.system.anchor.services.design.ChevronIcon
import dawn.system.anchor.services.design.SectionLabelStyle
import dawn.system.anchor.services.design.artifactKindIcon

/**
 * The Browser: **Miller columns**, one per level, scrolling horizontally.
 *
 * Not a fixed three-pane layout. Column 0 lists the roots, each subsequent column lists the
 * children of the folder selected in the one before it, and the strip grows as deep as you
 * descend. Picking a folder in a column you had already passed **discards every column to its
 * right** — those columns described a path you are no longer on, which is what the columns
 * mean rather than a case to handle.
 */
@Composable
internal fun BrowserSurface(
    slice: BrowserSlice,
    onNav: (NavEvent) -> Unit,
    modifier: Modifier = Modifier,
) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics
    val scroll = rememberScrollState()

    // Descending should reveal what you descended into, not leave it off-screen to the right.
    LaunchedEffect(slice.columns.size) { scroll.animateScrollTo(scroll.maxValue) }

    Row(modifier.fillMaxSize().horizontalScroll(scroll)) {
        slice.columns.forEachIndexed { depth, column ->
            Column(
                Modifier
                    .width(if (depth == 0) m.browserFolderColumn else m.browserItemColumn)
                    .fillMaxHeight(),
            ) {
                Row(
                    Modifier.fillMaxWidth().padding(start = 18.dp, end = 18.dp, top = 16.dp, bottom = 8.dp),
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    Text(column.title.uppercase(), style = SectionLabelStyle, color = c.ink4)
                    Text("${column.rows.size}", style = SectionLabelStyle, color = c.ink4)
                }

                if (column.rows.isEmpty()) {
                    Text(
                        "Nothing here matches the filter.",
                        style = MaterialTheme.typography.bodyMedium,
                        color = c.ink3,
                        modifier = Modifier.padding(horizontal = 18.dp, vertical = 24.dp),
                    )
                } else {
                    LazyColumn(contentPadding = PaddingValues(horizontal = 10.dp, vertical = 2.dp)) {
                        items(column.rows, key = { it.id }) { row ->
                            ColumnRow(row) {
                                if (row.isContainer) onNav(NavEvent.SelectFolder(depth, row.id))
                                else onNav(NavEvent.SelectItem(row.id))
                            }
                        }
                    }
                }
            }
            VerticalHairline()
        }

        DetailColumn(slice, onNav, Modifier.width(m.browserItemColumn).fillMaxHeight())
    }
}

@Composable
private fun ColumnRow(row: BrowserRow, onClick: () -> Unit) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    Row(
        Modifier
            .fillMaxWidth()
            .height(m.browserRow)
            .clip(AnchorShape.row)
            .background(if (row.selected) c.sel else c.bg.copy(alpha = 0f))
            .clickable(onClick = onClick)
            .padding(horizontal = 12.dp),
        horizontalArrangement = Arrangement.spacedBy(12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        artifactKindIcon(
            artifactType = row.kind?.wire,
            isContainer = row.isContainer,
            tint = if (row.isContainer) c.amber else c.ink3,
            size = 17.dp,
        )
        Text(
            row.title,
            style = MaterialTheme.typography.bodyMedium,
            color = if (row.selected) c.ink else c.ink2,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.weight(1f),
        )
        // Only a purpose the server can actually vouch for gets a chip. `Learning` has no
        // representation server-side, so it never appears rather than being guessed at.
        row.purpose?.takeIf { it == FolderPurpose.Research }?.let {
            Text(
                "RESEARCH",
                style = SectionLabelStyle,
                color = c.amber,
                modifier = Modifier
                    .clip(AnchorShape.chip)
                    .background(c.amberSoft)
                    .padding(horizontal = 6.dp, vertical = 3.dp),
            )
        }
        if (row.isContainer) {
            ChevronIcon(tint = c.ink4, size = 14.dp, direction = ChevronDirection.Right)
        }
    }
}

/**
 * The detail pane — what one selected item is, and the action row that opens it.
 *
 * Opening from here leaves the Browser lit and its columns selected, which is what makes the
 * breadcrumb's `Browser` jump meaningful afterwards.
 */
@Composable
private fun DetailColumn(slice: BrowserSlice, onNav: (NavEvent) -> Unit, modifier: Modifier = Modifier) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics
    val detail = slice.detail

    if (detail == null) {
        Box(modifier.padding(horizontal = 22.dp), contentAlignment = Alignment.Center) {
            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                Text(
                    "Select an item to preview it.",
                    style = MaterialTheme.typography.bodyLarge,
                    color = c.ink3,
                )
                Spacer(Modifier.height(8.dp))
                Text(
                    "OPENING A DOCUMENT LEAVES BROWSER LIT",
                    style = SectionLabelStyle,
                    color = c.ink4,
                )
            }
        }
        return
    }

    Column(modifier.padding(horizontal = 22.dp, vertical = 16.dp)) {
        Text(
            detail.kindLabel,
            style = SectionLabelStyle,
            color = c.ink2,
            modifier = Modifier
                .clip(AnchorShape.chip)
                .background(c.sel)
                .padding(horizontal = 7.dp, vertical = 4.dp),
        )
        Spacer(Modifier.height(10.dp))
        Text(detail.title, style = MaterialTheme.typography.headlineSmall, color = c.ink)
        Spacer(Modifier.height(16.dp))

        detail.openKey?.let { key ->
            Row(
                Modifier
                    .height(m.detailAction)
                    .clip(AnchorShape.row)
                    .background(c.amber)
                    .clickable { onNav(NavEvent.OpenDoc(key)) }
                    .padding(horizontal = 20.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text("Open", style = MaterialTheme.typography.titleMedium, color = c.onAmber)
            }
        }
    }
}
