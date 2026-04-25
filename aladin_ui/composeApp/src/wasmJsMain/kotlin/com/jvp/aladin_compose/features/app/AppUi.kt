package com.jvp.aladin_compose.features.app

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.sharp.AutoGraph
import androidx.compose.material.icons.sharp.Folder
import androidx.compose.material.icons.sharp.Home
import androidx.compose.material.icons.sharp.Hub
import androidx.compose.material.icons.sharp.Notifications
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.jvp.aladin_compose.model.*
import com.jvp.aladin_compose.ui.screens.SourcesScreen
import com.jvp.aladin_compose.ui_lib.AladinColor
import com.jvp.aladin_compose.ui_lib.AladinInteractionDefaults
import com.jvp.aladin_compose.ui_lib.aladinClickable
import com.slack.circuit.runtime.CircuitContext
import com.slack.circuit.runtime.screen.Screen
import com.slack.circuit.runtime.ui.Ui

private val DividerThickness = 0.5.dp
private val SharpRadius = 4.dp
private val ControlRadius = 6.dp

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

    Row(modifier = modifier.fillMaxSize().background(AladinColor.Canvas)) {
      AppRail(
          selected = state.destination,
          onSelect = { state.eventSink(AppEvent.NavigateDestination(it)) },
          modifier = Modifier.background(AladinColor.Canvas),
      )
      VerticalDivider(
          color = AladinColor.Divider,
          thickness = DividerThickness,
      )
      PaneTwo(state = state)
      VerticalDivider(
          color = AladinColor.Divider,
          thickness = DividerThickness,
      )
      Column(modifier = Modifier.weight(1f)) {
        TopBar(
            state = state,
            onCreateFolder = { state.eventSink(AppEvent.CreateFolder) },
            onCreateArtifact = { state.eventSink(AppEvent.CreateArtifact) },
        )
        HorizontalDivider(
            color = AladinColor.Divider,
            thickness = DividerThickness,
        )
        Row(modifier = Modifier.weight(1f)) { PaneThree(state = state) }
      }
    }
  }
}

@Composable
private fun AppRail(
    selected: NavDestination,
    onSelect: (NavDestination) -> Unit,
    modifier: Modifier = Modifier,
) {
  val items =
      listOf(
          NavDestination.Home to Icons.Sharp.Home,
          NavDestination.Folders to Icons.Sharp.Folder,
          NavDestination.Signals to Icons.Sharp.Notifications,
          NavDestination.Sources to Icons.Sharp.Hub,
          NavDestination.Graph to Icons.Sharp.AutoGraph,
      )

  Column(
      modifier = modifier.width(72.dp).fillMaxHeight().padding(vertical = 14.dp),
      horizontalAlignment = Alignment.CenterHorizontally,
      verticalArrangement = Arrangement.spacedBy(10.dp),
  ) {
    Box(
        modifier =
            Modifier.size(36.dp)
                .border(
                    1.dp,
                    AladinColor.Divider,
                    RoundedCornerShape(ControlRadius),
                )
                .background(AladinColor.Panel, RoundedCornerShape(ControlRadius)),
        contentAlignment = Alignment.Center,
    ) {
      Text("A", color = AladinColor.Ink, fontWeight = FontWeight.Bold)
    }

    Spacer(Modifier.height(8.dp))

    items.forEach { (destination, icon) ->
      val active = destination == selected

      Box(
          modifier =
              Modifier.size(42.dp).aladinClickable(
                  selected = active,
                  shape = RoundedCornerShape(SharpRadius),
              ) {
                onSelect(destination)
              },
          contentAlignment = Alignment.Center,
      ) {
        Icon(
            imageVector = icon,
            contentDescription = destination.name,
            tint =
                if (active) AladinColor.Ink else AladinColor.InkSecondary,
        )
      }
    }
  }
}

@Composable
private fun TopBar(
    state: AppState,
    onCreateFolder: () -> Unit,
    onCreateArtifact: () -> Unit,
) {
  Row(
      modifier =
          Modifier.fillMaxWidth()
              .height(60.dp)
              .background(AladinColor.Canvas)
              .padding(horizontal = 18.dp),
      verticalAlignment = Alignment.CenterVertically,
      horizontalArrangement = Arrangement.spacedBy(4.dp),
  ) {
    Spacer(Modifier.weight(1f))

    AladinToolbarField(
        modifier = Modifier.width(260.dp).aladinClickable(shape = RoundedCornerShape(ControlRadius)) {},
        text = "Search or jump...",
    )

    Spacer(Modifier.width(12.dp))

    AladinGhostAction(
        label = "+ Folder",
        onClick = onCreateFolder,
    )
    AladinGhostAction(
        label = "+ Artifact",
        onClick = onCreateArtifact,
        enabled = state.selectedItemId != null,
    )
  }
}

