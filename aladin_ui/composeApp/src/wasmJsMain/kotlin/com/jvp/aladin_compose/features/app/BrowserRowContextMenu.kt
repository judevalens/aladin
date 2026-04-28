package com.jvp.aladin_compose.features.app

import androidx.compose.foundation.border
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.MoreHoriz
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.layout.boundsInWindow
import androidx.compose.ui.layout.onGloballyPositioned
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.dp
import com.jvp.aladin_compose.ui_lib.AladinColor
import com.jvp.aladin_compose.ui_lib.AladinInteractionDefaults
import com.jvp.aladin_compose.ui_lib.aladinClickable
import kotlin.math.max
import kotlin.math.min
import kotlin.math.roundToInt

internal val MenuRadius = 6.dp
internal val MenuTriggerSize = 24.dp
internal val MenuIconSize = 16.dp
internal val MenuPanelWidth = 196.dp
internal val MenuSectionVerticalPadding = 6.dp
internal val MenuRowHorizontalPadding = 10.dp
internal val MenuRowVerticalPadding = 7.dp

@Composable
fun BrowserRowContextMenu(
    menu: BrowserRowMenuModel,
    selected: Boolean,
    onActionSelected: (String) -> Unit,
    onOpenMenu: (BrowserRowMenuRequest) -> Unit,
    modifier: Modifier = Modifier,
) {
    val enabled = menu.sections.any { section -> section.actions.isNotEmpty() }
    val anchorBounds = remember(menu.rowId) { androidx.compose.runtime.mutableStateOf<Rect?>(null) }

    Box(modifier = modifier) {
        Box(
            modifier =
                Modifier.size(MenuTriggerSize)
                    .onGloballyPositioned { coordinates ->
                        anchorBounds.value = coordinates.boundsInWindow()
                    }
                    .alpha(if (enabled || selected) 1f else 0.8f)
                    .then(
                        if (enabled) {
                            Modifier.aladinClickable(
                                shape = RoundedCornerShape(MenuRadius),
                                colors =
                                    AladinInteractionDefaults.colors(
                                        hovered = AladinColor.ControlHover,
                                        pressed = AladinColor.ControlPressed,
                                    ),
                                onClick = {
                                    val anchor = anchorBounds.value ?: return@aladinClickable
                                    onOpenMenu(
                                        BrowserRowMenuRequest(
                                            menu = menu,
                                            anchorLeftPx = anchor.left,
                                            anchorRightPx = anchor.right,
                                            anchorBottomPx = anchor.bottom,
                                            onActionSelected = onActionSelected,
                                        )
                                    )
                                },
                            )
                        } else {
                            Modifier
                        }
                    ),
            contentAlignment = Alignment.Center,
        ) {
            Icon(
                imageVector = Icons.Outlined.MoreHoriz,
                contentDescription = if (enabled) "Row actions" else "No row actions",
                tint =
                    when {
                        !enabled -> AladinColor.InkDisabled
                        selected -> AladinColor.InkSecondary
                        else -> AladinColor.InkMuted
                    },
                modifier = Modifier.size(MenuIconSize),
            )
        }
    }
}

@Composable
fun BrowserRowContextMenuOverlay(
    request: BrowserRowMenuRequest,
    onDismiss: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val density = LocalDensity.current

    BoxWithConstraints(
        modifier =
            modifier.fillMaxSize().aladinClickable(
                shape = RoundedCornerShape(0.dp),
                colors =
                    AladinInteractionDefaults.colors(
                        rest = androidx.compose.ui.graphics.Color.Transparent,
                        hovered = androidx.compose.ui.graphics.Color.Transparent,
                        pressed = androidx.compose.ui.graphics.Color.Transparent,
                    ),
                onClick = onDismiss,
            )
    ) {
        val menuWidthPx = with(density) { MenuPanelWidth.roundToPx() }
        val viewportWidthPx = with(density) { maxWidth.roundToPx() }
        val preferredRightX = (request.anchorRightPx + 8f).roundToInt()
        val fallbackLeftX = (request.anchorLeftPx - menuWidthPx - 8f).roundToInt()
        val maxX = viewportWidthPx - menuWidthPx - 8
        val xPx =
            when {
                preferredRightX <= maxX -> preferredRightX
                fallbackLeftX >= 8 -> fallbackLeftX
                else -> max(8, min(preferredRightX, maxX))
            }
        val yPx = max(8, request.anchorBottomPx.roundToInt() + 4)

        Box(
            modifier = Modifier.offset { IntOffset(xPx, yPx) }
        ) {
            BrowserRowMenuPanel(
                menu = request.menu,
                onActionSelected = { actionId ->
                    onDismiss()
                    request.onActionSelected(actionId)
                },
            )
        }
    }
}

@Composable
fun BrowserRowMenuPanel(
    menu: BrowserRowMenuModel,
    onActionSelected: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier =
            modifier.width(MenuPanelWidth)
                .border(1.dp, AladinColor.Border, RoundedCornerShape(MenuRadius))
                .background(AladinColor.Panel, RoundedCornerShape(MenuRadius)),
    ) {
        menu.sections.forEachIndexed { sectionIndex, section ->
            if (sectionIndex > 0) {
                HorizontalDivider(color = AladinColor.Divider)
            }
            Column(
                modifier = Modifier.fillMaxWidth().padding(vertical = MenuSectionVerticalPadding)
            ) {
                Text(
                    text = section.title,
                    modifier = Modifier.padding(horizontal = MenuRowHorizontalPadding, vertical = 4.dp),
                    color = AladinColor.InkMuted,
                    style = MaterialTheme.typography.labelMedium,
                    fontWeight = FontWeight.Medium,
                )
                section.actions.forEach { action ->
                    BrowserRowMenuActionRow(
                        action = action,
                        onClick = {
                            onActionSelected(action.id)
                        },
                    )
                }
            }
        }
    }
}

@Composable
private fun BrowserRowMenuActionRow(
    action: BrowserRowMenuAction,
    onClick: () -> Unit,
) {
    Row(
        modifier =
            Modifier.fillMaxWidth()
                .then(
                    if (action.enabled) {
                        Modifier.aladinClickable(
                            shape = RoundedCornerShape(MenuRadius),
                            colors =
                                AladinInteractionDefaults.colors(
                                    hovered = AladinColor.ControlHover,
                                    pressed = AladinColor.ControlPressed,
                                ),
                            onClick = onClick,
                        )
                    } else {
                        Modifier
                    }
                )
                .padding(horizontal = MenuRowHorizontalPadding, vertical = MenuRowVerticalPadding),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(
            modifier =
                Modifier.size(18.dp)
                    .border(1.dp, AladinColor.Divider, RoundedCornerShape(4.dp)),
            contentAlignment = Alignment.Center,
        ) {
            Text(
                text = actionGlyph(action.label),
                color = if (action.enabled) AladinColor.CodeText else AladinColor.InkDisabled,
                style = MaterialTheme.typography.labelMedium,
            )
        }
        Text(
            text = action.label,
            color =
                when {
                    !action.enabled -> AladinColor.InkDisabled
                    action.tone == BrowserRowMenuActionTone.Destructive -> AladinColor.Ink
                    else -> AladinColor.Ink
                },
            style = MaterialTheme.typography.bodyMedium,
        )
        Spacer(modifier = Modifier.weight(1f))
    }
}

private fun actionGlyph(label: String): String {
    return label.firstOrNull()?.uppercase() ?: "+"
}
