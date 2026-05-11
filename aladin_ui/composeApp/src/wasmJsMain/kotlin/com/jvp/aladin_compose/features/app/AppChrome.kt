package com.jvp.aladin_compose.features.app

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.jvp.aladin_compose.model.NavDestination
import com.jvp.aladin_compose.ui_lib.AladinColor

val DividerThickness = 0.5.dp
val SharpRadius = 4.dp
val ControlRadius = 6.dp
val SidebarWidth = 232.dp
val SidebarBrandHeight = 36.dp
val SidebarNavItemHeight = 38.dp
val SidebarCreateHeight = 40.dp
val TabHeight = 42.dp
val RailIconSize = 18.dp
val RailMarkerHeight = 18.dp
val RailMarkerWidth = 2.dp
val WorkspaceMaxWidth = 980.dp
val WorkspaceChromeMaxWidth = 1120.dp

typealias AppOverlayContent = @Composable () -> Unit

@Composable
fun AladinToolbarField(text: String, modifier: Modifier = Modifier) {
    Box(
        modifier =
            modifier
                .border(DividerThickness, AladinColor.Border, RoundedCornerShape(ControlRadius))
                .background(AladinColor.CommandSurface, RoundedCornerShape(ControlRadius))
                .padding(horizontal = 14.dp, vertical = 9.dp),
        contentAlignment = Alignment.CenterStart,
    ) {
        androidx.compose.foundation.layout.Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = androidx.compose.foundation.layout.Arrangement.spacedBy(8.dp),
        ) {
            Text(
                ">",
                color = AladinColor.Ink,
                style = MaterialTheme.typography.labelMedium,
                fontFamily = FontFamily.Monospace,
                fontWeight = FontWeight.Bold,
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
fun PlaceholderPane(title: String, body: String, background: Color = AladinColor.Canvas) {
    Column(
        modifier = Modifier.fillMaxSize().background(background).padding(24.dp),
        verticalArrangement = androidx.compose.foundation.layout.Arrangement.spacedBy(8.dp),
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

@Composable
fun AladinPanel(
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

interface AppNavigationProducer {
    @Composable fun produce(): AppNavigationState
}

data class AppNavigationState(
    val destination: NavDestination,
    val eventSink: (AppNavigationEvent) -> Unit,
)

sealed interface AppNavigationEvent {
    data class Navigate(val destination: NavDestination) : AppNavigationEvent
}

class AppNavigationProducerImpl : AppNavigationProducer {
    @Composable
    override fun produce(): AppNavigationState {
        var destination by remember { mutableStateOf(NavDestination.Home) }

        return AppNavigationState(
            destination = destination,
            eventSink = { event ->
                when (event) {
                    is AppNavigationEvent.Navigate -> destination = event.destination
                }
            },
        )
    }
}

sealed interface AppShortcutEvent {
    data object DismissTopOverlay : AppShortcutEvent
}
