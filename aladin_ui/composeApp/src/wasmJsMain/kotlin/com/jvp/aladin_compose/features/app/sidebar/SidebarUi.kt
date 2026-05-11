package com.jvp.aladin_compose.features.app.sidebar

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.Logout
import androidx.compose.material.icons.outlined.AutoGraph
import androidx.compose.material.icons.outlined.Folder
import androidx.compose.material.icons.outlined.Home
import androidx.compose.material.icons.outlined.Hub
import androidx.compose.material.icons.outlined.Notifications
import androidx.compose.material.icons.sharp.Add
import androidx.compose.material.icons.sharp.ExpandMore
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.layout.boundsInWindow
import androidx.compose.ui.layout.onGloballyPositioned
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.jvp.aladin_compose.features.app.AladinToolbarField
import com.jvp.aladin_compose.features.app.BrowserMenuPlacement
import com.jvp.aladin_compose.features.app.BrowserRowKind
import com.jvp.aladin_compose.features.app.BrowserRowMenuAction
import com.jvp.aladin_compose.features.app.BrowserRowMenuModel
import com.jvp.aladin_compose.features.app.BrowserRowMenuRequest
import com.jvp.aladin_compose.features.app.BrowserRowMenuSection
import com.jvp.aladin_compose.features.app.ControlRadius
import com.jvp.aladin_compose.features.app.DividerThickness
import com.jvp.aladin_compose.features.app.RailIconSize
import com.jvp.aladin_compose.features.app.RailMarkerHeight
import com.jvp.aladin_compose.features.app.RailMarkerWidth
import com.jvp.aladin_compose.features.app.SharpRadius
import com.jvp.aladin_compose.features.app.SidebarBrandHeight
import com.jvp.aladin_compose.features.app.SidebarCreateHeight
import com.jvp.aladin_compose.features.app.SidebarNavItemHeight
import com.jvp.aladin_compose.features.app.SidebarWidth
import com.jvp.aladin_compose.model.NavDestination
import com.jvp.aladin_compose.ui_lib.AladinColor
import com.jvp.aladin_compose.ui_lib.AladinInteractionDefaults
import com.jvp.aladin_compose.ui_lib.aladinClickable

@Composable
fun AppSidebar(
    state: SidebarState,
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
            NavDestination.Home to Icons.Outlined.Home,
            NavDestination.Folders to Icons.Outlined.Folder,
            NavDestination.Signals to Icons.Outlined.Notifications,
            NavDestination.Sources to Icons.Outlined.Hub,
            NavDestination.Graph to Icons.Outlined.AutoGraph,
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
                    style = MaterialTheme.typography.labelLarge,
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
                                "create:folder" -> state.eventSink(SidebarEvent.CreateFolder)
                                "create:note",
                                "create:link" -> state.eventSink(SidebarEvent.CreateArtifact)
                                "create:voice" -> state.eventSink(SidebarEvent.CreateVoice)
                            }
                        },
                    )
                )
            },
            enabled = state.canCreateArtifact,
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
                                if (destination == state.selectedDestination) AladinColor.OnInkSurface
                                else AladinColor.InkMuted,
                            modifier = Modifier.size(RailIconSize),
                        )
                    },
                    active = destination == state.selectedDestination,
                    onClick = { state.eventSink(SidebarEvent.Navigate(destination)) },
                )
            }
        }

        Spacer(Modifier.weight(1f))

        Column(
            modifier = Modifier.fillMaxWidth().padding(horizontal = 10.dp, vertical = 2.dp),
            verticalArrangement = Arrangement.spacedBy(2.dp),
        ) {
            Text(
                "signed in",
                color = AladinColor.InkMuted,
                style = MaterialTheme.typography.labelSmall,
                fontFamily = FontFamily.Monospace,
            )
            Text(
                state.userEmail,
                color = AladinColor.InkSecondary,
                style = MaterialTheme.typography.labelMedium,
                maxLines = 1,
            )
        }

        SidebarNavItem(
            label = "Sign out",
            icon = {
                Icon(
                    imageVector = Icons.AutoMirrored.Outlined.Logout,
                    contentDescription = "Sign out",
                    tint = AladinColor.InkMuted,
                )
            },
            active = false,
            onClick = { state.eventSink(SidebarEvent.Logout) },
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
                            selected = AladinColor.InkSurface,
                            selectedHovered = AladinColor.InkSurfaceHover,
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
                        .background(AladinColor.OnInkSurface, RoundedCornerShape(999.dp))
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
                color = if (active) AladinColor.OnInkSurface else AladinColor.InkSecondary,
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
