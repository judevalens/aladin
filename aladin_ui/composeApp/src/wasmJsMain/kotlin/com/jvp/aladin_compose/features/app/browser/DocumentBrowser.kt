package com.jvp.aladin_compose.features.app.browser

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
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.KeyboardArrowRight
import androidx.compose.material.icons.automirrored.sharp.NavigateBefore
import androidx.compose.material.icons.outlined.Folder
import androidx.compose.material.icons.outlined.KeyboardArrowDown
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.jvp.aladin_compose.features.app.BrowserRowContextMenu
import com.jvp.aladin_compose.features.app.BrowserRowKind
import com.jvp.aladin_compose.features.app.BrowserRowMenuModel
import com.jvp.aladin_compose.features.app.BrowserRowMenuRequest
import com.jvp.aladin_compose.model.ArtifactKind
import com.jvp.aladin_compose.model.ArtifactPreview
import com.jvp.aladin_compose.model.BreadcrumbItem
import com.jvp.aladin_compose.model.FolderNode
import com.jvp.aladin_compose.ui_lib.AladinColor
import com.jvp.aladin_compose.ui_lib.AladinInteractionDefaults
import com.jvp.aladin_compose.ui_lib.ErrorState
import com.jvp.aladin_compose.ui_lib.LoadingState
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
fun DocumentBrowser(
    state: DocumentBrowserState,
    onOpenRowMenu: (BrowserRowMenuRequest) -> Unit,
    onDismissRowMenu: () -> Unit,
) {
    if (state.loading) {
        LoadingState()
        return
    }
    if (state.errorMessage != null) {
        ErrorState(state.errorMessage) { state.eventSink(DocumentBrowserEvent.RetryLoad) }
        return
    }
    if (state.rows.isEmpty()) {
        BrowserEmptyState(state)
        return
    }

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
                    is BrowserTreeRow.Folder ->
                        BrowserFolderRow(
                            folder = row.folder,
                            depth = row.depth,
                            expanded = row.expanded,
                            expandable = row.expandable,
                            selected = row.selected,
                            menu = row.menu,
                            rename =
                                state.activeRename?.takeIf {
                                    it.rowKind == BrowserRowKind.Folder && it.rowId == row.folder.id
                                },
                            onToggleExpanded = {
                                onDismissRowMenu()
                                state.eventSink(
                                    DocumentBrowserEvent.ToggleFolderExpanded(
                                        row.folder.id,
                                        row.depth,
                                    )
                                )
                            },
                            onClick = {
                                onDismissRowMenu()
                                state.eventSink(DocumentBrowserEvent.FocusFolder(row.folder.id))
                                if (row.expandable) {
                                    state.eventSink(
                                        DocumentBrowserEvent.ToggleFolderExpanded(
                                            row.folder.id,
                                            row.depth,
                                        )
                                    )
                                }
                            },
                            onMenuAction = { option ->
                                state.eventSink(
                                    DocumentBrowserEvent.CreateInFolder(row.folder.id, option)
                                )
                            },
                            onStartRename = {
                                onDismissRowMenu()
                                state.eventSink(
                                    DocumentBrowserEvent.StartRename(
                                        BrowserRowKind.Folder,
                                        row.folder.id,
                                        row.folder.title,
                                    )
                                )
                            },
                            onRenameDraftChanged = { title ->
                                state.eventSink(
                                    DocumentBrowserEvent.RenameDraftChanged(row.folder.id, title)
                                )
                            },
                            onCommitRename = {
                                state.eventSink(DocumentBrowserEvent.CommitRename(row.folder.id))
                            },
                            onCancelRename = {
                                state.eventSink(DocumentBrowserEvent.CancelRename(row.folder.id))
                            },
                            onOpenMenu = onOpenRowMenu,
                        )
                    is BrowserTreeRow.Artifact ->
                        BrowserArtifactRow(
                            artifact = row.artifact,
                            depth = row.depth,
                            selected = row.selected,
                            menu = row.menu,
                            rename =
                                state.activeRename?.takeIf {
                                    it.rowKind == BrowserRowKind.Artifact &&
                                        it.rowId == row.artifact.id
                                },
                            onClick = {
                                onDismissRowMenu()
                                state.eventSink(DocumentBrowserEvent.OpenArtifact(row.artifact.id))
                            },
                            onStartRename = {
                                onDismissRowMenu()
                                state.eventSink(
                                    DocumentBrowserEvent.StartRename(
                                        BrowserRowKind.Artifact,
                                        row.artifact.id,
                                        row.artifact.title,
                                    )
                                )
                            },
                            onRenameDraftChanged = { title ->
                                state.eventSink(
                                    DocumentBrowserEvent.RenameDraftChanged(row.artifact.id, title)
                                )
                            },
                            onCommitRename = {
                                state.eventSink(DocumentBrowserEvent.CommitRename(row.artifact.id))
                            },
                            onCancelRename = {
                                state.eventSink(DocumentBrowserEvent.CancelRename(row.artifact.id))
                            },
                            onOpenMenu = onOpenRowMenu,
                        )
                }
            }
        }
    }
}

