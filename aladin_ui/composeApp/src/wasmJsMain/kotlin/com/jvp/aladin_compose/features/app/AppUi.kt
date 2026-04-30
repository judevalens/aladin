package com.jvp.aladin_compose.features.app

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.sharp.Add
import androidx.compose.material.icons.sharp.AutoGraph
import androidx.compose.material.icons.sharp.Close
import androidx.compose.material.icons.sharp.ExpandMore
import androidx.compose.material.icons.sharp.Folder
import androidx.compose.material.icons.sharp.Home
import androidx.compose.material.icons.sharp.Hub
import androidx.compose.material.icons.sharp.Notifications
import androidx.compose.material.icons.sharp.Settings
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.ExperimentalComposeUiApi
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.graphics.BlendMode
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.boundsInWindow
import androidx.compose.ui.layout.onGloballyPositioned
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.WebElementView
import androidx.compose.ui.window.ComposeViewport
import com.jvp.aladin_compose.model.*
import com.jvp.aladin_compose.service.web.WebWidget
import com.jvp.aladin_compose.ui.screens.SourcesScreen
import com.jvp.aladin_compose.ui_lib.AladinColor
import com.jvp.aladin_compose.ui_lib.AladinInteractionDefaults
import com.jvp.aladin_compose.ui_lib.aladinClickable
import com.slack.circuit.runtime.CircuitContext
import com.slack.circuit.runtime.screen.Screen
import com.slack.circuit.runtime.ui.Ui
import kotlinx.browser.document
import kotlinx.browser.window
import org.w3c.dom.HTMLDivElement

private val DividerThickness = 0.5.dp
private val SharpRadius = 4.dp
private val ControlRadius = 6.dp
private val SidebarWidth = 232.dp
private val SidebarBrandHeight = 36.dp
private val SidebarNavItemHeight = 38.dp
private val SidebarCreateHeight = 40.dp
private val TabHeight = 42.dp
private val RailIconSize = 18.dp
private val RailMarkerHeight = 18.dp
private val RailMarkerWidth = 2.dp
private val WorkspaceMaxWidth = 980.dp
private val WorkspaceChromeMaxWidth = 1120.dp

class AppUiFactory : Ui.Factory {
    override fun create(screen: Screen, context: CircuitContext): Ui<*>? {
        return when (screen) {
            AppScreen -> AppUi()
            else -> null
        }
    }
}

private class AppUi : Ui<AppState> {
    @Composable
    override fun Content(state: AppState, modifier: Modifier) {
        var overlayMenu by remember { mutableStateOf<BrowserRowMenuRequest?>(null) }

        Box(modifier = modifier.fillMaxSize().background(AladinColor.Canvas)) {
            Row(modifier = Modifier.fillMaxSize()) {
                AppSidebar(
                    selected = state.destination,
                    onSelect = { state.eventSink(AppEvent.NavigateDestination(it)) },
                    onCreateFolder = { state.browser.eventSink(DocumentBrowserEvent.CreateFolder) },
                    onCreateArtifact = {
                        state.browser.eventSink(DocumentBrowserEvent.CreateArtifact)
                    },
                    canCreateArtifact = state.canCreateArtifact,
                    onOpenMenu = { overlayMenu = it },
                    modifier = Modifier.background(AladinColor.PanelMuted),
                )
                VerticalDivider(color = AladinColor.Divider, thickness = DividerThickness)
                PaneTwo(
                    state = state,
                    onOpenRowMenu = { overlayMenu = it },
                    onDismissRowMenu = { overlayMenu = null },
                )
                VerticalDivider(color = AladinColor.Divider, thickness = DividerThickness)
                Row(modifier = Modifier.weight(1f)) { PaneThree(state = state) }
            }

            if (overlayMenu != null) {
                AppOverlayViewport(
                    request = overlayMenu,
                    onDismiss = { overlayMenu = null },
                    modifier = Modifier.fillMaxSize(),
                )
            }
        }
    }
}

