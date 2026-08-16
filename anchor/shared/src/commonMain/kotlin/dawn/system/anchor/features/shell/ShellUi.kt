package dawn.system.anchor.features.shell

import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.ContentTransform
import androidx.compose.animation.Crossfade
import androidx.compose.animation.SizeTransform
import androidx.compose.animation.core.CubicBezierEasing
import androidx.compose.animation.core.animateDpAsState
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.WindowInsetsSides
import androidx.compose.foundation.layout.exclude
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.ime
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.only
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.requiredWidth
import androidx.compose.foundation.layout.safeDrawing
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.pager.HorizontalPager
import androidx.compose.foundation.pager.rememberPagerState
import androidx.compose.runtime.snapshotFlow
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.runtime.key
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.IntSize
import androidx.compose.ui.unit.dp
import androidx.compose.ui.zIndex
import dawn.system.anchor.features.note.WebSurfacePage
import dawn.system.anchor.features.note.WebSurfaceHost
import dawn.system.anchor.features.note.rememberWebSurfaceHost
import dawn.system.anchor.features.artifact.LinkArtifactView
import dawn.system.anchor.features.artifact.VoiceArtifactView
import dawn.system.anchor.features.reader.DocumentReader
import dawn.system.anchor.domain.ContentRow
import dawn.system.anchor.domain.DetailSection
import dawn.system.anchor.domain.Destination
import dawn.system.anchor.domain.NodeKind
import dawn.system.anchor.domain.PathLevel
import dawn.system.anchor.domain.SidebarNav
import dawn.system.anchor.domain.StatCard
import dawn.system.anchor.domain.Tone
import dawn.system.anchor.domain.WorkspaceNode
import dawn.system.anchor.services.design.AladinMetrics
import dawn.system.anchor.services.design.AnchorShape
import dawn.system.anchor.services.design.AnchorTheme
import dawn.system.anchor.services.design.BellIcon
import dawn.system.anchor.services.design.ChevronDirection
import dawn.system.anchor.services.design.ChevronIcon
import dawn.system.anchor.services.design.ChipStyle
import dawn.system.anchor.services.design.CloseIcon
import dawn.system.anchor.services.design.ColumnsIcon
import dawn.system.anchor.services.design.DeleteIcon
import dawn.system.anchor.services.design.NewFolderIcon
import dawn.system.anchor.services.design.NewPageIcon
import dawn.system.anchor.services.design.PlusIcon
import dawn.system.anchor.services.design.RenameIcon
import dawn.system.anchor.services.design.MetaStyle
import dawn.system.anchor.services.design.OpenItemsIcon
import dawn.system.anchor.services.design.SearchIcon
import dawn.system.anchor.services.design.SectionLabelStyle
import dawn.system.anchor.services.design.SparkleIcon
import dawn.system.anchor.services.design.artifactKindIcon
import dawn.system.anchor.services.design.destinationIcon
import dawn.system.anchor.services.sync.SyncStatus

/**
 * The no-rail shell's chrome (PRD rev 2). Stateless: it renders [ShellScreen.State] and
 * emits events, which is what lets the same layout be driven by a test, a preview, or the
 * real presenter.
 *
 * Every column runs the full height of the screen so its background reaches the physical
 * edges, under the status bar and the home indicator; only *content* is inset, via
 * [safeContent]. A chrome-coloured band across the top would read as a title bar, and this
 * design does not have one.
 */
