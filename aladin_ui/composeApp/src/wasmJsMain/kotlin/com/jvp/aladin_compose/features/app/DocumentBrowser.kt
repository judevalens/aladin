package com.jvp.aladin_compose.features.app

import androidx.compose.foundation.background
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
import androidx.compose.material.icons.automirrored.sharp.NavigateBefore
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
private val BrowserRowHorizontalPadding = 8.dp
private val BrowserRowVerticalPadding = 5.dp
private val BrowserRowContentGap = 6.dp
private val BrowserGlyphSize = 22.dp
private val BrowserIconSize = 15.dp
private val BrowserChevronSize = 16.dp
private val BrowserMarkerHeight = 20.dp
private val BrowserIndent = 16.dp

@Composable
fun DocumentBrowser(state: DocumentBrowserState) {
    Column(modifier = Modifier.fillMaxSize().padding(horizontal = 8.dp, vertical = 10.dp)) {
        BrowserBreadcrumbRow(state)
        Spacer(modifier = Modifier.size(12.dp))
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            verticalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            items(state.rows, key = { it.key }) { row ->
                when (row) {
                    is BrowserTreeRow.ScopeBack ->
                        BrowserScopeBackRow(
                            label = row.label,
                            onClick = { state.eventSink(DocumentBrowserEvent.NavigateScope(row.targetScopeId)) },
                        )
                    is BrowserTreeRow.Folder ->
                        BrowserFolderRow(
                            item = row.item,
                            depth = row.depth,
                            expanded = row.expanded,
                            expandable = row.expandable,
                            selected = row.selected,
                            onToggleExpanded = { state.eventSink(DocumentBrowserEvent.ToggleItemExpanded(row.item.id, row.depth)) },
                            onClick = {
                                state.eventSink(DocumentBrowserEvent.SelectItem(row.item.id))
                                if (row.expandable) {
                                    state.eventSink(DocumentBrowserEvent.ToggleItemExpanded(row.item.id, row.depth))
                                }
                            },
                        )
                    is BrowserTreeRow.Artifact ->
                        BrowserArtifactRow(
                            item = row.item,
                            artifact = row.artifact,
                            depth = row.depth,
                            selected = row.selected,
                            onClick = { state.eventSink(DocumentBrowserEvent.SelectItem(row.item.id)) },
                        )
                    is BrowserTreeRow.Generic ->
                        BrowserGenericRow(
                            item = row.item,
                            depth = row.depth,
                            expanded = row.expanded,
                            expandable = row.expandable,
                            selected = row.selected,
                            onToggleExpanded = { state.eventSink(DocumentBrowserEvent.ToggleItemExpanded(row.item.id, row.depth)) },
                            onClick = {
                                state.eventSink(DocumentBrowserEvent.SelectItem(row.item.id))
                                if (row.expandable) {
                                    state.eventSink(DocumentBrowserEvent.ToggleItemExpanded(row.item.id, row.depth))
                                }
                            },
                        )
                }
            }
        }
    }
}