@Composable
private fun BrowserEmptyState(state: DocumentBrowserState) {
    val inFolder = state.scopeBreadcrumbs.size > 1
    val folderLabel = state.scopeBreadcrumbs.lastOrNull()?.label ?: "this folder"
    val title =
        if (inFolder) {
            "This folder is empty"
        } else {
            "Start your workspace"
        }
    val body =
        if (inFolder) {
            "Add a note or create a subfolder inside $folderLabel to begin organizing artifacts."
        } else {
            "Create a folder to organize research, or add a note to begin capturing artifacts."
        }

    Box(modifier = Modifier.fillMaxSize().padding(18.dp), contentAlignment = Alignment.Center) {
        Column(
            modifier =
                Modifier.width(320.dp)
                    .border(1.dp, AladinColor.Border, RoundedCornerShape(ControlRadius))
                    .background(AladinColor.Panel, RoundedCornerShape(ControlRadius))
                    .padding(horizontal = 20.dp, vertical = 22.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Box(
                modifier =
                    Modifier.size(42.dp)
                        .background(AladinColor.ControlHover, RoundedCornerShape(SharpRadius)),
                contentAlignment = Alignment.Center,
            ) {
                Icon(
                    imageVector = Icons.Outlined.Folder,
                    contentDescription = null,
                    tint = AladinColor.InkSecondary,
                    modifier = Modifier.size(18.dp),
                )
            }
            Text(
                title,
                color = AladinColor.Ink,
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.SemiBold,
            )
            Text(body, color = AladinColor.InkMuted, style = MaterialTheme.typography.bodyMedium)
            Row(
                horizontalArrangement = Arrangement.spacedBy(10.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                TextButton(onClick = { state.eventSink(DocumentBrowserEvent.CreateFolder) }) {
                    Text("+ Folder", color = AladinColor.Ink)
                }
                TextButton(onClick = { state.eventSink(DocumentBrowserEvent.CreateArtifact) }) {
                    Text(if (inFolder) "+ Note" else "+ Artifact", color = AladinColor.Ink)
                }
            }
        }
    }
}

@Composable
private fun BrowserScopeBreadcrumbRow(state: DocumentBrowserState) {
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
    artifact: ArtifactPreview,
    depth: Int,
    selected: Boolean,
    menu: BrowserRowMenuModel,
    rename: BrowserRenameState?,
    onClick: () -> Unit,
    onStartRename: () -> Unit,
    onRenameDraftChanged: (String) -> Unit,
    onCommitRename: () -> Unit,
    onCancelRename: () -> Unit,
    onOpenMenu: (BrowserRowMenuRequest) -> Unit,
) {
    val isRenaming = rename != null
    Row(
        modifier =
            Modifier.fillMaxWidth()
                .padding(start = treeIndent(depth))
                .aladinClickable(
                    enabled = !isRenaming,
                    selected = selected,
                    shape = RoundedCornerShape(ControlRadius),
                    colors =
                        AladinInteractionDefaults.colors(
                            hovered = AladinColor.ControlHover,
                            selected = AladinColor.RowSelected,
                            selectedHovered = AladinColor.ControlPressed,
                        ),
                    onDoubleClick = onStartRename,
                    onClick = onClick,
                )
                .padding(
                    horizontal = BrowserRowHorizontalPadding,
                    vertical = BrowserRowVerticalPadding,
                ),
        horizontalArrangement = Arrangement.spacedBy(BrowserRowContentGap),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        RowSelectionMarker(selected = selected)
        ArtifactGlyph(artifact.kind, selected = selected)
        Column(verticalArrangement = Arrangement.spacedBy(2.dp), modifier = Modifier.weight(1f)) {
            if (rename != null) {
                RenameTitleField(
                    rename = rename,
                    selected = selected,
                    onDraftChanged = onRenameDraftChanged,
                    onCommit = onCommitRename,
                    onCancel = onCancelRename,
                )
            } else {
                BrowserRowTitle(title = artifact.title, selected = selected)
            }
        }
        BrowserRowContextMenu(
            menu = menu,
            selected = selected,
            onActionSelected = { actionId ->
                if (actionId == RenameActionId) {
                    onStartRename()
                }
            },
            onOpenMenu = onOpenMenu,
        )
    }
}

@Composable
private fun BrowserRowTitle(title: String, selected: Boolean) {
    Text(
        title,
        color = AladinColor.Ink,
        fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Medium,
        maxLines = 1,
        overflow = TextOverflow.Ellipsis,
    )
}

@Composable
private fun RenameTitleField(
    rename: BrowserRenameState,
    selected: Boolean,
    onDraftChanged: (String) -> Unit,
    onCommit: () -> Unit,
    onCancel: () -> Unit,
) {
    val focusRequester = remember { FocusRequester() }
    val fieldShape = RoundedCornerShape(5.dp)
    var hadFocus by remember(rename.rowId) { mutableStateOf(false) }
    var cancelled by remember(rename.rowId) { mutableStateOf(false) }
    var fieldValue by
        remember(rename.rowId) {
            mutableStateOf(
                TextFieldValue(
                    text = rename.draftTitle,
                    selection = TextRange(0, rename.draftTitle.length),
                )
            )
        }

    LaunchedEffect(rename.rowId) { focusRequester.requestFocus() }

    BasicTextField(
        value = fieldValue,
        onValueChange = { next ->
            fieldValue = next
            onDraftChanged(next.text)
        },
        singleLine = true,
        enabled = !rename.saving,
        textStyle =
            MaterialTheme.typography.bodyMedium.copy(
                color = if (rename.saving) AladinColor.InkMuted else AladinColor.Ink,
                fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Medium,
            ),
        modifier =
            Modifier.fillMaxWidth()
                .focusRequester(focusRequester)
                .onFocusChanged { focus ->
                    if (focus.isFocused) {
                        hadFocus = true
                    } else if (hadFocus && !cancelled) {
                        onCommit()
                    }
                }
                .onPreviewKeyEvent { event ->
                    if (event.type != KeyEventType.KeyDown) {
                        return@onPreviewKeyEvent false
                    }
                    when (event.key) {
                        Key.Enter -> {
                            onCommit()
                            true
                        }
                        Key.Escape -> {
                            cancelled = true
                            onCancel()
                            true
                        }
                        else -> false
                    }
                }
                .background(AladinColor.Panel, fieldShape),
        decorationBox = { innerTextField ->
            Row(
                modifier =
                    Modifier.fillMaxWidth()
                        .border(
                            0.5.dp,
                            if (selected) AladinColor.Border else AladinColor.Divider,
                            fieldShape,
                        )
                        .background(AladinColor.Panel, fieldShape)
                        .padding(horizontal = 6.dp, vertical = 6.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Box(
                    modifier =
                        Modifier.width(2.dp)
                            .height(16.dp)
                            .background(AladinColor.ActiveMarker, RoundedCornerShape(999.dp))
                )
                Spacer(modifier = Modifier.width(6.dp))
                Box(modifier = Modifier.weight(1f)) { innerTextField() }
            }
        },
    )
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
    folder: FolderNode,
    depth: Int,
    expanded: Boolean,
    expandable: Boolean,
    selected: Boolean,
    menu: BrowserRowMenuModel,
    rename: BrowserRenameState?,
    onToggleExpanded: () -> Unit,
    onClick: () -> Unit,
    onMenuAction: (BrowserCreateOption) -> Unit,
    onStartRename: () -> Unit,
    onRenameDraftChanged: (String) -> Unit,
    onCommitRename: () -> Unit,
    onCancelRename: () -> Unit,
    onOpenMenu: (BrowserRowMenuRequest) -> Unit,
) {
    val isRenaming = rename != null
    Row(
        modifier =
            Modifier.fillMaxWidth()
                .padding(start = treeIndent(depth))
                .aladinClickable(
                    enabled = !isRenaming,
                    selected = selected,
                    shape = RoundedCornerShape(ControlRadius),
                    colors =
                        AladinInteractionDefaults.colors(
                            hovered = AladinColor.ControlHover,
                            selected = AladinColor.RowSelected,
                            selectedHovered = AladinColor.ControlPressed,
                        ),
                    onDoubleClick = onStartRename,
                    onClick = onClick,
                )
                .padding(
                    horizontal = BrowserRowHorizontalPadding,
                    vertical = BrowserRowVerticalPadding,
                ),
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
            if (rename != null) {
                RenameTitleField(
                    rename = rename,
                    selected = selected,
                    onDraftChanged = onRenameDraftChanged,
                    onCommit = onCommitRename,
                    onCancel = onCancelRename,
                )
            } else {
                BrowserRowTitle(title = folder.title, selected = selected)
            }
        }
        BrowserRowContextMenu(
            menu = menu,
            selected = selected,
            onActionSelected = { actionId ->
                if (actionId == RenameActionId) {
                    onStartRename()
                } else {
                    menuActionToCreateOption(actionId)?.let(onMenuAction)
                }
            },
            onOpenMenu = onOpenMenu,
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
            imageVector =
                if (expanded) Icons.Outlined.KeyboardArrowDown
                else Icons.AutoMirrored.Outlined.KeyboardArrowRight,
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

private fun menuActionToCreateOption(actionId: String): BrowserCreateOption? {
    return when (actionId) {
        "create:folder" -> BrowserCreateOption.Folder
        "create:note" -> BrowserCreateOption.Note
        "create:link" -> BrowserCreateOption.Link
        "create:voice" -> BrowserCreateOption.Voice
        "create:upload" -> BrowserCreateOption.Upload
        else -> null
    }
}

private const val RenameActionId = "rename"

@Composable
fun ArtifactGlyph(kind: ArtifactKind, selected: Boolean = false) {
    Box(
        modifier =
            Modifier.size(BrowserGlyphSize)
                .background(
                    if (selected) AladinColor.ControlPressed else AladinColor.ControlHover,
                    RoundedCornerShape(SharpRadius),
                ),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            when (kind) {
                ArtifactKind.Note -> "N"
                ArtifactKind.Link -> "L"
                ArtifactKind.Voice -> "V"
                ArtifactKind.File -> "F"
            },
            color = if (selected) AladinColor.Ink else AladinColor.InkSecondary,
            style = MaterialTheme.typography.labelMedium,
            fontWeight = FontWeight.SemiBold,
        )
    }
}
