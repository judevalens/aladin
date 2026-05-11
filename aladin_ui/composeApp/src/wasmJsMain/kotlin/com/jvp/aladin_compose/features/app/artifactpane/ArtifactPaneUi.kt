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
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.sharp.AutoGraph
import androidx.compose.material.icons.sharp.Close
import androidx.compose.material.icons.sharp.Lock
import androidx.compose.material.icons.sharp.LockOpen
import androidx.compose.material.icons.sharp.Search
import androidx.compose.material.icons.sharp.StarBorder
import androidx.compose.material.icons.sharp.Tune
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.VerticalDivider
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
import com.jvp.aladin_compose.features.app.artifactpane.link.LinkUi
import com.jvp.aladin_compose.features.app.artifactpane.page.PageEditorState
import com.jvp.aladin_compose.features.app.artifactpane.page.PageUi
import com.jvp.aladin_compose.features.app.artifactpane.page.PageStateEvent
import com.jvp.aladin_compose.features.app.artifactpane.voice.VoiceUi
import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.model.ArtifactKind
import com.jvp.aladin_compose.model.BreadcrumbItem
import com.jvp.aladin_compose.service.formatUpdatedTimestamp
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
                        pageState = state.pages.pagesById[artifact.id],
                        onRetrySave = { pageId ->
                            state.pages.eventSink(PageStateEvent.RetrySave(pageId))
                        },
                        onRetryLoad = { pageId ->
                            state.pages.eventSink(PageStateEvent.RetryLoad(pageId))
                        },
                        onRetryUpload = { pageId ->
                            state.pages.eventSink(PageStateEvent.RetryUpload(pageId))
                        },
                        onToggleEditable = { pageId ->
                            state.pages.eventSink(PageStateEvent.ToggleEditable(pageId))
                        },
                        inspectorOpen = state.inspectorOpen,
                        onToggleInspector = {
                            state.eventSink(ArtifactPaneEvent.ToggleInspector)
                        },
                    )
                }
                Box(
                    modifier = Modifier.fillMaxWidth().weight(1f),
                    contentAlignment = Alignment.TopCenter,
                ) {
                    Row(
                        modifier = Modifier.fillMaxSize(),
                    ) {
                        ArtifactWorkspaceView(
                            artifact = artifact,
                            state = state,
                            modifier = Modifier.weight(1f).fillMaxHeight(),
                        )
                        if (state.inspectorOpen) {
                            VerticalDivider(
                                modifier = Modifier.fillMaxHeight(),
                                thickness = DividerThickness,
                                color = AladinColor.Divider,
                            )
                            ArtifactInspector(
                                insight = state.activeInsight,
                                modifier = Modifier.fillMaxHeight(),
                            )
                        }
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
    pageState: PageEditorState?,
    onRetrySave: (String) -> Unit,
    onRetryLoad: (String) -> Unit,
    onRetryUpload: (String) -> Unit,
    onToggleEditable: (String) -> Unit,
    inspectorOpen: Boolean,
    onToggleInspector: () -> Unit,
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
        if (artifact.kind == ArtifactKind.Note && pageState != null) {
            WorkspacePageStatus(
                state = pageState,
                onRetrySave = onRetrySave,
                onRetryLoad = onRetryLoad,
                onRetryUpload = onRetryUpload,
            )
            WorkspaceUtilityIcon(
                icon = if (pageState.isEditable) Icons.Sharp.LockOpen else Icons.Sharp.Lock,
                contentDescription =
                    if (pageState.isEditable) {
                        "Lock document editing"
                    } else {
                        "Unlock document editing"
                    },
                isActive = !pageState.isEditable,
                onClick = { onToggleEditable(pageState.pageId) },
            )
        }
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
            isActive = inspectorOpen,
            onClick = onToggleInspector,
        )
    }
}

@Composable
private fun WorkspaceUtilityIcon(
    icon: ImageVector,
    contentDescription: String,
    isActive: Boolean = false,
    onClick: () -> Unit,
) {
    Box(
        modifier =
            Modifier.size(26.dp).aladinClickable(
                selected = isActive,
                shape = RoundedCornerShape(SharpRadius),
                colors =
                    AladinInteractionDefaults.colors(
                        hovered = AladinColor.ControlHover,
                        pressed = AladinColor.ControlPressed,
                        selected = AladinColor.InkSurface,
                        selectedHovered = AladinColor.InkSurfaceHover,
                    ),
                onClick = onClick,
            ),
        contentAlignment = Alignment.Center,
    ) {
        Icon(
            imageVector = icon,
            contentDescription = contentDescription,
            tint = if (isActive) AladinColor.OnInkSurface else AladinColor.InkSecondary,
            modifier = Modifier.size(15.dp),
        )
    }
}

