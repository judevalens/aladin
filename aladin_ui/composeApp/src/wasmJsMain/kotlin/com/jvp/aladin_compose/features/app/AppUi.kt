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
import com.jvp.aladin_compose.ui_lib.AladinLight
import com.jvp.aladin_compose.ui.screens.SourcesScreen
import com.jvp.aladin_compose.ui_lib.AladinInteractionDefaults
import com.jvp.aladin_compose.ui_lib.aladinClickable
import com.slack.circuit.runtime.CircuitContext
import com.slack.circuit.runtime.screen.Screen
import com.slack.circuit.runtime.ui.Ui

private val DividerThickness = 0.3.dp

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

    Row(modifier = modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
      AppRail(
          selected = state.destination,
          onSelect = { state.eventSink(AppEvent.NavigateDestination(it)) },
          modifier = Modifier.background(AladinLight.Background),
      )
      VerticalDivider(
          color = AladinLight.Border,
          thickness = DividerThickness,
      )
      PaneTwo(state = state)
      VerticalDivider(
          color = AladinLight.Border,
          thickness = DividerThickness,
      )
      Column(modifier = Modifier.weight(1f)) {
        TopBar(
            state = state,
            onCreateFolder = { state.eventSink(AppEvent.CreateFolder) },
            onCreateArtifact = { state.eventSink(AppEvent.CreateArtifact) },
        )
        HorizontalDivider(
            color = MaterialTheme.colorScheme.outlineVariant,
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
      modifier = modifier.width(72.dp).fillMaxHeight().padding(vertical = 16.dp),
      horizontalAlignment = Alignment.CenterHorizontally,
      verticalArrangement = Arrangement.spacedBy(10.dp),
  ) {
    Box(
        modifier =
            Modifier.size(36.dp)
                .border(
                    DividerThickness,
                    MaterialTheme.colorScheme.outlineVariant,
                    RoundedCornerShape(10.dp),
                )
                .background(MaterialTheme.colorScheme.background, RoundedCornerShape(10.dp)),
        contentAlignment = Alignment.Center,
    ) {
      Text("A", color = MaterialTheme.colorScheme.onSurface, fontWeight = FontWeight.Bold)
    }

    Spacer(Modifier.height(8.dp))

    items.forEach { (destination, icon) ->
      val active = destination == selected

      Box(
          modifier =
              Modifier.size(42.dp).aladinClickable(
                  selected = active,
                  shape = RoundedCornerShape(8.dp),
                  colors =
                      AladinInteractionDefaults.colors(
                          selected = AladinLight.Surface,
                          selectedHovered = AladinLight.SurfaceRaised,
                      ),
              ) {
                onSelect(destination)
              },
          contentAlignment = Alignment.Center,
      ) {
        Icon(
            imageVector = icon,
            contentDescription = destination.name,
            tint =
                if (active) MaterialTheme.colorScheme.onPrimaryContainer
                else MaterialTheme.colorScheme.onSurfaceVariant,
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
              .background(MaterialTheme.colorScheme.background)
              .padding(horizontal = 18.dp),
      verticalAlignment = Alignment.CenterVertically,
      horizontalArrangement = Arrangement.spacedBy(4.dp),
  ) {
    Spacer(Modifier.weight(1f))

    AladinToolbarField(
        modifier = Modifier.width(260.dp).aladinClickable(shape = RoundedCornerShape(10.dp)) {},
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
        enabled = state.selectedFolderId != null,
    )
  }
}

@Composable
private fun PaneTwo(state: AppState) {
  Box(
      modifier =
          Modifier.width(300.dp).fillMaxHeight().background(MaterialTheme.colorScheme.background)
  ) {
    when (state.destination) {
      NavDestination.Home,
      NavDestination.Folders -> DocumentBrowser(state = state)

      NavDestination.Signals ->
          PlaceholderPane(
              "Signals",
              "Signal stream is coming after folder and artifact flows.",
              MaterialTheme.colorScheme.surface,
          )

      NavDestination.Sources ->
          PlaceholderPane(
              "Sources",
              "Sources stay available while the shell is being refactored.",
              MaterialTheme.colorScheme.surface,
          )

      NavDestination.Graph ->
          PlaceholderPane(
              "Graph",
              "Graph will remain a workspace-wide context view.",
              MaterialTheme.colorScheme.surface,
          )
    }
  }
}

@Composable
private fun RowScope.PaneThree(state: AppState) {
  Box(modifier = Modifier.weight(1f).fillMaxHeight().background(AladinLight.Background)) {
    when (state.destination) {
      NavDestination.Home,
      NavDestination.Folders -> {
        val workspace = state.workspace
        val artifact = state.selectedArtifact
        if (artifact != null) {
          ArtifactWorkspaceView(artifact = artifact)
        } else if (workspace == null) {
          PlaceholderPane(
              "Select a folder",
              "Choose a folder from the middle pane to open its workspace.",
          )
        } else {
          FolderWorkspaceView(workspace = workspace)
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
private fun FolderWorkspaceView(workspace: FolderWorkspace) {
  Box(
      modifier = Modifier.fillMaxSize().padding(24.dp),
      contentAlignment = Alignment.TopCenter,
  ) {
    AladinPanel(
        modifier = Modifier.fillMaxWidth().widthIn(max = 920.dp),
    ) {
      Column(
          modifier = Modifier.padding(24.dp),
          verticalArrangement = Arrangement.spacedBy(22.dp),
      ) {
        Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
          Text(
              workspace.folder.title,
              style = MaterialTheme.typography.headlineMedium,
              color = AladinLight.TextPrimary,
              fontWeight = FontWeight.SemiBold,
          )
          Text(
              "${workspace.artifacts.size} artifacts · ${workspace.signalCount} relevant signals",
              style = MaterialTheme.typography.bodyMedium,
              color = AladinLight.TextSecondary,
          )
        }

        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
          Text(
              "Signals",
              style = MaterialTheme.typography.titleMedium,
              color = AladinLight.TextPrimary,
              fontWeight = FontWeight.SemiBold,
          )
          Text(
              "Relevant signals will surface here once the first signal endpoints are wired.",
              color = AladinLight.TextSecondary,
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
      modifier = Modifier.fillMaxSize().padding(24.dp),
      contentAlignment = Alignment.TopCenter,
  ) {
    AladinPanel(
        modifier = Modifier.fillMaxWidth().widthIn(max = 920.dp),
    ) {
      Column(
          modifier = Modifier.padding(24.dp),
          verticalArrangement = Arrangement.spacedBy(16.dp),
      ) {
        Row(
            horizontalArrangement = Arrangement.spacedBy(10.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
          ArtifactGlyph(artifact.kind)
          Text(
              artifact.title,
              color = AladinLight.TextPrimary,
              style = MaterialTheme.typography.headlineMedium,
              fontWeight = FontWeight.SemiBold,
          )
        }
        Text(
            artifact.summary,
            color = AladinLight.TextSecondary,
            style = MaterialTheme.typography.bodyLarge,
        )
        Text(
            artifact.updatedLabel,
            color = AladinLight.TextMuted,
            style = MaterialTheme.typography.labelMedium,
        )
      }
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
                  MaterialTheme.colorScheme.outlineVariant,
                  RoundedCornerShape(10.dp),
              )
              .padding(horizontal = 14.dp, vertical = 10.dp),
      contentAlignment = Alignment.CenterStart,
  ) {
    Text(
        text,
        color = MaterialTheme.colorScheme.onSurfaceVariant,
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
                  shape = RoundedCornerShape(10.dp),
                  colors =
                      AladinInteractionDefaults.colors(
                          disabled = AladinLight.SurfaceRaised.copy(alpha = 0.7f),
                      ),
                  onClick = onClick,
              )
              .padding(horizontal = 12.dp, vertical = 10.dp),
      contentAlignment = Alignment.Center,
  ) {
    Text(
        label,
        color = if (enabled) AladinLight.TextPrimary else AladinLight.TextMuted,
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
                  AladinLight.Border,
                  RoundedCornerShape(8.dp),
              )
              .background(AladinLight.Surface, RoundedCornerShape(8.dp)),
      content = content,
  )
}

@Composable
private fun PlaceholderPane(
    title: String,
    body: String,
    background: Color = MaterialTheme.colorScheme.background,
) {
  Column(
      modifier = Modifier.fillMaxSize().background(background).padding(24.dp),
      verticalArrangement = Arrangement.spacedBy(8.dp),
  ) {
    Text(title, style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.SemiBold)
    Text(body, color = MaterialTheme.colorScheme.onSurfaceVariant)
  }
}
