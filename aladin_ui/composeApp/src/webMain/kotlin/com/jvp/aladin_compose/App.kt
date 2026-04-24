package com.jvp.aladin_compose

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Bookmark
import androidx.compose.material.icons.outlined.DynamicFeed
import androidx.compose.material.icons.outlined.Hub
import androidx.compose.material.icons.outlined.Lightbulb
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.jvp.aladin_compose.ui.*
import com.jvp.aladin_compose.ui.screens.*

enum class Screen { Feed, Saved, Insights, Sources }

@Composable
fun App() {
    AladinTheme {
        var current by remember { mutableStateOf(Screen.Feed) }
        Row(modifier = Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
            Sidebar(current = current, onNavigate = { current = it })
            VerticalDivider(
                color = MaterialTheme.colorScheme.outlineVariant,
                thickness = 0.3.dp
            )
            Box(
                modifier = Modifier
                    .weight(1f)
                    .fillMaxHeight()
            ) {
                when (current) {
                    Screen.Feed -> FeedScreen()
                    Screen.Saved -> FeedScreen(savedOnly = true)
                    Screen.Insights -> InsightsScreen()
                    Screen.Sources -> SourcesScreen()
                }
            }
        }
    }
}

@Composable
private fun Sidebar(current: Screen, onNavigate: (Screen) -> Unit) {
    Column(
        modifier = Modifier
            .width(220.dp)
            .fillMaxHeight()
            .background(MaterialTheme.colorScheme.background)
            .padding(vertical = 20.dp),
        verticalArrangement = Arrangement.spacedBy(2.dp)
    ) {
        Row(
            modifier = Modifier.padding(horizontal = 20.dp).padding(bottom = 24.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(10.dp)
        ) {
            Box(
                modifier = Modifier
                    .size(28.dp)
                    .clip(RoundedCornerShape(7.dp))
                    .background(MaterialTheme.colorScheme.onSurface),
                contentAlignment = Alignment.Center
            ) {
                Text(
                    "A",
                    style = MaterialTheme.typography.labelLarge,
                    color = MaterialTheme.colorScheme.surface,
                    fontWeight = FontWeight.Bold,
                    fontSize = 13.sp
                )
            }
            Text(
                "Aladin",
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.SemiBold,
                color = MaterialTheme.colorScheme.onSurface
            )
        }

        SidebarLabel("RESEARCH")
        NavItem(Screen.Feed,     "Feed",     Icons.Outlined.DynamicFeed, current, onNavigate)
        NavItem(Screen.Saved,    "Saved",    Icons.Outlined.Bookmark,    current, onNavigate)
        NavItem(Screen.Insights, "Insights", Icons.Outlined.Lightbulb,   current, onNavigate)

        Spacer(Modifier.height(16.dp))

        SidebarLabel("SETTINGS")
        NavItem(Screen.Sources, "Sources", Icons.Outlined.Hub, current, onNavigate)
    }
}

@Composable
private fun SidebarLabel(text: String) {
    Text(
        text,
        modifier = Modifier.padding(horizontal = 20.dp, vertical = 4.dp),
        style = MaterialTheme.typography.labelSmall,
        color = AladinDark.TextMuted,
        fontWeight = FontWeight.SemiBold
    )
}

@Composable
private fun NavItem(
    screen: Screen,
    label: String,
    icon: ImageVector,
    current: Screen,
    onClick: (Screen) -> Unit
) {
    val active = screen == current
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 10.dp)
            .clip(RoundedCornerShape(8.dp))
            .clickable { onClick(screen) }
            .background(
                if (active) MaterialTheme.colorScheme.primaryContainer
                else androidx.compose.ui.graphics.Color.Transparent
            )
            .padding(horizontal = 12.dp, vertical = 9.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(10.dp)
    ) {
        Icon(
            imageVector = icon,
            contentDescription = label,
            modifier = Modifier.size(16.dp),
            tint = if (active) MaterialTheme.colorScheme.onPrimaryContainer
                   else MaterialTheme.colorScheme.onSurfaceVariant
        )
        Text(
            label,
            style = MaterialTheme.typography.bodyMedium,
            fontWeight = if (active) FontWeight.Medium else FontWeight.Normal,
            color = if (active) MaterialTheme.colorScheme.onPrimaryContainer
                    else MaterialTheme.colorScheme.onSurfaceVariant
        )
    }
}
