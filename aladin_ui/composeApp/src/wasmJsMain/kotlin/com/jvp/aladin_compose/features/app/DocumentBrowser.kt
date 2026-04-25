package com.jvp.aladin_compose.features.app

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.InsertDriveFile
import androidx.compose.material.icons.automirrored.outlined.KeyboardArrowRight
import androidx.compose.material.icons.outlined.Folder
import androidx.compose.material.icons.outlined.KeyboardArrowDown
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.PlainTooltip
import androidx.compose.material3.Text
import androidx.compose.material3.TooltipAnchorPosition
import androidx.compose.material3.TooltipBox
import androidx.compose.material3.TooltipDefaults
import androidx.compose.material3.rememberTooltipState
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.model.ArtifactKind
import com.jvp.aladin_compose.model.BreadcrumbItem
import com.jvp.aladin_compose.model.Item
import com.jvp.aladin_compose.ui_lib.AladinColor
import com.jvp.aladin_compose.ui_lib.AladinInteractionDefaults
import com.jvp.aladin_compose.ui_lib.aladinClickable

private val SharpRadius = 4.dp
private val ControlRadius = 6.dp

@Composable
fun DocumentBrowser(state: AppState) {

    Column(modifier = Modifier.fillMaxSize().padding(horizontal = 8.dp, vertical = 10.dp)) {
        BrowserBreadcrumbRow(state)
        Spacer(modifier = Modifier.size(10.dp))
        BrowserFilterBar(
            selected = state.browserFilter,
            onSelect = { state.eventSink(AppEvent.ChangeBrowserFilter(it)) },
            modifier = Modifier.padding(bottom = 8.dp),
        )
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            verticalArrangement = Arrangement.spacedBy(2.dp),
        ) {
            items(state.browserRows, key = { it.key }) { row ->
                when (row) {
                    is BrowserTreeRow.Folder ->
                        BrowserFolderRow(
                            item = row.item,
                            depth = row.depth,
                            expanded = row.expanded,
                            expandable = row.expandable,
                            selected = row.selected,
                            onToggleExpanded = { state.eventSink(AppEvent.ToggleItemExpanded(row.item.id)) },
                            onClick = { state.eventSink(AppEvent.SelectItem(row.item.id)) },
                        )
                    is BrowserTreeRow.Artifact ->
                        BrowserArtifactRow(
                            item = row.item,
                            artifact = row.artifact,
                            depth = row.depth,
                            selected = row.selected,
                            onClick = { state.eventSink(AppEvent.SelectItem(row.item.id)) },
                        )
                    is BrowserTreeRow.Generic ->
                        BrowserGenericRow(
                            item = row.item,
                            depth = row.depth,
                            expanded = row.expanded,
                            expandable = row.expandable,
                            selected = row.selected,
                            onToggleExpanded = { state.eventSink(AppEvent.ToggleItemExpanded(row.item.id)) },
                            onClick = { state.eventSink(AppEvent.SelectItem(row.item.id)) },
                        )
                    }
            }
        }
    }
}

@Composable
private fun BrowserFilterBar(
    selected: BrowserFilter,
    onSelect: (BrowserFilter) -> Unit,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier = modifier.fillMaxWidth().padding(horizontal = 2.dp),
        horizontalArrangement = Arrangement.spacedBy(4.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        FilterChip("All", BrowserFilter.All, selected, onSelect)
        FilterChip("Folders", BrowserFilter.Folders, selected, onSelect)
        FilterChip("Artifacts", BrowserFilter.Artifacts, selected, onSelect)
    }
}

@Composable
private fun FilterChip(
    label: String,
    filter: BrowserFilter,
    selected: BrowserFilter,
    onSelect: (BrowserFilter) -> Unit,
) {
    val active = filter == selected
    Box(
        modifier =
            Modifier.aladinClickable(
                    selected = active,
                    shape = RoundedCornerShape(999.dp),
                    colors =
                        AladinInteractionDefaults.colors(
                            hovered = AladinColor.ControlHover,
                            pressed = AladinColor.ControlHover,
                            selected = AladinColor.RowSelected,
                            selectedHovered = AladinColor.ControlHover,
                        ),
                ) {
                    onSelect(filter)
                }
                .padding(horizontal = 10.dp, vertical = 5.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            label,
            color = if (active) AladinColor.Ink else AladinColor.InkSecondary,
            style = MaterialTheme.typography.labelSmall,
            fontWeight = if (active) FontWeight.SemiBold else FontWeight.Medium,
        )
    }
}