@Composable
private fun BrowserScopeBackRow(
    label: String,
    onClick: () -> Unit,
) {
    Row(
        modifier =
            Modifier.fillMaxWidth()
                .aladinClickable(
                    shape = RoundedCornerShape(ControlRadius),
                    colors =
                        AladinInteractionDefaults.colors(
                            hovered = AladinColor.ControlHover,
                            pressed = AladinColor.ControlPressed,
                        ),
                    onClick = onClick,
                )
                .padding(horizontal = BrowserRowHorizontalPadding, vertical = BrowserRowVerticalPadding),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(BrowserRowContentGap),
    ) {
        Icon(
            imageVector = Icons.AutoMirrored.Sharp.NavigateBefore,
            contentDescription = null,
            tint = AladinColor.InkMuted,
            modifier = Modifier.size(BrowserIconSize),
        )
        Text(
            label,
            color = AladinColor.InkMuted,
            fontWeight = FontWeight.Medium,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
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
                            selected = AladinColor.InkSurface,
                            selectedHovered = AladinColor.InkSurfaceHover,
                        ),
                    onClick = onClick,
                )
                .padding(horizontal = BrowserRowHorizontalPadding, vertical = BrowserRowVerticalPadding),
        horizontalArrangement = Arrangement.spacedBy(BrowserRowContentGap),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        RowSelectionMarker(selected = selected)
        ArtifactGlyph(artifact?.kind ?: ArtifactKind.Note, selected = selected)
        Column(verticalArrangement = Arrangement.spacedBy(2.dp), modifier = Modifier.weight(1f)) {
            Text(
                item.title,
                color = if (selected) AladinColor.OnInkSurface else AladinColor.Ink,
                fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Medium,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Text(
                artifact?.updatedLabel ?: "Linked item",
                style = MaterialTheme.typography.bodySmall,
                color = if (selected) AladinColor.OnInkSurface.copy(alpha = 0.72f) else AladinColor.InkMuted,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
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
private fun BrowserBreadcrumbRow(state: DocumentBrowserState, modifier: Modifier = Modifier) {
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
                                if (isLast) AladinColor.Ink else AladinColor.InkMuted,
                            fontWeight = if (isLast) FontWeight.Medium else FontWeight.Normal,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                            modifier =
                                Modifier.aladinClickable(
                                    shape = RoundedCornerShape(SharpRadius),
                                    colors =
                                        AladinInteractionDefaults.colors(
                                            hovered = AladinColor.ControlHover,
                                            pressed = AladinColor.ControlPressed,
                                        ),
                                ) {
                                        state.eventSink(
                                            DocumentBrowserEvent.NavigateBreadcrumb(segment.item.id)
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
                            selected = AladinColor.InkSurface,
                            selectedHovered = AladinColor.InkSurfaceHover,
                        ),
                    onClick = onClick,
                )
                .padding(horizontal = BrowserRowHorizontalPadding, vertical = BrowserRowVerticalPadding),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(BrowserRowContentGap),
    ) {
        RowSelectionMarker(selected = selected)
        ExpandToggle(
            expanded = expanded,
            expandable = expandable,
            selected = selected,
            onClick = onToggleExpanded,
        )
        Icon(
            imageVector = Icons.Outlined.Folder,
            contentDescription = null,
            tint = if (selected) AladinColor.OnInkSurface else AladinColor.InkSecondary,
            modifier = Modifier.size(BrowserIconSize),
        )
        Column(verticalArrangement = Arrangement.spacedBy(1.dp), modifier = Modifier.weight(1f)) {
            Text(
                item.title,
                color = if (selected) AladinColor.OnInkSurface else AladinColor.Ink,
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
                            selected = AladinColor.InkSurface,
                            selectedHovered = AladinColor.InkSurfaceHover,
                        ),
                    onClick = onClick,
                )
                .padding(horizontal = BrowserRowHorizontalPadding, vertical = BrowserRowVerticalPadding),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(BrowserRowContentGap),
    ) {
        RowSelectionMarker(selected = selected)
        ExpandToggle(
            expanded = expanded,
            expandable = expandable,
            selected = selected,
            onClick = onToggleExpanded,
        )
        Icon(
            imageVector = Icons.AutoMirrored.Outlined.InsertDriveFile,
            contentDescription = null,
            tint = if (selected) AladinColor.OnInkSurface else AladinColor.InkSecondary,
            modifier = Modifier.size(BrowserIconSize),
        )
        Text(
            item.title,
            color = if (selected) AladinColor.OnInkSurface else AladinColor.Ink,
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
    selected: Boolean,
    onClick: () -> Unit,
) {
    if (!expandable) {
        Spacer(modifier = Modifier.size(BrowserChevronSize))
        return
    }

    Box(
        modifier =
            Modifier.size(BrowserChevronSize)
                .aladinClickable(
                    shape = RoundedCornerShape(SharpRadius),
                    colors =
                        AladinInteractionDefaults.colors(
                            hovered =
                                if (selected) AladinColor.OnInkSurface.copy(alpha = 0.12f)
                                else AladinColor.ControlHover,
                            pressed =
                                if (selected) AladinColor.OnInkSurface.copy(alpha = 0.18f)
                                else AladinColor.ControlPressed,
                        ),
                    onClick = onClick,
                ),
        contentAlignment = Alignment.Center,
    ) {
        Icon(
            imageVector = if (expanded) Icons.Outlined.KeyboardArrowDown else Icons.AutoMirrored.Outlined.KeyboardArrowRight,
            contentDescription = if (expanded) "Collapse" else "Expand",
            tint = if (selected) AladinColor.OnInkSurface.copy(alpha = 0.72f) else AladinColor.InkMuted,
            modifier = Modifier.size(14.dp),
        )
    }
}

@Composable
private fun RowSelectionMarker(selected: Boolean) {
    Box(
        modifier =
            Modifier.width(2.dp)
                .height(BrowserMarkerHeight)
                .background(
                    if (selected) AladinColor.OnInkSurface.copy(alpha = 0.42f)
                    else androidx.compose.ui.graphics.Color.Transparent,
                    RoundedCornerShape(999.dp),
                )
    )
}

private fun treeIndent(depth: Int) = BrowserIndent * depth

@Composable
fun ArtifactGlyph(kind: ArtifactKind, selected: Boolean = false) {
    Box(
        modifier =
            Modifier.size(BrowserGlyphSize)
                .background(
                    if (selected) AladinColor.OnInkSurface.copy(alpha = 0.14f)
                    else AladinColor.ControlHover,
                    RoundedCornerShape(SharpRadius),
                ),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            when (kind) {
                ArtifactKind.Note -> "N"
                ArtifactKind.Link -> "L"
                ArtifactKind.Voice -> "V"
            },
            color = if (selected) AladinColor.OnInkSurface else AladinColor.InkSecondary,
            style = MaterialTheme.typography.labelMedium,
            fontWeight = FontWeight.SemiBold,
        )
    }
}
