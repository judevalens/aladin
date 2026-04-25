package com.jvp.aladin_compose.features.app

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Folder
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
import com.jvp.aladin_compose.model.FolderNode
import com.jvp.aladin_compose.ui_lib.AladinLight
import com.jvp.aladin_compose.ui_lib.AladinInteractionDefaults
import com.jvp.aladin_compose.ui_lib.aladinClickable

@Composable
fun DocumentBrowser(state: AppState) {

    Column(modifier = Modifier.fillMaxSize().padding(4.dp)) {
        BrowserBreadcrumbRow(state)
        Spacer(modifier = Modifier.size(8.dp))
        BrowserFilterBar(
            selected = state.browserFilter,
            onSelect = { state.eventSink(AppEvent.ChangeBrowserFilter(it)) },
            modifier = Modifier.padding(bottom = 8.dp),
        )
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            verticalArrangement = Arrangement.spacedBy(2.dp),
        ) {
            if (state.browserFilter != BrowserFilter.Artifacts) {
                items(state.nodes, key = { it.id }) { node ->
                    BrowserFolderRow(
                        node = node,
                        selected =
                            node.id == state.selectedFolderId && state.selectedArtifact == null,
                    ) {
                        state.eventSink(AppEvent.OpenNode(node.id))
                    }
                }
            }
            val workspace = state.workspace
            if (workspace != null && state.browserFilter != BrowserFilter.Folders) {
                items(workspace.artifacts, key = { it.id }) { artifact ->
                    BrowserArtifactRow(
                        artifact = artifact,
                        selected = artifact.id == state.selectedArtifact?.id,
                        onClick = { state.eventSink(AppEvent.OpenArtifact(artifact.id)) },
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
            Modifier.aladinClickable(selected = active, shape = RoundedCornerShape(999.dp)) {
                    onSelect(filter)
                }
                .padding(horizontal = 10.dp, vertical = 5.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            label,
            color = if (active) AladinLight.TextPrimary else AladinLight.TextSecondary,
            style = MaterialTheme.typography.labelSmall,
            fontWeight = if (active) FontWeight.SemiBold else FontWeight.Medium,
        )
    }
}

@Composable
private fun BrowserArtifactRow(artifact: Artifact, selected: Boolean, onClick: () -> Unit) {
    Row(
        modifier =
            Modifier.fillMaxWidth()
                .aladinClickable(
                    selected = selected,
                    shape = RoundedCornerShape(8.dp),
                    onClick = onClick,
                )
                .padding(horizontal = 10.dp, vertical = 9.dp),
        horizontalArrangement = Arrangement.spacedBy(12.dp),
        verticalAlignment = Alignment.Top,
    ) {
        ArtifactGlyph(artifact.kind)
        Column(verticalArrangement = Arrangement.spacedBy(4.dp), modifier = Modifier.weight(1f)) {
            Text(artifact.title, color = AladinLight.TextPrimary, fontWeight = FontWeight.Medium)
            Text(
                artifact.summary,
                style = MaterialTheme.typography.bodySmall,
                color = AladinLight.TextSecondary,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Text(
                artifact.updatedLabel,
                style = MaterialTheme.typography.labelSmall,
                color = AladinLight.TextMuted,
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
                        color = AladinLight.TextMuted,
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
                                if (isLast) AladinLight.TextPrimary else AladinLight.TextSecondary,
                            fontWeight = if (isLast) FontWeight.Medium else FontWeight.Normal,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                            modifier =
                                Modifier.aladinClickable(shape = RoundedCornerShape(4.dp)) {
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
                            color = AladinLight.TextMuted,
                            style = MaterialTheme.typography.bodySmall,
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun BrowserFolderRow(node: FolderNode, selected: Boolean, onClick: () -> Unit) {
    Row(
        modifier =
            Modifier.fillMaxWidth()
                .aladinClickable(
                    selected = selected,
                    shape = RoundedCornerShape(8.dp),
                    colors =
                        AladinInteractionDefaults.colors(
                            selected = AladinLight.Border.copy(alpha = 0.55f),
                            selectedHovered = AladinLight.Border.copy(alpha = 0.7f),
                        ),
                    onClick = onClick,
                )
                .padding(horizontal = 10.dp, vertical = 9.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(11.dp),
    ) {
        Box(
            modifier =
                Modifier.size(30.dp)
                    .background(
                        if (selected) AladinLight.Surface else AladinLight.SurfaceRaised,
                        RoundedCornerShape(7.dp),
                    ),
            contentAlignment = Alignment.Center,
        ) {
            Icon(
                imageVector = Icons.Outlined.Folder,
                contentDescription = null,
                tint = AladinLight.TextSecondary,
                modifier = Modifier.size(19.dp),
            )
        }
        Column(verticalArrangement = Arrangement.spacedBy(1.dp), modifier = Modifier.weight(1f)) {
            Text(
                node.title,
                color = AladinLight.TextPrimary,
                fontWeight = FontWeight.Medium,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
    }
}

@Composable
fun ArtifactGlyph(kind: ArtifactKind) {
    Box(
        modifier =
            Modifier.size(30.dp).background(AladinLight.SurfaceRaised, RoundedCornerShape(7.dp)),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            when (kind) {
                ArtifactKind.Note -> "N"
                ArtifactKind.Link -> "L"
                ArtifactKind.Voice -> "V"
            },
            color = AladinLight.TextSecondary,
            style = MaterialTheme.typography.labelMedium,
            fontWeight = FontWeight.SemiBold,
        )
    }
}
