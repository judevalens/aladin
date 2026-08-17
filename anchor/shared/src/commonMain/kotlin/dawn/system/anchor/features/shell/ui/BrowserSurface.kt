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
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.flow.first
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
 * descend. Picking anything in a column you had already passed **discards every column to its
 * right** — those columns described a path you are no longer on, which is what the columns
 * mean rather than a case to handle.
 *
 * Every column is the same width, and three fit. That is the whole reason depth is legible
 * here where a 300pt indented tree would have squeezed titles into nothing: depth is expressed
 * by columns and by the breadcrumb, not by indentation.
 *
 * [visible] exists for the auto-scroll and nothing else — see below.
 */
@Composable
internal fun BrowserRail(
    slice: BrowserSlice,
    columnWidth: Dp,
    onNav: (NavEvent) -> Unit,
    modifier: Modifier = Modifier,
    visible: Boolean = true,
) {
    val c = AnchorTheme.colors
    val scroll = rememberScrollState()

    // Descending should reveal what you descended into, not leave it off-screen to the right.
    //
    // Keyed on **visibility as well as depth**: reopening the dropdown on an already-deep path
    // must land on the deepest columns, not the shallowest, and a depth-keyed effect would not
    // re-fire because the depth had not changed.
    //
    // `snapshotFlow` rather than reading `maxValue` here: the new column has not been measured
    // when this effect runs, so the value read in the same frame is the PREVIOUS maximum — the
    // Compose shape of the same one-frame staleness the handoff warns about for the DOM.
    LaunchedEffect(slice.columns.size, visible) {
        if (!visible) return@LaunchedEffect
        snapshotFlow { scroll.maxValue }
            .first { it > 0 || slice.columns.size <= 1 }
            .let { scroll.animateScrollTo(scroll.maxValue) }
    }

    Row(modifier.fillMaxSize().horizontalScroll(scroll)) {
        slice.columns.forEachIndexed { depth, column ->
            Column(Modifier.width(columnWidth).fillMaxHeight()) {
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
                            // One handler for folders and leaves alike: both truncate the
                            // path to this column and append. It is the only "up" there is.
                            ColumnRow(row) { onNav(NavEvent.Select(depth, row.id)) }
                        }
                    }
                }
            }
            VerticalHairline()
        }
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
 * The footer: what is selected, and the button that opens it.
 *
 * Replaces the third pane the first pass used as a preview. A pane that exists only to hold
 * one button costs a whole column of a rail whose columns are the content — and the design
 * puts the same information on one bar, so the rail keeps its width for what it is for.
 *
 * Opening from here leaves the Browser lit and its columns selected, which is what makes the
 * breadcrumb's `Browser` jump meaningful afterwards.
 */
@Composable
internal fun BrowserFooter(
    slice: BrowserSlice,
    onNav: (NavEvent) -> Unit,
    modifier: Modifier = Modifier,
    depthHint: String? = null,
) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics
    val detail = slice.detail

    Row(
        modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp),
        horizontalArrangement = Arrangement.spacedBy(10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            text = if (detail == null) {
                "SELECT AN ITEM TO OPEN IT"
            } else {
                "${detail.kindLabel} · ${detail.title}"
            },
            style = SectionLabelStyle,
            color = c.ink4,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.weight(1f),
        )

        // Only past the third column, and only then: a readout that is always there stops
        // being a readout. Amber because it is the one thing on this bar worth noticing.
        depthHint?.let { Text(it, style = SectionLabelStyle, color = c.amber) }

        detail?.openKey?.let { key ->
            Row(
                Modifier
                    .height(m.touchTarget)
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
