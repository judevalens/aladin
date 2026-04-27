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
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
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
private val BrowserScopeHeaderVerticalPadding = 7.dp
private val BrowserGlyphSize = 32.dp
private val BrowserIconSize = 15.dp
private val BrowserChevronSize = 16.dp
private val BrowserMarkerHeight = 20.dp
private val BrowserIndent = 16.dp

@Composable
fun DocumentBrowser(state: DocumentBrowserState) {
    Column(modifier = Modifier.fillMaxSize().padding(horizontal = 10.dp, vertical = 10.dp)) {
        BrowserScopeBreadcrumbRow(state)
        Spacer(modifier = Modifier.size(8.dp))
        HorizontalDivider(color = AladinColor.Divider)
        Spacer(modifier = Modifier.size(8.dp))
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            verticalArrangement = Arrangement.spacedBy(3.dp),
        ) {
            items(state.rows, key = { it.key }) { row ->
                when (row) {
                    is BrowserTreeRow.ScopeBack -> Unit
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
private fun BrowserScopeBreadcrumbRow(
    state: DocumentBrowserState,
) {
    val displaySegments = compressedBreadcrumbs(state.scopeBreadcrumbs)

    Row(
        modifier =
            Modifier.fillMaxWidth()
                .aladinClickable(
                    enabled = state.canNavigateScopeBack,
                    shape = RoundedCornerShape(ControlRadius),
                    colors =
                        AladinInteractionDefaults.colors(
                            hovered = AladinColor.ControlHover,
                            pressed = AladinColor.ControlPressed,
                        ),
                    onClick = {
                        state.eventSink(DocumentBrowserEvent.NavigateScope(state.scopeBackTargetId))
                    },
                )
                .padding(
                    horizontal = BrowserRowHorizontalPadding,
                    vertical = BrowserScopeHeaderVerticalPadding,
                ),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(BrowserRowContentGap),
    ) {
        if (state.canNavigateScopeBack) {
            Icon(
                imageVector = Icons.AutoMirrored.Sharp.NavigateBefore,
                contentDescription = null,
                tint = AladinColor.InkMuted,
                modifier = Modifier.size(BrowserIconSize),
            )
        }
        displaySegments.forEachIndexed { index, segment ->
            if (index > 0) {
                Text(
                    "/",
                    color = AladinColor.InkMuted,
                    style = MaterialTheme.typography.labelMedium,
                )
            }
            when (segment) {
                is BreadcrumbSegment.Crumb ->
                    Text(
                        segment.item.label,
                        color = AladinColor.InkSecondary,
                        style = MaterialTheme.typography.titleSmall,
                        fontWeight = FontWeight.SemiBold,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                BreadcrumbSegment.Ellipsis ->
                    Text(
                        "...",
                        color = AladinColor.InkMuted,
                        style = MaterialTheme.typography.titleSmall,
                        fontWeight = FontWeight.SemiBold,
                    )
            }
        }
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
                            selectedHovered = AladinColor.ControlPressed,
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
                color = AladinColor.Ink,
                fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Medium,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Text(
                artifact?.updatedLabel ?: "Linked item",
                style = MaterialTheme.typography.bodySmall,
                color = if (selected) AladinColor.InkSecondary else AladinColor.InkMuted,
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
                            selectedHovered = AladinColor.ControlPressed,
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
            tint = if (selected) AladinColor.Ink else AladinColor.InkSecondary,
            modifier = Modifier.size(BrowserIconSize),
        )
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
                            selectedHovered = AladinColor.ControlPressed,
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
            tint = if (selected) AladinColor.Ink else AladinColor.InkSecondary,
            modifier = Modifier.size(BrowserIconSize),
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
                            hovered = AladinColor.ControlHover,
                            pressed = AladinColor.ControlPressed,
                        ),
                    onClick = onClick,
                ),
        contentAlignment = Alignment.Center,
    ) {
        Icon(
            imageVector = if (expanded) Icons.Outlined.KeyboardArrowDown else Icons.AutoMirrored.Outlined.KeyboardArrowRight,
            contentDescription = if (expanded) "Collapse" else "Expand",
            tint = if (selected) AladinColor.InkSecondary else AladinColor.InkMuted,
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
                    if (selected) AladinColor.ActiveMarker else Color.Transparent,
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
                    if (selected) AladinColor.ControlPressed
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
            color = if (selected) AladinColor.Ink else AladinColor.InkSecondary,
            style = MaterialTheme.typography.labelMedium,
            fontWeight = FontWeight.SemiBold,
        )
    }
}