@OptIn(ExperimentalComposeUiApi::class)
@Composable
private fun AppOverlayViewport(
    request: BrowserRowMenuRequest?,
    onDismiss: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val currentRequest by rememberUpdatedState(request)
    val currentOnDismiss by rememberUpdatedState(onDismiss)

    WebElementView(
        modifier = modifier,
        factory = {
            (document.createElement("div") as HTMLDivElement).apply {
                style.background = "transparent"
                style.position = "absolute"
                style.top = "0"
                style.left = "0"
                style.right = "0"
                style.bottom = "0"

                window.requestAnimationFrame {
                    ComposeViewport(this) {
                        Box(
                            modifier =
                                Modifier.fillMaxSize().drawBehind {
                                    drawRect(color = Color.Transparent, blendMode = BlendMode.Clear)
                                }
                        ) {
                            currentRequest?.let {
                                Box(modifier = Modifier.fillMaxSize()) {
                                    BrowserRowContextMenuOverlay(
                                        request = it,
                                        onDismiss = currentOnDismiss,
                                        modifier = Modifier.fillMaxSize(),
                                    )
                                }
                            }
                        }
                    }
                }
            }
        },
    )
}

@Composable
private fun AppSidebar(
    selected: NavDestination,
    onSelect: (NavDestination) -> Unit,
    onCreateFolder: () -> Unit,
    onCreateArtifact: () -> Unit,
    canCreateArtifact: Boolean,
    onOpenMenu: (BrowserRowMenuRequest) -> Unit,
    modifier: Modifier = Modifier,
) {
    val createMenuModel = remember {
        BrowserRowMenuModel(
            rowId = "sidebar_create",
            rowKind = BrowserRowKind.Folder,
            sections =
                listOf(
                    BrowserRowMenuSection(
                        title = "Create",
                        actions =
                            listOf(
                                BrowserRowMenuAction(id = "create:folder", label = "New folder"),
                                BrowserRowMenuAction(id = "create:note", label = "New note"),
                                BrowserRowMenuAction(id = "create:link", label = "New link"),
                                BrowserRowMenuAction(
                                    id = "create:voice",
                                    label = "New voice",
                                    enabled = false,
                                ),
                                BrowserRowMenuAction(
                                    id = "create:upload",
                                    label = "New upload",
                                    enabled = false,
                                ),
                            ),
                    )
                ),
        )
    }
    val createAnchorBounds = remember { mutableStateOf<Rect?>(null) }
    val items =
        listOf(
            NavDestination.Home to Icons.Sharp.Home,
            NavDestination.Folders to Icons.Sharp.Folder,
            NavDestination.Signals to Icons.Sharp.Notifications,
            NavDestination.Sources to Icons.Sharp.Hub,
            NavDestination.Graph to Icons.Sharp.AutoGraph,
        )

    Column(
        modifier =
            modifier
                .width(SidebarWidth)
                .fillMaxHeight()
                .padding(horizontal = 14.dp, vertical = 14.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth().height(SidebarBrandHeight),
            horizontalArrangement = Arrangement.spacedBy(10.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Box(
                modifier =
                    Modifier.size(28.dp)
                        .background(AladinColor.ControlHover, RoundedCornerShape(SharpRadius))
                        .border(
                            DividerThickness,
                            AladinColor.Divider,
                            RoundedCornerShape(SharpRadius),
                        ),
                contentAlignment = Alignment.Center,
            ) {
                Text(
                    "A",
                    color = AladinColor.Ink,
                    style = MaterialTheme.typography.labelMedium,
                    fontFamily = FontFamily.Monospace,
                    fontWeight = FontWeight.Bold,
                )
            }
            Column(verticalArrangement = Arrangement.spacedBy(1.dp)) {
                Text(
                    "Aladin",
                    color = AladinColor.Ink,
                    style = MaterialTheme.typography.labelLarge,
                    fontWeight = FontWeight.SemiBold,
                )
                Text(
                    "workspace",
                    color = AladinColor.InkMuted,
                    style = MaterialTheme.typography.labelMedium,
                    fontFamily = FontFamily.Monospace,
                )
            }
        }

        AladinToolbarField(
            modifier =
                Modifier.fillMaxWidth().aladinClickable(
                    shape = RoundedCornerShape(ControlRadius)
                ) {},
            text = "Search or jump...",
        )

        SidebarCreateAction(
            label = "Create",
            modifier =
                Modifier.onGloballyPositioned { coordinates ->
                    createAnchorBounds.value = coordinates.boundsInWindow()
                },
            onClick = {
                val anchor = createAnchorBounds.value ?: return@SidebarCreateAction
                onOpenMenu(
                    BrowserRowMenuRequest(
                        menu = createMenuModel,
                        anchorLeftPx = anchor.left,
                        anchorRightPx = anchor.right,
                        anchorBottomPx = anchor.bottom,
                        placement = BrowserMenuPlacement.DropdownBelow,
                        matchAnchorWidth = true,
                        elevated = true,
                        onActionSelected = { actionId ->
                            when (actionId) {
                                "create:folder" -> onCreateFolder()
                                "create:note",
                                "create:link" -> onCreateArtifact()
                            }
                        },
                    )
                )
            },
            enabled = canCreateArtifact,
        )
        Spacer(Modifier.height(2.dp))
        HorizontalDivider(
            modifier = Modifier.fillMaxWidth().padding(horizontal = 6.dp),
            color = AladinColor.Divider,
            thickness = DividerThickness,
        )
        Spacer(Modifier.height(4.dp))
        Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
            items.forEach { (destination, icon) ->
                SidebarNavItem(
                    label = destinationLabel(destination),
                    icon = {
                        Icon(
                            imageVector = icon,
                            contentDescription = destination.name,
                            tint =
                                if (destination == selected) AladinColor.Ink
                                else AladinColor.InkMuted,
                            modifier = Modifier.size(RailIconSize),
                        )
                    },
                    active = destination == selected,
                    onClick = { onSelect(destination) },
                )
            }
        }

        Spacer(Modifier.weight(1f))

        SidebarNavItem(
            label = "Settings",
            icon = {
                Icon(
                    imageVector = Icons.Sharp.Settings,
                    contentDescription = "Settings",
                    tint = AladinColor.InkMuted,
                )
            },
            active = false,
            onClick = {},
        )
    }
}