@Composable
private fun ArtifactWorkspaceView(
    artifact: Artifact,
    state: ArtifactPaneState,
    modifier: Modifier = Modifier,
) {
    when (artifact.kind) {
        ArtifactKind.Note -> {
            val pageState = state.pages.pagesById[artifact.id]
            if (pageState != null) {
                PageUi(
                    state = pageState,
                    onDocumentUpdated = { pageId, markdown ->
                        state.pages.eventSink(PageStateEvent.DocumentUpdated(pageId, markdown))
                    },
                    onFileUploadRequested = { pageId, requestId, filename, contentType, base64Data ->
                        state.pages.eventSink(
                            PageStateEvent.FileUploadRequested(
                                pageId = pageId,
                                requestId = requestId,
                                filename = filename,
                                contentType = contentType,
                                base64Data = base64Data,
                            ),
                        )
                    },
                    pageEditorBridge = state.pageEditorBridge,
                    modifier = modifier,
                )
            } else {
                PlaceholderPane(
                    "Page unavailable",
                    "The selected page could not be prepared for editing.",
                )
            }
        }
        ArtifactKind.Link -> {
            val linkState = state.links.activeLink
            if (linkState != null) {
                Box(
                    modifier =
                        modifier.fillMaxSize()
                            .padding(horizontal = 32.dp, vertical = 44.dp),
                    contentAlignment = Alignment.TopCenter,
                ) {
                    LinkUi(state = linkState, modifier = Modifier.fillMaxSize())
                }
            } else {
                PlaceholderPane(
                    "Link unavailable",
                    "The selected link could not be prepared for viewing.",
                )
            }
        }
        ArtifactKind.Voice -> {
            val voiceState = state.voices.activeVoice
            if (voiceState != null) {
                Box(
                    modifier =
                        modifier.fillMaxSize()
                            .padding(horizontal = 32.dp, vertical = 44.dp),
                    contentAlignment = Alignment.TopCenter,
                ) {
                    VoiceUi(state = voiceState, modifier = Modifier.fillMaxSize())
                }
            } else {
                PlaceholderPane(
                    "Voice note unavailable",
                    "The selected recording could not be prepared for playback.",
                )
            }
        }
        ArtifactKind.File -> UnsupportedArtifactPane("File view coming next.")
    }
}

@Composable
private fun WorkspacePageStatus(
    state: PageEditorState,
    onRetrySave: (String) -> Unit,
    onRetryLoad: (String) -> Unit,
    onRetryUpload: (String) -> Unit,
) {
    val statusText =
        when {
            state.loadError != null -> "Load failed"
            state.uploadError != null -> "Upload failed"
            state.isUploading -> "Uploading..."
            state.isSaving -> "Saving..."
            state.saveError != null -> "Save failed"
            state.showUnsavedStatus -> "Unsaved changes"
            state.lastSavedAt != null ->
                "Saved • ${formatUpdatedTimestamp(raw = state.lastSavedAt, now = state.timestampTick)}"
            else -> "Ready"
        }

    Row(
        horizontalArrangement = Arrangement.spacedBy(8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            text = statusText,
            color =
                when {
                    state.loadError != null || state.saveError != null || state.uploadError != null -> MaterialTheme.colorScheme.error
                    state.isSaving || state.isUploading -> AladinColor.InkSecondary
                    else -> AladinColor.InkMuted
                },
            style = MaterialTheme.typography.bodySmall,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
        if (state.loadError != null) {
            ContextStatusAction("Retry load") { onRetryLoad(state.pageId) }
        } else if (state.uploadError != null) {
            ContextStatusAction("Retry upload") { onRetryUpload(state.pageId) }
        } else if (state.saveError != null) {
            ContextStatusAction("Retry save") { onRetrySave(state.pageId) }
        }
    }
}

@Composable
private fun ContextStatusAction(
    label: String,
    onClick: () -> Unit,
) {
    Text(
        text = label,
        color = AladinColor.Ink,
        style = MaterialTheme.typography.bodySmall,
        modifier =
            Modifier.aladinClickable(
                shape = RoundedCornerShape(SharpRadius),
                colors =
                    AladinInteractionDefaults.colors(
                        hovered = AladinColor.ControlHover,
                        pressed = AladinColor.ControlPressed,
                    ),
                onClick = onClick,
            )
                .padding(horizontal = 6.dp, vertical = 2.dp),
    )
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
                                        selectedHovered = AladinColor.ControlPressed,
                                    ),
                                onClick = { onActivateArtifact(artifact.id) },
                            )
                            .padding(horizontal = 12.dp, vertical = 12.dp),
                ) {
                    Text(
                        artifact.title,
                        color = AladinColor.Ink,
                        style = MaterialTheme.typography.titleSmall,
                        fontWeight = if (active) FontWeight.SemiBold else FontWeight.Medium,
                    )
                    Icon(
                        imageVector = Icons.Sharp.Close,
                        contentDescription = "Close tab",
                        tint = if (active) AladinColor.InkSecondary else AladinColor.Ink,
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
private fun UnsupportedArtifactPane(message: String) {
    PlaceholderPane(
        "Viewer not ready",
        message,
    )
}