@Composable
fun ShellUi(state: ShellScreen.State, modifier: Modifier = Modifier) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    Box(modifier.fillMaxSize().background(c.bg)) {
        Row(Modifier.fillMaxSize()) {
            val sidebarWidth by animateDpAsState(
                targetValue = if (state.sidebarOpen) m.sidebarWidth else 0.dp,
                animationSpec = tween(
                    durationMillis = AladinMetrics.COLUMN_COLLAPSE_MILLIS,
                    easing = CubicBezierEasing(0.2f, 0.8f, 0.2f, 1f),
                ),
                label = "sidebar-collapse",
            )
            // Collapsing animates width to zero while the content keeps its natural width,
            // so text does not reflow mid-animation.
            Box(Modifier.width(sidebarWidth).fillMaxHeight().clip(RoundedCornerShape(0.dp))) {
                Sidebar(state, Modifier.requiredWidth(m.sidebarWidth))
            }
            if (state.sidebarOpen) VerticalHairline()

            DetailColumn(state, Modifier.weight(1f))

            if (state.copilotOpen) {
                VerticalHairline()
                CopilotPane(state)
            }
        }

        if (state.switcherOpen) SwitcherOverlay(state)
        if (state.createOpen) CreateMenu(state)
        if (state.browseOpen) ColumnBrowser(state)
        state.actionsFor?.let { NodeActionsSheet(state, it) }
        state.rename?.let { RenameDialog(state, it) }
        state.pendingDelete?.let { DeleteConfirm(state, it) }
        state.writeError?.let { WriteErrorToast(state, it) }
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// Writes — create, rename, delete
// ─────────────────────────────────────────────────────────────────────────────

/** A dimmed backdrop that dismisses on tap. Every modal here sits on one. */
@Composable
private fun Scrim(onDismiss: () -> Unit) {
    Box(
        Modifier
            .fillMaxSize()
            .background(AnchorTheme.colors.scrim)
            .clickable(indication = null, interactionSource = null, onClick = onDismiss),
    )
}

/**
 * What a long press offers. Desktop reaches this with a right-click; a sheet anchored to
 * nothing in particular is the honest touch equivalent — it leads with the row's own name
 * so there is never a question about which one you are about to rename or delete.
 */
@Composable
private fun NodeActionsSheet(state: ShellScreen.State, node: WorkspaceNode) {
    val c = AnchorTheme.colors
    val dismiss = { state.eventSink(ShellScreen.Event.Write.ActionsDismissed) }

    Scrim(dismiss)
    Box(Modifier.fillMaxSize().safeContent(), contentAlignment = Alignment.Center) {
        Column(
            Modifier
                .width(360.dp)
                .clip(AnchorShape.card)
                .background(c.card)
                .border(1.dp, c.line, AnchorShape.card)
                .padding(vertical = 8.dp),
        ) {
            Row(
                Modifier.fillMaxWidth().padding(horizontal = 18.dp, vertical = 12.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(11.dp),
            ) {
                artifactKindIcon(
                    artifactType = node.artifactType,
                    isContainer = node.isContainer,
                    tint = if (node.isContainer) c.amber else c.ink3,
                )
                Text(
                    node.title.ifBlank { "Untitled" },
                    style = MaterialTheme.typography.titleMedium,
                    color = c.ink,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
            Box(Modifier.fillMaxWidth().height(1.dp).background(c.line2))

            MenuRow("Rename", { RenameIcon(c.ink3, size = 18.dp) }) {
                state.eventSink(ShellScreen.Event.Write.RenameStarted)
            }
            MenuRow("Delete", { DeleteIcon(c.against, size = 18.dp) }, tint = c.against) {
                state.eventSink(ShellScreen.Event.Write.DeleteRequested)
            }
        }
    }
}

/** The sidebar's + menu: what can be made, and where it would land. */
@Composable
private fun CreateMenu(state: ShellScreen.State) {
    val c = AnchorTheme.colors
    val dismiss = { state.eventSink(ShellScreen.Event.Chrome.ToggleCreate) }

    Scrim(dismiss)
    Column(
        Modifier
            .fillMaxSize()
            .safeContent()
            .padding(start = 12.dp, top = 74.dp),
    ) {
        Column(
            Modifier
                .width(AnchorTheme.metrics.pathPopoverWidth)
                .clip(AnchorShape.popover)
                .background(c.card)
                .border(1.dp, c.line, AnchorShape.popover)
                .padding(bottom = 8.dp),
        ) {
            PopoverLabel("New in ${state.createTarget}")
            MenuRow("Folder", { NewFolderIcon(c.amber, size = 18.dp) }) {
                state.eventSink(ShellScreen.Event.Write.CreateFolder)
            }
            MenuRow("Page", { NewPageIcon(c.ink3, size = 18.dp) }) {
                state.eventSink(ShellScreen.Event.Write.CreatePage)
            }
        }
    }
}

@Composable
private fun MenuRow(
    label: String,
    icon: @Composable () -> Unit,
    tint: Color = AnchorTheme.colors.ink2,
    onClick: () -> Unit,
) {
    Row(
        Modifier
            .fillMaxWidth()
            .heightIn(min = 50.dp)
            .padding(horizontal = 8.dp)
            .clip(AnchorShape.menuRow)
            .clickable(onClick = onClick)
            .padding(horizontal = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        icon()
        Text(label, style = MaterialTheme.typography.bodyLarge, color = tint, maxLines = 1)
    }
}

/**
 * Rename. Opens focused with the text selected, because renaming is nearly always
 * replacing rather than editing — a new folder arrives here already called "New folder".
 */
@Composable
private fun RenameDialog(state: ShellScreen.State, draft: ShellScreen.RenameDraft) {
    val c = AnchorTheme.colors
    val focus = remember { FocusRequester() }
    var field by remember(draft.node.id) {
        mutableStateOf(
            TextFieldValue(draft.title, selection = TextRange(0, draft.title.length)),
        )
    }

    LaunchedEffect(draft.node.id) { focus.requestFocus() }

    Scrim { state.eventSink(ShellScreen.Event.Write.RenameCancelled) }
    Box(
        Modifier.fillMaxSize().safeContent().imePadding(),
        contentAlignment = Alignment.Center,
    ) {
        Column(
            Modifier
                .width(420.dp)
                .clip(AnchorShape.card)
                .background(c.card)
                .border(1.dp, c.line, AnchorShape.card)
                .padding(20.dp),
        ) {
            Text(
                "Rename ${if (draft.node.isContainer) "folder" else "artifact"}",
                style = MaterialTheme.typography.titleMedium,
                color = c.ink,
            )
            Spacer(Modifier.height(14.dp))
            Box(
                Modifier
                    .fillMaxWidth()
                    .height(46.dp)
                    .clip(AnchorShape.field)
                    .background(c.field)
                    .padding(horizontal = 14.dp),
                contentAlignment = Alignment.CenterStart,
            ) {
                BasicTextField(
                    value = field,
                    onValueChange = {
                        field = it
                        state.eventSink(ShellScreen.Event.Write.RenameEdited(it.text))
                    },
                    singleLine = true,
                    enabled = !draft.saving,
                    textStyle = MaterialTheme.typography.bodyLarge.copy(color = c.ink),
                    cursorBrush = SolidColor(c.amber),
                    keyboardOptions = KeyboardOptions(imeAction = ImeAction.Done),
                    keyboardActions = KeyboardActions(
                        onDone = { state.eventSink(ShellScreen.Event.Write.RenameCommitted) },
                    ),
                    modifier = Modifier.fillMaxWidth().focusRequester(focus),
                )
            }
            Spacer(Modifier.height(16.dp))
            Row(
                Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(10.dp, Alignment.End),
            ) {
                QuietButton("Cancel") { state.eventSink(ShellScreen.Event.Write.RenameCancelled) }
                AmberButton(if (draft.saving) "Saving…" else "Rename", enabled = !draft.saving) {
                    state.eventSink(ShellScreen.Event.Write.RenameCommitted)
                }
            }
        }
    }
}

/**
 * Delete asks first, and says what goes with it. The server soft-deletes the whole subtree,
 * so "delete this folder" is never only about the folder.
 */
@Composable
private fun DeleteConfirm(state: ShellScreen.State, node: WorkspaceNode) {
    val c = AnchorTheme.colors

    Scrim { state.eventSink(ShellScreen.Event.Write.DeleteCancelled) }
    Box(Modifier.fillMaxSize().safeContent(), contentAlignment = Alignment.Center) {
        Column(
            Modifier
                .width(420.dp)
                .clip(AnchorShape.card)
                .background(c.card)
                .border(1.dp, c.line, AnchorShape.card)
                .padding(20.dp),
        ) {
            Text(
                "Delete “${node.title.ifBlank { "Untitled" }}”?",
                style = MaterialTheme.typography.titleMedium,
                color = c.ink,
            )
            Spacer(Modifier.height(8.dp))
            Text(
                if (node.isContainer) {
                    "Everything inside it goes too. It is recoverable on the desktop, " +
                        "but not from here."
                } else {
                    "It is recoverable on the desktop, but not from here."
                },
                style = MetaStyle,
                color = c.ink4,
            )
            Spacer(Modifier.height(18.dp))
            Row(
                Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(10.dp, Alignment.End),
            ) {
                QuietButton("Cancel") { state.eventSink(ShellScreen.Event.Write.DeleteCancelled) }
                DangerButton("Delete") { state.eventSink(ShellScreen.Event.Write.DeleteConfirmed) }
            }
        }
    }
}

/** A refused write, said plainly and left up until dismissed rather than flashed past. */
@Composable
private fun WriteErrorToast(state: ShellScreen.State, message: String) {
    val c = AnchorTheme.colors

    Box(Modifier.fillMaxSize().safeContent(), contentAlignment = Alignment.BottomCenter) {
        Row(
            Modifier
                .padding(24.dp)
                .widthIn(max = 520.dp)
                .clip(AnchorShape.card)
                .background(c.raise)
                .border(1.dp, c.line, AnchorShape.card)
                .clickable { state.eventSink(ShellScreen.Event.Write.WriteErrorDismissed) }
                .padding(horizontal = 18.dp, vertical = 14.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Text(
                message,
                style = MaterialTheme.typography.bodyLarge,
                color = c.against,
                modifier = Modifier.weight(1f, fill = false),
            )
            CloseIcon(c.ink4, size = 14.dp)
        }
    }
}

@Composable
private fun AmberButton(label: String, enabled: Boolean = true, onClick: () -> Unit) {
    val c = AnchorTheme.colors
    Box(
        Modifier
            .heightIn(min = 44.dp)
            .clip(AnchorShape.field)
            .background(if (enabled) c.amber else c.field)
            .clickable(enabled = enabled, onClick = onClick)
            .padding(horizontal = 20.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            label,
            style = MaterialTheme.typography.titleMedium,
            color = if (enabled) c.onAmber else c.ink4,
        )
    }
}

@Composable
private fun DangerButton(label: String, onClick: () -> Unit) {
    val c = AnchorTheme.colors
    Box(
        Modifier
            .heightIn(min = 44.dp)
            .clip(AnchorShape.field)
            .background(c.field)
            .border(1.dp, c.against.copy(alpha = 0.4f), AnchorShape.field)
            .clickable(onClick = onClick)
            .padding(horizontal = 20.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(label, style = MaterialTheme.typography.titleMedium, color = c.against)
    }
}

@Composable
private fun QuietButton(label: String, onClick: () -> Unit) {
    val c = AnchorTheme.colors
    Box(
        Modifier
            .heightIn(min = 44.dp)
            .clip(AnchorShape.field)
            .clickable(onClick = onClick)
            .padding(horizontal = 18.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(label, style = MaterialTheme.typography.titleMedium, color = c.ink3)
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// Column browser — the whole tree, one column per level
// ─────────────────────────────────────────────────────────────────────────────

/**
 * Miller columns, as a modal. Desktop offers the same thing from a folder's context menu
 * ("Browse in columns"), and it is the right answer to the same problem here: a popover
 * that lists a handful of siblings can only ever show part of the tree, and the row you
 * want is as likely as not the one it cut.
 *
 * Each column is a folder's contents. Tapping a folder opens the column to its right and
 * the browser stays put; tapping an artifact is a destination, so it opens and the browser
 * gets out of the way.
 */
@Composable
private fun ColumnBrowser(state: ShellScreen.State) {
    val c = AnchorTheme.colors
    val scroll = rememberScrollState()

    // A new column appearing off the right edge would be invisible, so the browser follows
    // the descent the way a Finder column view does.
    LaunchedEffect(state.browseColumns.size) { scroll.animateScrollTo(scroll.maxValue) }

    Scrim { state.eventSink(ShellScreen.Event.Chrome.ToggleBrowse) }
    Box(Modifier.fillMaxSize().safeContent().padding(24.dp), contentAlignment = Alignment.Center) {
        Column(
            Modifier
                .fillMaxSize()
                .clip(AnchorShape.card)
                .background(c.panel)
                .border(1.dp, c.line, AnchorShape.card),
        ) {
            Row(
                Modifier.fillMaxWidth().padding(start = 20.dp, end = 12.dp, top = 14.dp, bottom = 12.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    "Browse",
                    style = MaterialTheme.typography.headlineMedium,
                    color = c.ink,
                    modifier = Modifier.weight(1f),
                )
                SmallButton(onClick = { state.eventSink(ShellScreen.Event.Chrome.ToggleBrowse) }) {
                    CloseIcon(c.ink3, size = 15.dp)
                }
            }
            Box(Modifier.fillMaxWidth().height(1.dp).background(c.line))

            Row(Modifier.fillMaxWidth().weight(1f).horizontalScroll(scroll)) {
                state.browseColumns.forEachIndexed { depth, column ->
                    BrowseColumnView(state, depth, column)
                    VerticalHairline()
                }
            }
        }
    }
}

@Composable
private fun BrowseColumnView(
    state: ShellScreen.State,
    depth: Int,
    column: ShellScreen.BrowseColumn,
) {
    val c = AnchorTheme.colors

    Column(Modifier.width(268.dp).fillMaxHeight()) {
        Text(
            column.title.ifBlank { "Untitled" }.uppercase(),
            style = SectionLabelStyle,
            color = c.ink4,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.padding(start = 16.dp, end = 16.dp, top = 14.dp, bottom = 8.dp),
        )
        if (column.items.isEmpty()) {
            EmptyLine("empty")
        } else {
            LazyColumn(
                Modifier.fillMaxWidth().weight(1f).padding(horizontal = 8.dp),
                contentPadding = PaddingValues(bottom = 12.dp),
                verticalArrangement = Arrangement.spacedBy(2.dp),
            ) {
                items(column.items, key = { it.id }) { node ->
                    BrowseRow(
                        node = node,
                        selected = node.id == column.selectedId,
                        onClick = {
                            state.eventSink(ShellScreen.Event.Nav.BrowsePicked(depth, node))
                        },
                        onLongClick = {
                            state.eventSink(ShellScreen.Event.Write.ActionsRequested(node))
                        },
                    )
                }
            }
        }
    }
}

@Composable
private fun BrowseRow(
    node: WorkspaceNode,
    selected: Boolean,
    onClick: () -> Unit,
    onLongClick: () -> Unit,
) {
    val c = AnchorTheme.colors

    Row(
        Modifier
            .fillMaxWidth()
            .heightIn(min = 46.dp)
            .clip(AnchorShape.menuRow)
            .background(if (selected) c.sel else Color.Transparent)
            .combinedClickable(onClick = onClick, onLongClick = onLongClick)
            .padding(horizontal = 11.dp, vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        artifactKindIcon(
            artifactType = node.artifactType,
            isContainer = node.isContainer,
            tint = if (node.isContainer) c.amber else c.ink3,
            size = 17.dp,
        )
        Text(
            node.title.ifBlank { "Untitled" },
            style = MaterialTheme.typography.bodyLarge,
            color = if (selected) c.ink else c.ink2,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.weight(1f),
        )
        // Only containers advertise a next column; an artifact ends the path.
        if (node.isContainer) ChevronIcon(c.ink4, size = 14.dp)
    }
}

@Composable
private fun VerticalHairline() {
    Box(Modifier.width(1.dp).fillMaxHeight().background(AnchorTheme.colors.line))
}

/**
 * Applied to a column's content *after* its background, so the fill reaches the screen
 * edges while nothing readable sits under the clock or the home indicator.
 *
 * **The keyboard is excluded here and handled per surface**, because the right answer
 * differs by column and a blanket rule got it wrong in both directions:
 *
 *  - The **detail** column must NOT move. It hosts the editor, and WebKit scrolls the caret
 *    into view itself; padding the column too made the Compose pane slide while the web
 *    content did its own thing — two layers reacting to one keyboard.
 *  - The **sidebar** and **copilot** pane MUST compress, or their lower half (the list's
 *    tail, the copilot hero) sits behind the keyboard and cannot be reached at all.
 *
 * So `safeDrawing` minus the IME here, and an explicit `imePadding()` on the surfaces that
 * need to give way — including the web view itself, which is the detail column's answer.
 */
@Composable
private fun Modifier.safeContent(): Modifier =
    windowInsetsPadding(
        WindowInsets.safeDrawing.exclude(WindowInsets.ime).only(WindowInsetsSides.Vertical),
    )

// ─────────────────────────────────────────────────────────────────────────────
// Sidebar — the whole navigation model lives here in rev 2
// ─────────────────────────────────────────────────────────────────────────────

@Composable
private fun Sidebar(state: ShellScreen.State, modifier: Modifier = Modifier) {
    val c = AnchorTheme.colors

    Box(modifier.fillMaxHeight().background(c.panel)) {
        // imePadding: with the keyboard up (typically because the editor on the right has
        // focus) the sidebar must stay usable rather than running on underneath it.
        Column(Modifier.fillMaxSize().safeContent().imePadding()) {
            SidebarHeader(state)
            SidebarList(state, Modifier.weight(1f))
            CopilotHero(state)
        }

        if (state.pathOpen) {
            PathPopover(state, Modifier.align(Alignment.TopStart))
        }
    }
}

@Composable
private fun SidebarHeader(state: ShellScreen.State) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    Column(Modifier.fillMaxWidth().padding(start = 18.dp, end = 18.dp, top = 10.dp, bottom = 10.dp)) {
        Row(
            Modifier.fillMaxWidth().heightIn(min = m.smallControl),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(6.dp),
        ) {
            // Back is a plain chevron with the parent only in its tooltip — rev 2 drops the
            // breadcrumb, so the label lives in the path popover instead.
            if (state.nav.canBack) {
                SmallButton(onClick = { state.eventSink(ShellScreen.Event.Nav.Back) }) {
                    ChevronIcon(c.amber, size = 20.dp, direction = ChevronDirection.Left)
                }
            }

            // The title is a button: tapping it answers "where am I". It takes ALL the
            // leftover width — an extra weighted spacer here would split the row and
            // ellipsise a title that fits perfectly well.
            Row(
                Modifier
                    .weight(1f)
                    .clip(AnchorShape.chip)
                    .background(if (state.pathOpen) c.sel else Color.Transparent)
                    .clickable { state.eventSink(ShellScreen.Event.Chrome.TogglePath) }
                    .padding(start = 6.dp, end = 6.dp, top = 4.dp, bottom = 4.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(6.dp),
            ) {
                Text(
                    state.sidebarTitle,
                    style = MaterialTheme.typography.headlineMedium,
                    color = c.ink,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.weight(1f, fill = false),
                )
                ChevronIcon(c.ink3, size = 15.dp, direction = ChevronDirection.Down)
            }

            // The count yields before the title does — it is the least useful thing here.
            if (state.sidebarCount.isNotEmpty()) {
                Text(state.sidebarCount, style = MetaStyle, color = c.ink4, maxLines = 1)
            }
            // New goes where you are: at root there is no folder to put anything in, so
            // the affordance simply isn't there rather than being there and failing.
            if (!state.nav.isRoot) {
                SmallButton(onClick = { state.eventSink(ShellScreen.Event.Chrome.ToggleCreate) }) {
                    PlusIcon(if (state.createOpen) c.amber else c.ink3, size = 19.dp)
                }
            }
            SmallButton(onClick = { state.eventSink(ShellScreen.Event.Chrome.ToggleSidebar) }) {
                ColumnsIcon(c.ink3, size = 19.dp)
            }
        }

        Spacer(Modifier.height(11.dp))
        SearchField(
            query = state.query,
            placeholder = state.searchPlaceholder,
            onQueryChange = { state.eventSink(ShellScreen.Event.Nav.QueryChanged(it)) },
        )
    }
}

/**
 * The stack's three levels. Each push/pop replays a slide, which — with the Back chevron
 * and the path popover — is all the spatial awareness rev 2 gives you; there is no
 * breadcrumb to fall back on, so the animation is load-bearing rather than decorative.
 */
@Composable
private fun SidebarList(state: ShellScreen.State, modifier: Modifier = Modifier) {
    val depth = state.nav.drill.size + if (state.nav.isRoot) 0 else 1

    AnimatedContentByDepth(depth = depth, modifier = modifier) {
        LazyColumn(
            Modifier.fillMaxSize().padding(horizontal = 10.dp),
            contentPadding = PaddingValues(bottom = 14.dp),
            verticalArrangement = Arrangement.spacedBy(2.dp),
        ) {
            when (state.level) {
                SidebarNav.Level.Root -> items(state.destinations, key = { it.id }) { destination ->
                    SectionRow(
                        destination = destination,
                        onClick = { state.eventSink(ShellScreen.Event.Nav.SectionOpened(destination)) },
                    )
                }

                SidebarNav.Level.Section -> {
                    if (state.rows.isEmpty()) item { EmptyLine(emptyCopy(state)) }
                    items(state.rows, key = { it.id }) { node ->
                        BrowserRow(
                            node = node,
                            selected = node.id == state.nav.currentId,
                            onClick = { state.eventSink(ShellScreen.Event.Nav.RowSelected(node)) },
                            onLongClick = {
                                state.eventSink(ShellScreen.Event.Write.ActionsRequested(node))
                            },
                        )
                    }
                }

                SidebarNav.Level.Drill -> {
                    if (state.contents.isEmpty()) item { EmptyLine(emptyCopy(state)) }
                    items(state.contents, key = { it.id }) { node ->
                        DrillRow(
                            node = node,
                            selected = node.id == state.nav.currentId,
                            onClick = {
                                if (node.isContainer) {
                                    state.eventSink(ShellScreen.Event.Nav.DrilledInto(node))
                                } else {
                                    state.eventSink(ShellScreen.Event.Nav.RowSelected(node))
                                }
                            },
                            onLongClick = {
                                state.eventSink(ShellScreen.Event.Write.ActionsRequested(node))
                            },
                        )
                    }
                }
            }
        }
    }
}

/**
 * A plain cross-fade between levels. The handoff leans on a push/pop slide as its depth
 * cue, but a full-width slide on every tap reads as the pane jumping — and Back plus the
 * path popover already say where you are. A short fade marks the change without narrating
 * it.
 */
@Composable
private fun AnimatedContentByDepth(
    depth: Int,
    modifier: Modifier = Modifier,
    content: @Composable () -> Unit,
) {
    Crossfade(
        targetState = depth,
        animationSpec = tween(AladinMetrics.SIDEBAR_LEVEL_MILLIS),
        label = "sidebar-stack",
        modifier = modifier,
    ) { _ -> content() }
}

@Composable
private fun SectionRow(destination: Destination, onClick: () -> Unit) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    Row(
        Modifier
            .fillMaxWidth()
            .height(m.rootRowHeight)
            .clip(AnchorShape.menuRow)
            .clickable(onClick = onClick)
            .padding(horizontal = 14.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(13.dp),
    ) {
        Box(Modifier.size(24.dp), contentAlignment = Alignment.Center) {
            destinationIcon(destination = destination, tint = c.ink3, size = 22.dp)
        }
        Text(
            destination.title,
            style = MaterialTheme.typography.titleMedium,
            color = c.ink2,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
    }
}

/**
 * A section's rows: folders and artifacts as siblings, as desktop presents them.
 *
 * Long press is the touch equivalent of desktop's right-click context menu — the same
 * rename/delete/create-here vocabulary, reached the way a touch surface reaches it.
 */
@Composable
private fun BrowserRow(
    node: WorkspaceNode,
    selected: Boolean,
    onClick: () -> Unit,
    onLongClick: () -> Unit,
) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    Row(
        Modifier
            .fillMaxWidth()
            .heightIn(min = m.listRowMinHeight)
            .clip(AnchorShape.row)
            .background(if (selected) c.sel else Color.Transparent)
            .combinedClickable(onClick = onClick, onLongClick = onLongClick)
            .padding(horizontal = 13.dp, vertical = 11.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(11.dp),
    ) {
        Box(Modifier.size(22.dp), contentAlignment = Alignment.Center) {
            artifactKindIcon(
                artifactType = node.artifactType,
                isContainer = node.isContainer,
                // A folder is amber because of what it is — the design states the role once.
                tint = if (node.isContainer) c.amber else c.ink3,
            )
        }
        Column(Modifier.weight(1f)) {
            Text(
                node.title.ifBlank { "Untitled" },
                style = MaterialTheme.typography.titleMedium,
                color = if (selected) c.ink else c.ink2,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )
            val chip = chipFor(node)
            if (chip != null) {
                Spacer(Modifier.height(5.dp))
                Chip(chip, node.kind)
            }
        }
        if (node.isContainer) {
            ChevronIcon(c.ink4, size = 16.dp)
        }
    }
}

/** A row inside a folder's contents. Subfolders push; artifacts open. */
@Composable
private fun DrillRow(
    node: WorkspaceNode,
    selected: Boolean,
    onClick: () -> Unit,
    onLongClick: () -> Unit,
) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    Row(
        Modifier
            .fillMaxWidth()
            .heightIn(min = m.drillRowMinHeight)
            .clip(AnchorShape.row)
            // Selection is visible in here too now: opening a note no longer leaves the
            // folder, so the list has to be able to say which of its rows you are reading.
            .background(if (selected) c.sel else Color.Transparent)
            .combinedClickable(onClick = onClick, onLongClick = onLongClick)
            .padding(horizontal = 13.dp, vertical = 9.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Box(Modifier.size(22.dp), contentAlignment = Alignment.Center) {
            artifactKindIcon(
                artifactType = node.artifactType,
                isContainer = node.isContainer,
                tint = if (node.isContainer) c.amber else c.ink3,
            )
        }
        Column(Modifier.weight(1f)) {
            Text(
                node.title.ifBlank { "Untitled" },
                style = MaterialTheme.typography.titleMedium,
                color = if (selected) c.ink else c.ink2,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            metaFor(node)?.let {
                Spacer(Modifier.height(2.dp))
                Text(it, style = MetaStyle, color = c.ink4, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
        }
        ChevronIcon(c.ink4, size = 16.dp)
    }
}

@Composable
private fun EmptyLine(text: String) {
    Text(
        text,
        style = MetaStyle,
        color = AnchorTheme.colors.ink4,
        modifier = Modifier.padding(horizontal = 10.dp, vertical = 6.dp),
    )
}

private fun emptyCopy(state: ShellScreen.State): String = when {
    state.query.isNotBlank() -> "nothing matches “${state.query}”"
    state.sync is SyncStatus.Offline -> "offline — showing what was cached"
    state.level == SidebarNav.Level.Drill -> "this folder is empty"
    state.nav.destination == Destination.Folders -> "nothing synced yet"
    else -> "no list for this section yet"
}

@Composable
private fun Chip(label: String, kind: NodeKind) {
    val c = AnchorTheme.colors
    val (fg, bg) = when (kind) {
        NodeKind.Research -> c.amber to c.amberSoft
        NodeKind.Folder -> c.ink2 to c.hover
        NodeKind.Artifact -> c.ink2 to c.hover
    }
    Box(Modifier.clip(AnchorShape.pill).background(bg).padding(horizontal = 8.dp, vertical = 3.dp)) {
        Text(label.uppercase(), style = ChipStyle, color = fg)
    }
}

/** A plain folder gets no chip — "folder" on a folder is noise (handoff). */
private fun chipFor(node: WorkspaceNode): String? = when (node.kind) {
    NodeKind.Research -> "research"
    NodeKind.Artifact -> node.artifactType
    NodeKind.Folder -> null
}

private fun metaFor(node: WorkspaceNode): String? = when {
    node.isContainer -> "folder"
    else -> node.artifactType
}

// ─────────────────────────────────────────────────────────────────────────────
// Path popover — rev 2's replacement for the breadcrumb
// ─────────────────────────────────────────────────────────────────────────────

@Composable
private fun PathPopover(state: ShellScreen.State, modifier: Modifier = Modifier) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    // Backdrop first, so a tap anywhere else dismisses.
    Box(
        Modifier
            .fillMaxSize()
            .clickable(indication = null, interactionSource = null) {
                state.eventSink(ShellScreen.Event.Chrome.TogglePath)
            },
    )

    Column(
        modifier
            .padding(start = 12.dp, top = 84.dp)
            .width(m.pathPopoverWidth)
            .heightIn(max = 560.dp)
            .clip(AnchorShape.popover)
            .background(c.card)
            .border(1.dp, c.line, AnchorShape.popover)
            .safeContent(),
    ) {
        PopoverLabel("You are in")
        Column(Modifier.padding(horizontal = 8.dp, vertical = 2.dp)) {
            state.pathLevels.forEachIndexed { index, level ->
                PathRow(level) { state.eventSink(ShellScreen.Event.Nav.JumpToLevel(index)) }
            }
        }

        if (state.browseColumns.isNotEmpty()) {
            Spacer(Modifier.height(8.dp))
            Box(Modifier.fillMaxWidth().height(1.dp).background(c.line2))
            // What used to be here was a "Switch to" list of at most six siblings of one
            // folder — a navigation aid that hid the very row you were looking for. The
            // column browser shows the whole tree instead, so nothing is truncated away.
            Box(Modifier.padding(horizontal = 8.dp, vertical = 8.dp)) {
                Row(
                    Modifier
                        .fillMaxWidth()
                        .height(44.dp)
                        .clip(AnchorShape.chip)
                        .clickable { state.eventSink(ShellScreen.Event.Chrome.ToggleBrowse) }
                        .padding(horizontal = 10.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(11.dp),
                ) {
                    ColumnsIcon(c.amber, size = 17.dp)
                    Text(
                        "Browse in columns",
                        style = MaterialTheme.typography.bodyLarge,
                        color = c.ink2,
                        maxLines = 1,
                        modifier = Modifier.weight(1f),
                    )
                    ChevronIcon(c.ink4, size = 14.dp)
                }
            }
        } else {
            Spacer(Modifier.height(10.dp))
        }
    }
}

@Composable
private fun PopoverLabel(text: String) {
    Text(
        text.uppercase(),
        style = SectionLabelStyle,
        color = AnchorTheme.colors.ink4,
        modifier = Modifier.padding(start = 16.dp, end = 16.dp, top = 14.dp, bottom = 8.dp),
    )
}

@Composable
private fun PathRow(level: PathLevel, onClick: () -> Unit) {
    val c = AnchorTheme.colors

    Row(
        Modifier
            .fillMaxWidth()
            .height(42.dp)
            .clip(AnchorShape.chip)
            .background(if (level.isCurrent) c.sel else Color.Transparent)
            .clickable(onClick = onClick)
            .padding(horizontal = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        // Indentation is the whole depth cue here — the popover has no tree lines.
        Spacer(Modifier.width((level.depth * 14).dp))
        ChevronIcon(if (level.isCurrent) c.ink else c.ink2, size = 14.dp)
        Text(
            level.title,
            style = MaterialTheme.typography.bodyLarge,
            color = if (level.isCurrent) c.ink else c.ink2,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.weight(1f),
        )
        Text(
            if (level.isCurrent) "· here" else level.tag,
            style = MetaStyle,
            color = c.ink4,
            maxLines = 1,
        )
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// Copilot hero — the sidebar's foot
// ─────────────────────────────────────────────────────────────────────────────

/**
 * Full at root, mini once you are in a list — auto, unless you pin it either way. The mini
 * capsule's press states are inset rounded pills, never square, so the segments read as
 * one control rather than three buttons wedged together.
 */
@Composable
private fun CopilotHero(state: ShellScreen.State) {
    val size = tween<IntSize>(
        durationMillis = AladinMetrics.HERO_COLLAPSE_MILLIS,
        easing = CubicBezierEasing(0.2f, 0.8f, 0.2f, 1f),
    )

    Box(Modifier.padding(start = 14.dp, end = 14.dp, top = 10.dp, bottom = 14.dp)) {
        // The card and the capsule are different shapes, not one resizing box, so the
        // height animates while the contents cross-fade. Collapsing leads with the fade so
        // the full card's text is gone before the capsule's width is reached — otherwise
        // "Ask a question" visibly squashes into "Ask".
        AnimatedContent(
            targetState = state.heroExpanded,
            transitionSpec = {
                ContentTransform(
                    targetContentEnter = fadeIn(tween(HERO_FADE_IN, delayMillis = HERO_FADE_OUT)),
                    initialContentExit = fadeOut(tween(HERO_FADE_OUT)),
                    sizeTransform = SizeTransform(clip = true) { _, _ -> size },
                )
            },
            label = "copilot-hero",
        ) { expanded ->
            if (expanded) HeroFull(state) else HeroMini(state)
        }
    }
}

private const val HERO_FADE_OUT = 110
private const val HERO_FADE_IN = 160

@Composable
private fun heroBrush(): Brush {
    val c = AnchorTheme.colors
    return Brush.linearGradient(listOf(c.amberSoft, c.amber.copy(alpha = 0.02f)))
}

@Composable
private fun HeroFull(state: ShellScreen.State) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    Column(
        Modifier
            .fillMaxWidth()
            .height(m.heroFullHeight)
            .clip(AnchorShape.heroCard)
            .background(heroBrush())
            .border(1.dp, c.amberLine, AnchorShape.heroCard)
            .padding(18.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(9.dp)) {
            SparkleIcon(c.amber, size = 19.dp)
            Text("COPILOT", style = SectionLabelStyle, color = c.amber)
            Spacer(Modifier.weight(1f))
            SmallButton(size = 34.dp, onClick = {}) { BellIcon(c.ink3, size = 18.dp) }
            SmallButton(size = 34.dp, onClick = { state.eventSink(ShellScreen.Event.Chrome.ToggleHero) }) {
                ChevronIcon(c.ink3, size = 17.dp, direction = ChevronDirection.Down)
            }
        }

        Spacer(Modifier.height(14.dp))
        Text(
            heroLine(state),
            style = MaterialTheme.typography.titleLarge,
            color = c.ink,
        )
        Spacer(Modifier.height(8.dp))
        Text(
            "Answers point back to the page they came from.",
            style = MaterialTheme.typography.bodyMedium,
            color = c.ink2,
        )

        Spacer(Modifier.weight(1f))

        Row(
            Modifier
                .fillMaxWidth()
                .height(48.dp)
                .clip(AnchorShape.row)
                .background(c.amber)
                .clickable { state.eventSink(ShellScreen.Event.Chrome.OpenCopilot) }
                .padding(start = 15.dp, end = 14.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                "Ask a question",
                style = MaterialTheme.typography.titleMedium,
                color = c.onAmber,
                modifier = Modifier.weight(1f),
            )
            ChevronIcon(c.onAmber, size = 18.dp)
        }

        Spacer(Modifier.height(10.dp))

        Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
            Row(
                Modifier
                    .weight(1f)
                    .height(46.dp)
                    .clip(AnchorShape.menuRow)
                    .background(if (state.switcherOpen) c.sel else Color.Transparent)
                    .clickable { state.eventSink(ShellScreen.Event.Chrome.ToggleSwitcher) }
                    .padding(horizontal = 12.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(9.dp),
            ) {
                OpenItemsIcon(c.ink2, size = 18.dp)
                Text(
                    "Open",
                    style = MaterialTheme.typography.bodyLarge,
                    color = c.ink2,
                    modifier = Modifier.weight(1f),
                )
                CountBadge(state.openItems.size)
            }
            AvatarButton(state.signedInAs)
        }
    }
}

@Composable
private fun HeroMini(state: ShellScreen.State) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    Row(
        Modifier
            .fillMaxWidth()
            .height(m.heroMiniHeight)
            .clip(AnchorShape.capsule)
            .background(heroBrush())
            .border(1.dp, c.amberLine, AnchorShape.capsule),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Row(
            Modifier
                .weight(1f)
                .padding(start = 5.dp, end = 3.dp, top = 5.dp, bottom = 5.dp)
                .fillMaxHeight()
                .clip(AnchorShape.chip)
                .clickable { state.eventSink(ShellScreen.Event.Chrome.OpenCopilot) }
                .padding(horizontal = 11.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(9.dp),
        ) {
            SparkleIcon(c.amber, size = 19.dp)
            Text(
                "Ask",
                style = MaterialTheme.typography.titleMedium,
                color = c.amber,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
        SegmentDivider()
        Row(
            Modifier
                .padding(vertical = 5.dp, horizontal = 3.dp)
                .fillMaxHeight()
                .clip(AnchorShape.chip)
                .clickable { state.eventSink(ShellScreen.Event.Chrome.ToggleSwitcher) }
                .padding(horizontal = 11.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            OpenItemsIcon(c.ink2, size = 18.dp)
            CountBadge(state.openItems.size)
        }
        SegmentDivider()
        Box(
            Modifier
                .padding(start = 3.dp, end = 5.dp, top = 5.dp, bottom = 5.dp)
                .width(40.dp)
                .fillMaxHeight()
                .clip(AnchorShape.chip)
                .clickable { state.eventSink(ShellScreen.Event.Chrome.ToggleHero) },
            contentAlignment = Alignment.Center,
        ) {
            ChevronIcon(c.ink3, size = 17.dp, direction = ChevronDirection.Up)
        }
    }
}

@Composable
private fun SegmentDivider() {
    Box(Modifier.width(1.dp).height(32.dp).background(AnchorTheme.colors.amberLine))
}

@Composable
private fun CountBadge(count: Int) {
    val c = AnchorTheme.colors
    Box(
        Modifier
            .widthIn(min = 19.dp)
            .height(19.dp)
            .clip(AnchorShape.pill)
            .background(c.amber)
            .padding(horizontal = 5.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text("$count", style = ChipStyle, color = c.onAmber)
    }
}

@Composable
private fun AvatarButton(signedInAs: String?) {
    val c = AnchorTheme.colors
    Box(
        Modifier
            .size(46.dp)
            .clip(AnchorShape.pill)
            .background(c.field)
            .border(1.dp, c.line, AnchorShape.pill),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            signedInAs?.firstOrNull()?.uppercase() ?: "?",
            style = MaterialTheme.typography.titleMedium,
            color = c.ink2,
        )
    }
}

private fun heroLine(state: ShellScreen.State): String = when (state.level) {
    SidebarNav.Level.Root -> "Ask across everything you have open."
    else -> "Ask about ${state.sidebarTitle.lowercase()}."
}

@Composable
private fun SmallButton(
    onClick: () -> Unit,
    size: androidx.compose.ui.unit.Dp = AladinMetrics.smallControl,
    content: @Composable () -> Unit,
) {
    Box(
        Modifier.size(size).clip(AnchorShape.chip).clickable(onClick = onClick),
        contentAlignment = Alignment.Center,
    ) { content() }
}

// ─────────────────────────────────────────────────────────────────────────────
// Detail
// ─────────────────────────────────────────────────────────────────────────────

@Composable
private fun DetailColumn(state: ShellScreen.State, modifier: Modifier = Modifier) {
    val c = AnchorTheme.colors

    // The one web view, created ABOVE the pager. This is the whole reason the pager is
    // usable here: the pager disposes pages it has scrolled away from, and this object must
    // never be disposed — it holds every open document's session and undo history. The web
    // page attaches it; nothing owns it but this scope.
    val webHost = rememberWebSurfaceHost(
        token = state.webHost.token,
        userName = state.webHost.userName,
        config = state.webHost.api,
    )

    Column(modifier.fillMaxHeight().background(c.bg).safeContent()) {
        DetailHeader(state)
        // weight, NOT fillMaxSize: a non-weighted child takes the column's *full* height,
        // not what is left under the header — so the scrolling body would be one header
        // taller than its viewport and the last 72dp would sit below the screen with no
        // way to reach it.
        Box(Modifier.weight(1f).fillMaxWidth()) {
            if (state.pages.isEmpty()) {
                EmptyDetail(state)
            } else {
                DetailPager(state, webHost)
            }
        }
    }
}

/**
 * The detail pager — used as a **recycler, not a gesture**.
 *
 * Residency is entirely the pager's: it composes the pages either side of the current one
 * and disposes the rest, and a disposed surface rebuilds from its snapshot. That replaced a
 * hand-rolled keep-alive list, a per-class retention budget and three separate ways of
 * hiding a surface — all of which existed to reimplement, badly, what this does correctly.
 *
 * Nothing is ever hidden. Exactly one page is on screen, so the overdraw, keying and reorder
 * hazards that came with a stack of alpha-hidden siblings cannot arise.
 *
 * **Dragging is off, deliberately.** The pages are not the open items: every web-backed
 * artifact shares one page, so swipe order is not open order, and a swipe onto the web page
 * would land on whichever document happened to be active last — arbitrary from the user's
 * side. A gesture that cannot describe the model is worse than no gesture, so selection
 * drives the pager and never the other way round. The stepper and the switcher remain the
 * ways to move between open things.
 */
@Composable
private fun DetailPager(state: ShellScreen.State, webHost: WebSurfaceHost?) {
    val target = state.activePage.takeIf { it in state.pages.indices } ?: -1
    // Both of these are read from lambdas that outlive this composition — the page-count
    // lambda is cached inside PagerState, and the collector below runs in a coroutine keyed
    // on the pager alone. Capturing `state` directly would freeze both at whatever the shell
    // looked like on first composition: opening the first item would leave the pager
    // believing it still had zero pages, and the header would never change again.
    val current by rememberUpdatedState(state)
    val pageCount by rememberUpdatedState(state.pages.size)
    val pagerState = rememberPagerState(initialPage = target.coerceAtLeast(0)) { pageCount }


    // Selection → pager, the only direction there is. A target of -1 means the selection is
    // not open as a page at all (it was just closed, or deleted elsewhere); staying put is
    // better than the coerce-to-zero that used to silently throw you onto the web page.
    LaunchedEffect(target, state.pages.size) {
        if (target >= 0 && pagerState.currentPage != target) pagerState.animateScrollToPage(target)
    }

    HorizontalPager(
        state = pagerState,
        // One page either side stays composed, so a swipe reveals a live surface rather
        // than one being built under the finger. Everything beyond is disposed.
        beyondViewportPageCount = 1,
        userScrollEnabled = false,
        modifier = Modifier.fillMaxSize(),
    ) { index ->
        when (val page = state.pages[index]) {
            ShellScreen.Page.Web -> webHost?.let {
                WebSurfacePage(
                    host = it,
                    panes = state.webHost.panes,
                    activeId = state.webHost.activeId,
                )
            }

            is ShellScreen.Page.Artifact -> ArtifactSurfaceContent(page.surface)

            // Open, but its row has not been read yet. Blank on the app's own background —
            // deliberately not a spinner: the read is a local index lookup, and a spinner
            // that appears for two frames is worse than nothing appearing at all.
            is ShellScreen.Page.Pending -> Box(Modifier.fillMaxSize().background(AnchorTheme.colors.bg))

            // Its OWN node's sections, not the active one's — otherwise two open folders
            // render identical content until the swipe settles.
            is ShellScreen.Page.Detail -> DetailBody(
                sections = page.sections,
                onOpen = { node -> state.eventSink(openEventFor(node)) },
            )
        }
    }
}

/** A container descends; anything else opens where it is. */
private fun openEventFor(node: WorkspaceNode): ShellScreen.Event =
    if (node.isContainer) ShellScreen.Event.Nav.DrilledInto(node) else ShellScreen.Event.Nav.RowSelected(node)

/** One artifact, rendered as whatever its kind opens as. */
@Composable
private fun ArtifactSurfaceContent(surface: ShellScreen.ArtifactSurface) {
    when (surface.kind) {
        ShellScreen.ArtifactSurface.Kind.Document ->
            DocumentReader(
                node = surface.node,
                resources = surface.resources,
                documents = surface.documents,
            )

        ShellScreen.ArtifactSurface.Kind.Voice ->
            VoiceArtifactView(node = surface.node, resources = surface.resources)

        ShellScreen.ArtifactSurface.Kind.Link -> LinkArtifactView(node = surface.node)

        // A kind the companion has no surface for yet — an app/shard today.
        ShellScreen.ArtifactSurface.Kind.Other -> UnsupportedArtifact(surface.node)
    }
}

@Composable
private fun UnsupportedArtifact(node: WorkspaceNode) {
    val c = AnchorTheme.colors
    Column(
        Modifier.fillMaxSize().padding(horizontal = 32.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(
            node.title.ifBlank { "Untitled" },
            style = MaterialTheme.typography.titleMedium,
            color = c.ink3,
        )
        Spacer(Modifier.height(6.dp))
        Text(
            "${node.artifactType ?: "This artifact"} doesn't open on the companion yet.",
            style = MetaStyle,
            color = c.ink4,
        )
    }
}

@Composable
private fun DetailHeader(state: ShellScreen.State) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    Row(
        Modifier
            .fillMaxWidth()
            .height(m.detailHeaderHeight)
            // The toggle button carries its own inset, so the gutter only opens up to the
            // full 22 when the button is absent — otherwise the title sits too far in.
            .padding(start = if (state.sidebarOpen) 22.dp else 14.dp, end = 20.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        // Only when the sidebar is hidden — otherwise the sidebar carries its own toggle.
        if (!state.sidebarOpen) {
            HeaderButton(onClick = { state.eventSink(ShellScreen.Event.Chrome.ToggleSidebar) }) {
                ColumnsIcon(c.ink3, size = 20.dp)
            }
        }

        Column(Modifier.weight(1f)) {
            Text(
                state.displayed?.title?.ifBlank { "Untitled" } ?: state.sidebarTitle,
                style = MaterialTheme.typography.headlineSmall,
                color = c.ink,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            state.displayed?.let {
                Text(
                    subtitleFor(it),
                    style = MetaStyle,
                    color = c.ink4,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
        }

        // With the copilot open the header sheds the stepper and shortens the Open pill —
        // 452px comes off the canvas, and the chrome has to give way first.
        if (state.openItems.size > 0 && !state.copilotOpen) Stepper(state)
        if (state.openItems.size > 0) OpenPill(state, compact = state.copilotOpen)
    }
}

@Composable
private fun Stepper(state: ShellScreen.State) {
    val c = AnchorTheme.colors
    Row(
        Modifier.height(44.dp).clip(AnchorShape.menuRow).background(c.field).padding(horizontal = 6.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        StepArrow(ChevronDirection.Left) { state.eventSink(ShellScreen.Event.Open.StepOpenItem(-1)) }
        Text(
            "${state.openItems.activeOrdinal} of ${state.openItems.size}",
            style = MetaStyle,
            color = c.ink3,
            modifier = Modifier.padding(horizontal = 6.dp),
        )
        StepArrow(ChevronDirection.Right) { state.eventSink(ShellScreen.Event.Open.StepOpenItem(1)) }
    }
}

@Composable
private fun StepArrow(direction: ChevronDirection, onClick: () -> Unit) {
    Box(
        Modifier.size(36.dp).clip(AnchorShape.control).clickable(onClick = onClick),
        contentAlignment = Alignment.Center,
    ) {
        ChevronIcon(AnchorTheme.colors.ink3, size = 17.dp, direction = direction)
    }
}

@Composable
private fun OpenPill(state: ShellScreen.State, compact: Boolean) {
    val c = AnchorTheme.colors
    Row(
        Modifier
            .height(44.dp)
            .clip(AnchorShape.menuRow)
            .background(if (state.switcherOpen) c.sel else Color.Transparent)
            .border(1.dp, c.line, AnchorShape.menuRow)
            .clickable { state.eventSink(ShellScreen.Event.Chrome.ToggleSwitcher) }
            .padding(horizontal = 15.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(9.dp),
    ) {
        OpenItemsIcon(c.ink2, size = 17.dp)
        if (!compact) Text("Open", style = MaterialTheme.typography.bodyLarge, color = c.ink)
        Text("${state.openItems.size}", style = MetaStyle, color = c.ink4)
    }
}

private fun subtitleFor(node: WorkspaceNode): String = buildList {
    add(
        when (node.kind) {
            NodeKind.Research -> "research folder"
            NodeKind.Folder -> "folder"
            NodeKind.Artifact -> node.artifactType ?: "artifact"
        },
    )
    node.summary?.takeIf { it.isNotBlank() }?.let { add(it) }
}.joinToString(" · ")

@Composable
private fun HeaderButton(onClick: () -> Unit, content: @Composable () -> Unit) {
    Box(
        Modifier
            .size(AnchorTheme.metrics.touchTarget)
            .clip(AnchorShape.field)
            .clickable(onClick = onClick),
        contentAlignment = Alignment.Center,
    ) { content() }
}

@Composable
private fun EmptyDetail(state: ShellScreen.State) {
    val c = AnchorTheme.colors
    Column(
        Modifier.fillMaxSize().padding(horizontal = 32.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(
            if (state.level == SidebarNav.Level.Root) "Pick a section" else "Pick something on the left",
            style = MaterialTheme.typography.titleMedium,
            color = c.ink3,
        )
        Spacer(Modifier.height(6.dp))
        Text(
            "Folders open their contents in the sidebar; what you are inside shows here.",
            style = MetaStyle,
            color = c.ink4,
        )
    }
}

/**
 * One renderer for every section's detail, driven by the section vocabulary. While the
 * sidebar is drilled into this folder the contents section is omitted — the sidebar owns
 * that list, and showing it twice is the rev-2 mistake the drill was meant to fix.
 */
@Composable
private fun DetailBody(
    sections: List<DetailSection>,
    onOpen: (WorkspaceNode) -> Unit,
) {
    val c = AnchorTheme.colors

    Column(
        Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 32.dp),
        verticalArrangement = Arrangement.spacedBy(22.dp),
    ) {
        Spacer(Modifier.height(6.dp))
        sections.forEach { section ->
            when (section) {
                is DetailSection.Stats -> StatsRow(section.cards)
                is DetailSection.Prose -> ProseCard(section.label, section.statement)
                is DetailSection.Contents -> ContentsList(section.label, section.items) { row ->
                    section.nodes.firstOrNull { it.id == row.id }?.let(onOpen)
                }
                is DetailSection.Note -> Text(section.text, style = MetaStyle, color = c.ink4)
                is DetailSection.Topics -> SectionLabel(section.label)
            }
        }
        Spacer(Modifier.height(28.dp))
    }
}

@Composable
private fun SectionLabel(label: String) {
    Text(label.uppercase(), style = SectionLabelStyle, color = AnchorTheme.colors.ink4)
}

@Composable
private fun StatsRow(cards: List<StatCard>) {
    val c = AnchorTheme.colors
    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
        cards.forEach { card ->
            Column(
                Modifier
                    .weight(1f)
                    .clip(AnchorShape.card)
                    .background(c.card)
                    .border(1.dp, c.line, AnchorShape.card)
                    .padding(horizontal = 16.dp, vertical = 14.dp),
            ) {
                Text(card.label.uppercase(), style = SectionLabelStyle, color = c.ink4)
                Spacer(Modifier.height(6.dp))
                Text(
                    card.value,
                    style = MaterialTheme.typography.titleLarge,
                    color = when (card.tone) {
                        Tone.Amber -> c.amber
                        Tone.For -> c.forColor
                        Tone.Against -> c.against
                        Tone.Ink -> c.ink
                    },
                )
            }
        }
    }
}

@Composable
private fun ProseCard(label: String, statement: String) {
    val c = AnchorTheme.colors
    Column(Modifier.fillMaxWidth()) {
        SectionLabel(label)
        Spacer(Modifier.height(8.dp))
        Box(
            Modifier
                .fillMaxWidth()
                .clip(AnchorShape.card)
                .background(c.card)
                .border(1.dp, c.line, AnchorShape.card)
                .padding(horizontal = 20.dp, vertical = 18.dp),
        ) {
            Text(statement, style = MaterialTheme.typography.bodyLarge, color = c.ink)
        }
    }
}

@Composable
private fun ContentsList(label: String, items: List<ContentRow>, onOpen: (ContentRow) -> Unit) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    Column(Modifier.fillMaxWidth()) {
        SectionLabel(label)
        Spacer(Modifier.height(8.dp))
        Column(
            Modifier.fillMaxWidth().clip(AnchorShape.card).background(c.card).border(1.dp, c.line, AnchorShape.card),
        ) {
            items.forEachIndexed { index, row ->
                if (index > 0) Box(Modifier.fillMaxWidth().height(1.dp).background(c.line2))
                Row(
                    Modifier
                        .fillMaxWidth()
                        .heightIn(min = m.drillRowMinHeight)
                        .clickable { onOpen(row) }
                        .padding(horizontal = 20.dp, vertical = 14.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(14.dp),
                ) {
                    artifactKindIcon(
                        artifactType = row.artifactType,
                        isContainer = row.kind != NodeKind.Artifact,
                        tint = if (row.kind != NodeKind.Artifact) c.amber else c.ink3,
                    )
                    Text(
                        row.title,
                        style = MaterialTheme.typography.titleMedium,
                        color = c.ink,
                        modifier = Modifier.weight(1f),
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                    row.meta?.let { Text(it, style = MetaStyle, color = c.ink4) }
                    ChevronIcon(c.ink4, size = 16.dp)
                }
            }
        }
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// Copilot pane + open-items switcher
// ─────────────────────────────────────────────────────────────────────────────

/**
 * A right overflow pane rather than a sheet (rev 2), so the workspace stays visible while
 * you ask. The turn itself is not wired yet — the desktop copilot runs through the Go API
 * and a Node sidecar — so this states that plainly instead of miming a conversation.
 */
@Composable
private fun CopilotPane(state: ShellScreen.State) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    Column(
        Modifier
            .width(m.copilotPaneWidth)
            .fillMaxHeight()
            .background(c.panel)
            .safeContent()
            .imePadding(),
    ) {
        Row(
            Modifier.fillMaxWidth().height(m.detailHeaderHeight).padding(start = 20.dp, end = 14.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            Column(Modifier.weight(1f)) {
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(9.dp)) {
                    SparkleIcon(c.amber, size = 18.dp)
                    Text("Copilot", style = MaterialTheme.typography.headlineSmall, color = c.ink)
                }
                Text(
                    "scoped to ${state.displayed?.title?.ifBlank { "Untitled" } ?: state.sidebarTitle}",
                    style = MetaStyle,
                    color = c.ink4,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
            HeaderButton(onClick = { state.eventSink(ShellScreen.Event.Chrome.CloseCopilot) }) {
                CloseIcon(c.ink3, size = 16.dp)
            }
        }

        Column(
            Modifier.weight(1f).fillMaxWidth().padding(horizontal = 20.dp),
            verticalArrangement = Arrangement.Center,
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            SparkleIcon(c.ink4, size = 28.dp)
            Spacer(Modifier.height(12.dp))
            Text("Not wired up yet", style = MaterialTheme.typography.titleMedium, color = c.ink3)
            Spacer(Modifier.height(6.dp))
            Text(
                "The pane is here; asking a question needs the copilot endpoint the desktop app uses.",
                style = MetaStyle,
                color = c.ink4,
            )
        }
    }
}

/**
 * Open items, grouped by owner — the desktop strip's "tabs of one folder stay contiguous"
 * rule surviving as headings you can actually see.
 */
@Composable
private fun SwitcherOverlay(state: ShellScreen.State) {
    val c = AnchorTheme.colors

    Box(
        Modifier
            .fillMaxSize()
            .clickable(indication = null, interactionSource = null) {
                state.eventSink(ShellScreen.Event.Chrome.ToggleSwitcher)
            },
    )

    Column(
        Modifier
            .fillMaxSize()
            .safeContent()
            .padding(20.dp),
        horizontalAlignment = Alignment.End,
    ) {
        Spacer(Modifier.height(AnchorTheme.metrics.detailHeaderHeight))
        Column(
            Modifier
                .width(424.dp)
                .heightIn(max = 660.dp)
                .clip(AnchorShape.popover)
                .background(c.card)
                .border(1.dp, c.line, AnchorShape.popover),
        ) {
            PopoverLabel("Open")
            if (state.openTabs.isEmpty()) {
                EmptyLine("nothing open yet")
                Spacer(Modifier.height(12.dp))
            } else {
                LazyColumn(
                    Modifier.padding(horizontal = 8.dp),
                    contentPadding = PaddingValues(bottom = 12.dp),
                ) {
                    state.openTabs.groupBy { it.owner }.forEach { (owner, group) ->
                        item(key = "owner:$owner") {
                            Text(
                                owner.uppercase(),
                                style = SectionLabelStyle,
                                color = c.ink4,
                                modifier = Modifier.padding(start = 10.dp, top = 10.dp, bottom = 6.dp),
                            )
                        }
                        items(group, key = { it.key }) { item ->
                            SwitcherRow(
                                title = item.title,
                                meta = listOfNotNull(item.owner, item.meta).joinToString(" · "),
                                active = item.active,
                                onOpen = { state.eventSink(ShellScreen.Event.Open.ActivateOpenItem(item.key)) },
                                onClose = { state.eventSink(ShellScreen.Event.Open.CloseOpenItem(item.key)) },
                            )
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun SwitcherRow(
    title: String,
    meta: String,
    active: Boolean,
    onOpen: () -> Unit,
    onClose: () -> Unit,
) {
    val c = AnchorTheme.colors

    Row(
        Modifier
            .fillMaxWidth()
            .heightIn(min = 50.dp)
            .clip(AnchorShape.menuRow)
            .background(if (active) c.sel else Color.Transparent)
            .clickable(onClick = onOpen)
            .padding(start = 13.dp, end = 8.dp, top = 8.dp, bottom = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Box(Modifier.size(7.dp).clip(AnchorShape.pill).background(if (active) c.amber else c.ink4))
        Column(Modifier.weight(1f)) {
            Text(
                title,
                style = MaterialTheme.typography.bodyLarge,
                color = if (active) c.ink else c.ink2,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Text(meta, style = MetaStyle, color = c.ink4, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
        Box(
            Modifier.size(34.dp).clip(AnchorShape.control).clickable(onClick = onClose),
            contentAlignment = Alignment.Center,
        ) {
            CloseIcon(c.ink4, size = 14.dp)
        }
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// Shared
// ─────────────────────────────────────────────────────────────────────────────

/** 42dp field, 13dp radius — the sidebar's search, wired to a real filter. */
@Composable
private fun SearchField(
    query: String,
    placeholder: String,
    onQueryChange: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    val c = AnchorTheme.colors

    Row(
        modifier
            .fillMaxWidth()
            .height(42.dp)
            .clip(AnchorShape.field)
            .background(c.field)
            .padding(horizontal = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(9.dp),
    ) {
        SearchIcon(c.ink4, size = 17.dp)
        Box(Modifier.weight(1f), contentAlignment = Alignment.CenterStart) {
            if (query.isEmpty()) {
                Text(
                    placeholder,
                    style = MaterialTheme.typography.bodyLarge,
                    color = c.ink4,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
            BasicTextField(
                value = query,
                onValueChange = onQueryChange,
                singleLine = true,
                textStyle = MaterialTheme.typography.bodyLarge.copy(color = c.ink),
                cursorBrush = SolidColor(c.amber),
                modifier = Modifier.fillMaxWidth(),
            )
        }
        if (query.isNotEmpty()) {
            Box(
                Modifier.size(24.dp).clip(AnchorShape.pill).clickable { onQueryChange("") },
                contentAlignment = Alignment.Center,
            ) {
                CloseIcon(c.ink3, size = 13.dp)
            }
        }
    }
}