private fun destinationLabel(destination: NavDestination): String =
    when (destination) {
        NavDestination.Home -> "Home"
        NavDestination.Folders -> "Folders"
        NavDestination.Signals -> "Signals"
        NavDestination.Sources -> "Sources"
        NavDestination.Graph -> "Graph"
    }

@Composable
private fun SidebarNavItem(
    label: String,
    icon: @Composable () -> Unit,
    active: Boolean,
    onClick: () -> Unit,
) {
    Box(
        modifier =
            Modifier.fillMaxWidth()
                .height(SidebarNavItemHeight)
                .aladinClickable(
                    selected = active,
                    shape = RoundedCornerShape(SharpRadius),
                    colors =
                        AladinInteractionDefaults.colors(
                            hovered = AladinColor.ControlHover,
                            pressed = AladinColor.ControlPressed,
                            selected = AladinColor.RowSelected,
                            selectedHovered = AladinColor.ControlPressed,
                        ),
                    onClick = onClick,
                )
    ) {
        if (active) {
            Box(
                modifier =
                    Modifier.align(Alignment.CenterStart)
                        .width(RailMarkerWidth)
                        .height(RailMarkerHeight)
                        .background(AladinColor.ActiveMarker, RoundedCornerShape(999.dp))
            )
        }
        Row(
            modifier = Modifier.fillMaxSize().padding(horizontal = 12.dp),
            horizontalArrangement = Arrangement.spacedBy(10.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            icon()
            Text(
                label,
                color = if (active) AladinColor.Ink else AladinColor.InkSecondary,
                style = MaterialTheme.typography.labelLarge,
                fontWeight = if (active) FontWeight.SemiBold else FontWeight.Medium,
            )
        }
    }
}

@Composable
private fun SidebarCreateAction(
    label: String,
    modifier: Modifier = Modifier,
    onClick: () -> Unit,
    enabled: Boolean = true,
) {
    Box(
        modifier =
            modifier
                .fillMaxWidth()
                .height(SidebarCreateHeight)
                .background(AladinColor.Ink, RoundedCornerShape(ControlRadius))
                .aladinClickable(
                    enabled = enabled,
                    shape = RoundedCornerShape(ControlRadius),
                    colors =
                        AladinInteractionDefaults.colors(
                            hovered = AladinColor.InkSurfaceHover,
                            pressed = AladinColor.ActiveMarker,
                            disabled = AladinColor.ControlHover.copy(alpha = 0.7f),
                        ),
                    onClick = onClick,
                )
                .padding(horizontal = 12.dp, vertical = 10.dp),
        contentAlignment = Alignment.CenterStart,
    ) {
        Row(
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(
                imageVector = Icons.Sharp.Add,
                contentDescription = "Create",
                tint = AladinColor.OnInkSurface,
            )
            Text(
                label,
                color = if (enabled) AladinColor.OnInkSurface else AladinColor.InkMuted,
                style = MaterialTheme.typography.labelLarge,
                fontWeight = FontWeight.Medium,
            )
            Spacer(Modifier.weight(1f))
            Icon(
                imageVector = Icons.Sharp.ExpandMore,
                contentDescription = "Create",
                tint = AladinColor.OnInkSurface,
            )
        }
    }
}

@Composable
private fun PaneTwo(
    state: AppState,
    onOpenRowMenu: (BrowserRowMenuRequest) -> Unit,
    onDismissRowMenu: () -> Unit,
) {
    Box(modifier = Modifier.width(300.dp).fillMaxHeight()) {
        when (state.destination) {
            NavDestination.Home,
            NavDestination.Folders ->
                DocumentBrowser(
                    state = state.browser,
                    onOpenRowMenu = onOpenRowMenu,
                    onDismissRowMenu = onDismissRowMenu,
                )

            NavDestination.Signals ->
                PlaceholderPane(
                    "Signals",
                    "Signal stream is coming after folder and artifact flows.",
                    AladinColor.Panel,
                )

            NavDestination.Sources ->
                PlaceholderPane(
                    "Sources",
                    "Sources stay available while the shell is being refactored.",
                    AladinColor.Panel,
                )

            NavDestination.Graph ->
                PlaceholderPane(
                    "Graph",
                    "Graph will remain a workspace-wide context view.",
                    AladinColor.Panel,
                )
        }
    }
}

@Composable
private fun RowScope.PaneThree(state: AppState) {
    Box(modifier = Modifier.weight(1f).fillMaxHeight().background(AladinColor.Canvas)) {
        when (state.destination) {
            NavDestination.Home,
            NavDestination.Folders -> {
                val artifact = state.activeArtifact
                if (artifact != null) {
                    Column(modifier = Modifier.fillMaxSize()) {
                        WorkspaceTabRail {
                            WorkspaceDocumentRail(
                                openArtifacts = state.openArtifacts,
                                activeArtifactId = artifact.id,
                                onActivateArtifact = {
                                    state.browser.eventSink(
                                        DocumentBrowserEvent.ActivateArtifactTab(it)
                                    )
                                },
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
                                        .padding(horizontal = 4.dp, vertical = 4.dp)
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

            NavDestination.Signals ->
                PlaceholderPane(
                    "Signals",
                    "Signal triage will be wired once the first folder and artifact endpoints land.",
                )

            NavDestination.Sources -> SourcesScreen()
            NavDestination.Graph ->
                PlaceholderPane("Graph", "Workspace-wide graph exploration will live here.")
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
        HorizontalDivider(color = AladinColor.Divider, thickness = DividerThickness)
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
) {
    Row(
        modifier =
            Modifier.fillMaxWidth()
                .widthIn(max = WorkspaceChromeMaxWidth)
                .padding(horizontal = 22.dp, vertical = 6.dp),
        horizontalArrangement = Arrangement.spacedBy(10.dp),
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
                val shape = RoundedCornerShape(SharpRadius)

                Row(
                    horizontalArrangement = Arrangement.spacedBy(10.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    modifier =
                        Modifier.background(if (active) AladinColor.Panel else Color.Transparent)
                            .border(
                                0.3.dp,
                                if (active) AladinColor.Border else Color.Transparent,
                                shape = shape,
                            )
                            .aladinClickable(
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
                            .padding(horizontal = 8.dp, vertical = 8.dp),
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
                                onClick = {},
                                colors =
                                    AladinInteractionDefaults.colors(
                                        hovered = AladinColor.InkDisabled
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
    /*ArtifactWebSurface(
        modifier = modifier.fillMaxSize(),
        title = artifact.title,
        kind = artifact.kind.name.lowercase(),
    )*/

    WebWidget(modifier = modifier.fillMaxSize())
}

@Composable
private fun WorkspaceSection(title: String, body: String) {
    Column(modifier = Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(12.dp)) {
        HorizontalDivider(color = AladinColor.Divider, thickness = DividerThickness)
        Column(
            modifier = Modifier.padding(top = 8.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Text(
                title.uppercase(),
                style = MaterialTheme.typography.labelMedium,
                color = AladinColor.Ink,
                fontWeight = FontWeight.Bold,
                fontFamily = FontFamily.Monospace,
                letterSpacing = 0.6.sp,
            )
            Text(
                body,
                color = AladinColor.InkSecondary,
                style = MaterialTheme.typography.bodyMedium,
            )
        }
    }
}

@Composable
private fun AladinToolbarField(text: String, modifier: Modifier = Modifier) {
    Box(
        modifier =
            modifier
                .border(DividerThickness, AladinColor.Border, RoundedCornerShape(ControlRadius))
                .background(AladinColor.CommandSurface, RoundedCornerShape(ControlRadius))
                .padding(horizontal = 14.dp, vertical = 9.dp),
        contentAlignment = Alignment.CenterStart,
    ) {
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Text(
                ">",
                color = AladinColor.CodeText,
                style = MaterialTheme.typography.labelMedium,
                fontFamily = FontFamily.Monospace,
            )
            Text(
                text,
                color = AladinColor.InkSecondary,
                style = MaterialTheme.typography.bodyMedium,
            )
        }
    }
}

@Composable
private fun AladinGhostAction(label: String, onClick: () -> Unit, enabled: Boolean = true) {
    Box(
        modifier =
            Modifier.wrapContentWidth()
                .aladinClickable(
                    enabled = enabled,
                    shape = RoundedCornerShape(ControlRadius),
                    colors =
                        AladinInteractionDefaults.colors(
                            hovered = AladinColor.ControlHover,
                            pressed = AladinColor.ControlPressed,
                            disabled = AladinColor.ControlHover.copy(alpha = 0.7f),
                        ),
                    onClick = onClick,
                )
                .padding(horizontal = 12.dp, vertical = 10.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            label,
            color = if (enabled) AladinColor.Ink else AladinColor.InkMuted,
            style = MaterialTheme.typography.labelLarge,
            fontWeight = FontWeight.Medium,
        )
    }
}

@Composable
private fun AladinPanel(
    modifier: Modifier = Modifier,
    content: @Composable ColumnScope.() -> Unit,
) {
    Column(
        modifier =
            modifier
                .border(DividerThickness, AladinColor.Divider, RoundedCornerShape(SharpRadius))
                .background(AladinColor.Panel, RoundedCornerShape(SharpRadius)),
        content = content,
    )
}

@Composable
private fun PlaceholderPane(title: String, body: String, background: Color = AladinColor.Canvas) {
    Column(
        modifier = Modifier.fillMaxSize().background(background).padding(24.dp),
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        Text(
            title,
            color = AladinColor.Ink,
            style = MaterialTheme.typography.headlineSmall,
            fontWeight = FontWeight.Bold,
        )
        Text(body, color = AladinColor.InkSecondary)
    }
}
