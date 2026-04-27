package com.jvp.aladin_compose.features.app

import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.MoreHoriz
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.jvp.aladin_compose.ui_lib.AladinColor
import com.jvp.aladin_compose.ui_lib.AladinInteractionDefaults
import com.jvp.aladin_compose.ui_lib.aladinClickable

private val MenuRadius = 6.dp
private val MenuTriggerSize = 24.dp
private val MenuIconSize = 16.dp
private val MenuPanelWidth = 196.dp
private val MenuSectionVerticalPadding = 6.dp
private val MenuRowHorizontalPadding = 10.dp
private val MenuRowVerticalPadding = 7.dp

@Composable
fun BrowserRowContextMenu(
    menu: BrowserRowMenuModel,
    selected: Boolean,
    onActionSelected: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    val enabled = menu.sections.any { section -> section.actions.isNotEmpty() }
    var expanded by remember(menu.rowId) { mutableStateOf(false) }

    Box(modifier = modifier) {
        Box(
            modifier =
                Modifier.size(MenuTriggerSize)
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
                                onClick = { expanded = true },
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

        DropdownMenu(
            expanded = expanded,
            onDismissRequest = { expanded = false },
            modifier = Modifier.width(MenuPanelWidth).border(1.dp, AladinColor.Border, RoundedCornerShape(MenuRadius)),
            shape = RoundedCornerShape(MenuRadius),
            containerColor = AladinColor.Panel,
            tonalElevation = 0.dp,
            shadowElevation = 0.dp,
            scrollState = rememberScrollState(),
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
                                expanded = false
                                onActionSelected(action.id)
                            },
                        )
                    }
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