@Composable
private fun PaneTwo(state: AppState) {
  Box(
      modifier =
          Modifier.width(300.dp).fillMaxHeight().background(AladinColor.Canvas)
  ) {
    when (state.destination) {
      NavDestination.Home,
      NavDestination.Folders -> DocumentBrowser(state = state)

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
        val artifact = state.selectedArtifact
        if (artifact != null) {
          ArtifactWorkspaceView(artifact = artifact)
        } else if (state.selectedItem == null) {
          PlaceholderPane(
              "Select an item",
              "Choose an item from the browser to open its workspace.",
          )
        } else {
          ItemWorkspaceView(item = state.selectedItem)
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
private fun ItemWorkspaceView(item: Item) {
  Box(
      modifier = Modifier.fillMaxSize().padding(horizontal = 28.dp, vertical = 24.dp),
      contentAlignment = Alignment.TopStart,
  ) {
    Column(
        modifier = Modifier.fillMaxWidth().widthIn(max = 980.dp),
        verticalArrangement = Arrangement.spacedBy(28.dp),
    ) {
      Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Text(
            item.title,
            style = MaterialTheme.typography.headlineLarge,
            color = AladinColor.Ink,
            fontWeight = FontWeight.Bold,
        )
        Text(
            "${item.kind.name.lowercase()} · 0 relevant signals",
            style = MaterialTheme.typography.bodyMedium,
            color = AladinColor.InkSecondary,
        )
      }

      AladinPanel(modifier = Modifier.fillMaxWidth()) {
        Column(
            modifier = Modifier.padding(22.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
          Text(
              "Signals",
              style = MaterialTheme.typography.titleMedium,
              color = AladinColor.Ink,
              fontWeight = FontWeight.SemiBold,
          )
          Text(
              "Relevant signals will surface here once the first signal endpoints are wired.",
              color = AladinColor.InkSecondary,
              style = MaterialTheme.typography.bodyMedium,
          )
        }
      }
    }
  }
}

@Composable
private fun ArtifactWorkspaceView(artifact: Artifact) {
  Box(
      modifier = Modifier.fillMaxSize().padding(horizontal = 28.dp, vertical = 24.dp),
      contentAlignment = Alignment.TopStart,
  ) {
    Column(
        modifier = Modifier.fillMaxWidth().widthIn(max = 980.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
      Row(
          horizontalArrangement = Arrangement.spacedBy(10.dp),
          verticalAlignment = Alignment.CenterVertically,
      ) {
        ArtifactGlyph(artifact.kind)
        Text(
            artifact.title,
            color = AladinColor.Ink,
            style = MaterialTheme.typography.headlineLarge,
            fontWeight = FontWeight.Bold,
        )
      }
      Text(
          artifact.summary,
          color = AladinColor.InkSecondary,
          style = MaterialTheme.typography.bodyLarge,
      )
      Text(
          artifact.updatedLabel,
          color = AladinColor.InkMuted,
          style = MaterialTheme.typography.labelMedium,
      )
    }
  }
}



@Composable
private fun AladinToolbarField(
    text: String,
    modifier: Modifier = Modifier,
) {
  Box(
      modifier =
          modifier
              .border(
                  DividerThickness,
                  AladinColor.Divider,
                  RoundedCornerShape(ControlRadius),
              )
              .padding(horizontal = 14.dp, vertical = 10.dp),
      contentAlignment = Alignment.CenterStart,
  ) {
    Text(
        text,
        color = AladinColor.InkSecondary,
        style = MaterialTheme.typography.bodyMedium,
    )
  }
}

@Composable
private fun AladinGhostAction(
    label: String,
    onClick: () -> Unit,
    enabled: Boolean = true,
) {
  Box(
      modifier =
          Modifier.wrapContentWidth()
              .aladinClickable(
                  enabled = enabled,
                  shape = RoundedCornerShape(ControlRadius),
                  colors =
                      AladinInteractionDefaults.colors(
                          hovered = AladinColor.ControlHover,
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
              .border(
                  DividerThickness,
                  AladinColor.Divider,
                  RoundedCornerShape(SharpRadius),
              )
              .background(AladinColor.Panel, RoundedCornerShape(SharpRadius)),
      content = content,
  )
}

@Composable
private fun PlaceholderPane(
    title: String,
    body: String,
    background: Color = AladinColor.Canvas,
) {
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