@Composable
private fun BrowserArtifactRow(
    item: Item,
    artifact: Artifact?,
    depth: Int,
    selected: Boolean,
    onClick: () -> Unit,
) {
    Row(
        modifier =
            Modifier.fillMaxWidth()
                .padding(start = treeIndent(depth))
                .aladinClickable(
                    selected = selected,
                    shape = RoundedCornerShape(ControlRadius),
                    colors =
                        AladinInteractionDefaults.colors(
                            hovered = AladinColor.ControlHover,
                            selected = AladinColor.RowSelected,
                            selectedHovered = AladinColor.ControlHover,
                        ),
                    onClick = onClick,
                )
                .padding(horizontal = 10.dp, vertical = 9.dp),
        horizontalArrangement = Arrangement.spacedBy(12.dp),
        verticalAlignment = Alignment.Top,
    ) {
        RowSelectionMarker(selected = selected)
        ArtifactGlyph(artifact?.kind ?: ArtifactKind.Note)
        Column(verticalArrangement = Arrangement.spacedBy(4.dp), modifier = Modifier.weight(1f)) {
            Text(
                item.title,
                color = AladinColor.Ink,
                fontWeight = FontWeight.Medium,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Text(
                artifact?.summary ?: "Artifact",
                style = MaterialTheme.typography.bodySmall,
                color = AladinColor.InkSecondary,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Text(
                artifact?.updatedLabel ?: "Linked item",
                style = MaterialTheme.typography.labelSmall,
                color = AladinColor.InkMuted,
            )
        }
    }
}

private sealed interface BreadcrumbSegment {
    data class Crumb(val item: BreadcrumbItem) : BreadcrumbSegment

    data object Ellipsis : BreadcrumbSegment
}

private fun compressedBreadcrumbs(items: List<BreadcrumbItem>): List<BreadcrumbSegment> {
    return when {
        items.size <= 2 -> items.map { BreadcrumbSegment.Crumb(it) }
        else ->
            listOf(
                BreadcrumbSegment.Crumb(items.first()),
                BreadcrumbSegment.Ellipsis,
                BreadcrumbSegment.Crumb(items.last()),
            )
    }
}

@Composable
@OptIn(ExperimentalMaterial3Api::class)
private fun BrowserBreadcrumbRow(state: AppState, modifier: Modifier = Modifier) {
    val fullPath = state.breadcrumbs.joinToString(" / ") { it.label }
    val displaySegments = compressedBreadcrumbs(state.breadcrumbs)

    TooltipBox(
        positionProvider =
            TooltipDefaults.rememberTooltipPositionProvider(TooltipAnchorPosition.Above),
        tooltip = { PlainTooltip { Text(fullPath) } },
        state = rememberTooltipState(),
    ) {
        Row(
            modifier = modifier.fillMaxWidth().padding(horizontal = 2.dp, vertical = 2.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            displaySegments.forEachIndexed { index, segment ->
                if (index > 0) {
                    Text(
                        "/",
                        color = AladinColor.InkMuted,
                        style = MaterialTheme.typography.labelMedium,
                    )
                }
                when (segment) {
                    is BreadcrumbSegment.Crumb -> {
                        val isLast = index == displaySegments.lastIndex
                        Text(
                            segment.item.label,
                            style = MaterialTheme.typography.bodySmall,
                            color =
                                if (isLast) AladinColor.Ink else AladinColor.InkSecondary,
                            fontWeight = if (isLast) FontWeight.Medium else FontWeight.Normal,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                            modifier =
                                Modifier.aladinClickable(shape = RoundedCornerShape(SharpRadius)) {
                                        state.eventSink(
                                            AppEvent.NavigateBreadcrumb(segment.item.id)
                                        )
                                    }
                                    .padding(horizontal = 2.dp),
                        )
                    }
                    BreadcrumbSegment.Ellipsis -> {
                        Text(
                            "...",
                            color = AladinColor.InkMuted,
                            style = MaterialTheme.typography.bodySmall,
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun BrowserFolderRow(
    item: Item,
    depth: Int,
    expanded: Boolean,
    expandable: Boolean,
    selected: Boolean,
    onToggleExpanded: () -> Unit,
    onClick: () -> Unit,
) {
    Row(
        modifier =
            Modifier.fillMaxWidth()
                .padding(start = treeIndent(depth))
                .aladinClickable(
                    selected = selected,
                    shape = RoundedCornerShape(ControlRadius),
                    colors =
                        AladinInteractionDefaults.colors(
                            hovered = AladinColor.ControlHover,
                            selected = AladinColor.RowSelected,
                            selectedHovered = AladinColor.ControlHover,
                        ),
                    onClick = onClick,
                )
                .padding(horizontal = 10.dp, vertical = 9.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        RowSelectionMarker(selected = selected)
        ExpandToggle(
            expanded = expanded,
            expandable = expandable,
            onClick = onToggleExpanded,
        )
        Box(
            modifier =
                Modifier.size(30.dp)
                    .background(
                        if (selected) AladinColor.Panel else AladinColor.ControlHover,
                        RoundedCornerShape(SharpRadius),
                    )
                    .border(
                        0.3.dp,
                        if (selected) AladinColor.Border else AladinColor.Divider,
                        RoundedCornerShape(SharpRadius),
                    ),
            contentAlignment = Alignment.Center,
        ) {
            Icon(
                imageVector = Icons.Outlined.Folder,
                contentDescription = null,
                tint = AladinColor.InkSecondary,
                modifier = Modifier.size(19.dp),
            )
        }
        Column(verticalArrangement = Arrangement.spacedBy(1.dp), modifier = Modifier.weight(1f)) {
            Text(
                item.title,
                color = AladinColor.Ink,
                fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Medium,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
    }
}

@Composable
private fun BrowserGenericRow(
    item: Item,
    depth: Int,
    expanded: Boolean,
    expandable: Boolean,
    selected: Boolean,
    onToggleExpanded: () -> Unit,
    onClick: () -> Unit,
) {
    Row(
        modifier =
            Modifier.fillMaxWidth()
                .padding(start = treeIndent(depth))
                .aladinClickable(
                    selected = selected,
                    shape = RoundedCornerShape(ControlRadius),
                    colors =
                        AladinInteractionDefaults.colors(
                            hovered = AladinColor.ControlHover,
                            selected = AladinColor.RowSelected,
                            selectedHovered = AladinColor.ControlHover,
                        ),
                    onClick = onClick,
                )
                .padding(horizontal = 10.dp, vertical = 9.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        RowSelectionMarker(selected = selected)
        ExpandToggle(
            expanded = expanded,
            expandable = expandable,
            onClick = onToggleExpanded,
        )
        Icon(
            imageVector = Icons.AutoMirrored.Outlined.InsertDriveFile,
            contentDescription = null,
            tint = AladinColor.InkSecondary,
            modifier = Modifier.size(19.dp),
        )
        Text(
            item.title,
            color = AladinColor.Ink,
            fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Medium,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
    }
}

@Composable
private fun ExpandToggle(
    expanded: Boolean,
    expandable: Boolean,
    onClick: () -> Unit,
) {
    if (!expandable) {
        Spacer(modifier = Modifier.size(18.dp))
        return
    }

    Box(
        modifier =
            Modifier.size(18.dp)
                .aladinClickable(shape = RoundedCornerShape(SharpRadius), onClick = onClick),
        contentAlignment = Alignment.Center,
    ) {
        Icon(
            imageVector = if (expanded) Icons.Outlined.KeyboardArrowDown else Icons.AutoMirrored.Outlined.KeyboardArrowRight,
            contentDescription = if (expanded) "Collapse" else "Expand",
            tint = AladinColor.InkMuted,
            modifier = Modifier.size(16.dp),
        )
    }
}

@Composable
private fun RowSelectionMarker(selected: Boolean) {
    Box(
        modifier =
            Modifier.width(2.dp)
                .height(30.dp)
                .background(
                    if (selected) AladinColor.Ink.copy(alpha = 0.35f)
                    else androidx.compose.ui.graphics.Color.Transparent,
                    RoundedCornerShape(999.dp),
                )
    )
}

private fun treeIndent(depth: Int) = (depth * 18).dp

@Composable
fun ArtifactGlyph(kind: ArtifactKind) {
    Box(
        modifier =
            Modifier.size(30.dp).background(AladinColor.ControlHover, RoundedCornerShape(SharpRadius)),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            when (kind) {
                ArtifactKind.Note -> "N"
                ArtifactKind.Link -> "L"
                ArtifactKind.Voice -> "V"
            },
            color = AladinColor.InkSecondary,
            style = MaterialTheme.typography.labelMedium,
            fontWeight = FontWeight.SemiBold,
        )
    }
}
