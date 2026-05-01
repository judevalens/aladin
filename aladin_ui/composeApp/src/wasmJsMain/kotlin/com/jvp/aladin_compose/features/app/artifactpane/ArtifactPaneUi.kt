package com.jvp.aladin_compose.features.app.artifactpane

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.sharp.AutoGraph
import androidx.compose.material.icons.sharp.Close
import androidx.compose.material.icons.sharp.Search
import androidx.compose.material.icons.sharp.StarBorder
import androidx.compose.material.icons.sharp.Tune
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.jvp.aladin_compose.features.app.ControlRadius
import com.jvp.aladin_compose.features.app.DividerThickness
import com.jvp.aladin_compose.features.app.PlaceholderPane
import com.jvp.aladin_compose.features.app.SharpRadius
import com.jvp.aladin_compose.features.app.WorkspaceChromeMaxWidth
import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.model.BreadcrumbItem
import com.jvp.aladin_compose.service.web.WebWidget
import com.jvp.aladin_compose.ui_lib.AladinColor
import com.jvp.aladin_compose.ui_lib.AladinInteractionDefaults
import com.jvp.aladin_compose.ui_lib.aladinClickable

@Composable
fun RowScope.ArtifactPane(
    state: ArtifactPaneState,
    modifier: Modifier = Modifier,
) {
    Box(modifier = modifier.weight(1f).fillMaxHeight().background(AladinColor.Canvas)) {
        val artifact = state.activeArtifact
        if (artifact != null) {
            Column(modifier = Modifier.fillMaxSize()) {
                WorkspaceTabRail {
                    WorkspaceDocumentRail(
                        openArtifacts = state.openArtifacts,
                        activeArtifactId = artifact.id,
                        onActivateArtifact = {
                            state.eventSink(ArtifactPaneEvent.ActivateArtifact(it))
                        },
                        onCloseArtifact = {
                            state.eventSink(ArtifactPaneEvent.CloseArtifact(it))
                        },
                    )
                    WorkspaceContextRail(
                        breadcrumbs = state.breadcrumbs,
                        artifact = artifact,
                    )
                }
                Box(
                    modifier = Modifier.fillMaxWidth().weight(1f),
                    contentAlignment = Alignment.TopCenter,
                ) {
                    Column(
                        modifier =
                            Modifier.fillMaxWidth()
                                .fillMaxHeight()
                                .widthIn(max = WorkspaceChromeMaxWidth)
                                .padding(horizontal = 4.dp, vertical = 0.dp)
                    ) {
                        ArtifactWorkspaceView(
                            artifact = artifact,
                            modifier = Modifier.fillMaxSize(),
                        )
                    }
                }
            }
        } else {
            PlaceholderPane(
                "Open a document",
                "Click an artifact in the browser to open it as a document tab here.",
            )
        }
    }
}

@Composable
private fun WorkspaceTabRail(content: @Composable ColumnScope.() -> Unit) {
    Column(
        modifier = Modifier.fillMaxWidth().background(AladinColor.Canvas),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        content()
    }
}

@Composable
private fun WorkspaceContextRail(
    breadcrumbs: List<BreadcrumbItem>,
    artifact: Artifact,
) {
    val pathText =
        when {
            breadcrumbs.isNotEmpty() -> breadcrumbs.joinToString(" / ") { it.label }
            else -> artifact.title
        }

    Row(
        modifier =
            Modifier.fillMaxWidth()
                .background(AladinColor.Panel)
                .border(DividerThickness, AladinColor.Divider)
                .padding(horizontal = 14.dp, vertical = 6.dp),
        horizontalArrangement = Arrangement.spacedBy(12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            pathText,
            color = AladinColor.InkMuted,
            style = MaterialTheme.typography.bodySmall,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.weight(1f),
        )
        WorkspaceUtilityIcon(
            icon = Icons.Sharp.Search,
            contentDescription = "Search document",
            onClick = {},
        )
        WorkspaceUtilityIcon(
            icon = Icons.Sharp.StarBorder,
            contentDescription = "Favorite document",
            onClick = {},
        )
        WorkspaceUtilityIcon(
            icon = Icons.Sharp.AutoGraph,
            contentDescription = "Open graph context",
            onClick = {},
        )
        WorkspaceUtilityIcon(
            icon = Icons.Sharp.Tune,
            contentDescription = "Open document panel",
            onClick = {},
        )
    }
}

@Composable
private fun WorkspaceUtilityIcon(
    icon: ImageVector,
    contentDescription: String,
    onClick: () -> Unit,
) {
    Box(
        modifier =
            Modifier.size(26.dp).aladinClickable(
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
            imageVector = icon,
            contentDescription = contentDescription,
            tint = AladinColor.InkSecondary,
            modifier = Modifier.size(15.dp),
        )
    }
}

@Composable
private fun ArtifactWorkspaceView(artifact: Artifact, modifier: Modifier = Modifier) {
    WorkspaceEmbeddedArtifactSurface(artifact = artifact, modifier = modifier)
}

@Composable
private fun WorkspaceDocumentRail(
    openArtifacts: List<Artifact>,
    activeArtifactId: String?,
    onActivateArtifact: (String) -> Unit,
    onCloseArtifact: (String) -> Unit,
) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(6.dp),
        horizontalArrangement = Arrangement.spacedBy(6.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (openArtifacts.isEmpty()) {
            Text(
                "No open documents",
                color = AladinColor.InkMuted,
                style = MaterialTheme.typography.bodySmall,
            )
        } else {
            openArtifacts.forEach { artifact ->
                val active = artifact.id == activeArtifactId
                val shape = RoundedCornerShape(8.dp)

                Row(
                    horizontalArrangement = Arrangement.spacedBy(10.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    modifier =
                        Modifier.aladinClickable(
                                selected = active,
                                shape = shape,
                                colors =
                                    AladinInteractionDefaults.colors(
                                        hovered = AladinColor.ControlHover,
                                        pressed = AladinColor.ControlPressed,
                                        selected = AladinColor.RowSelected,
                                        selectedHovered = AladinColor.ControlHover,
                                    ),
                                onClick = { onActivateArtifact(artifact.id) },
                            )
                            .padding(horizontal = 12.dp, vertical = 12.dp),
                ) {
                    Text(
                        artifact.title,
                        color = AladinColor.Ink,
                        style = MaterialTheme.typography.titleSmall,
                        fontWeight = FontWeight.Medium,
                    )
                    Icon(
                        imageVector = Icons.Sharp.Close,
                        contentDescription = "Close tab",
                        tint = AladinColor.Ink,
                        modifier =
                            Modifier.aladinClickable(
                                enabled = true,
                                onClick = { onCloseArtifact(artifact.id) },
                                colors =
                                    AladinInteractionDefaults.colors(
                                        hovered = AladinColor.ControlPressed
                                    ),
                            ),
                    )
                }
            }
        }
    }
}

@Composable
private fun WorkspaceEmbeddedArtifactSurface(artifact: Artifact, modifier: Modifier = Modifier) {
    WebWidget(modifier = modifier.fillMaxSize())
}
