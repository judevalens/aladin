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
private val RailItemSize = 38.dp
private val RailIconSize = 20.dp
private val WorkspaceMaxWidth = 980.dp

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
            onCreateFolder = { state.browser.eventSink(DocumentBrowserEvent.CreateFolder) },
            onCreateArtifact = { state.browser.eventSink(DocumentBrowserEvent.CreateArtifact) },
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
            Modifier.size(34.dp)
                .border(
                    1.dp,
                    AladinColor.InkSurface,
                    RoundedCornerShape(ControlRadius),
                )
                .background(AladinColor.InkSurface, RoundedCornerShape(ControlRadius)),
        contentAlignment = Alignment.Center,
    ) {
      Text("A", color = AladinColor.OnInkSurface, fontWeight = FontWeight.Bold)
    }

    Spacer(Modifier.height(8.dp))

    items.forEach { (destination, icon) ->
      val active = destination == selected

      Box(
          modifier =
              Modifier.size(RailItemSize).aladinClickable(
                  selected = active,
                  shape = RoundedCornerShape(SharpRadius),
                  colors =
                      AladinInteractionDefaults.colors(
                          hovered = AladinColor.ControlHover,
                          pressed = AladinColor.ControlPressed,
                          selected = AladinColor.InkSurface,
                          selectedHovered = AladinColor.InkSurfaceHover,
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
                if (active) AladinColor.OnInkSurface else AladinColor.InkMuted,
            modifier = Modifier.size(RailIconSize),
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
              .height(58.dp)
              .background(AladinColor.Canvas)
              .padding(horizontal = 18.dp),
      verticalAlignment = Alignment.CenterVertically,
      horizontalArrangement = Arrangement.spacedBy(4.dp),
  ) {
    Spacer(Modifier.weight(1f))

    AladinToolbarField(
        modifier = Modifier.width(250.dp).aladinClickable(shape = RoundedCornerShape(ControlRadius)) {},
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
        enabled = state.canCreateArtifact,
    )
  }
}

@Composable
private fun PaneTwo(state: AppState) {
  Box(
      modifier =
          Modifier.width(300.dp).fillMaxHeight().background(AladinColor.PanelMuted)
  ) {
    when (state.destination) {
      NavDestination.Home,
      NavDestination.Folders -> DocumentBrowser(state = state.browser)

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
          ArtifactWorkspaceView(
              artifact = artifact,
              breadcrumbs = state.browser.breadcrumbs,
              onNavigateBreadcrumb = {
                state.browser.eventSink(DocumentBrowserEvent.NavigateBreadcrumb(it))
              },
          )
        } else if (state.selectedItem == null) {
          PlaceholderPane(
              "Select an item",
              "Choose an item from the browser to open its workspace.",
          )
        } else {
          ItemWorkspaceView(
              item = state.selectedItem,
              breadcrumbs = state.browser.breadcrumbs,
              onNavigateBreadcrumb = {
                state.browser.eventSink(DocumentBrowserEvent.NavigateBreadcrumb(it))
              },
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
private fun ItemWorkspaceView(
    item: Item,
    breadcrumbs: List<BreadcrumbItem>,
    onNavigateBreadcrumb: (String?) -> Unit,
) {
  Box(
      modifier = Modifier.fillMaxSize().padding(horizontal = 32.dp, vertical = 28.dp),
      contentAlignment = Alignment.TopStart,
  ) {
    Column(
        modifier = Modifier.fillMaxWidth().widthIn(max = WorkspaceMaxWidth),
        verticalArrangement = Arrangement.spacedBy(22.dp),
    ) {
      WorkspaceBreadcrumbRow(
          breadcrumbs = breadcrumbs,
          onNavigateBreadcrumb = onNavigateBreadcrumb,
      )
      Column(verticalArrangement = Arrangement.spacedBy(5.dp)) {
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

      WorkspaceSection(
          title = "Signals",
          body = "Relevant signals will surface here once the first signal endpoints are wired.",
      )
    }
  }
}

@Composable
private fun ArtifactWorkspaceView(
    artifact: Artifact,
    breadcrumbs: List<BreadcrumbItem>,
    onNavigateBreadcrumb: (String?) -> Unit,
) {
  Box(
      modifier = Modifier.fillMaxSize().padding(horizontal = 32.dp, vertical = 28.dp),
      contentAlignment = Alignment.TopStart,
  ) {
    Column(
        modifier = Modifier.fillMaxWidth().widthIn(max = WorkspaceMaxWidth),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
      WorkspaceBreadcrumbRow(
          breadcrumbs = breadcrumbs,
          onNavigateBreadcrumb = onNavigateBreadcrumb,
      )
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

private sealed interface WorkspaceBreadcrumbSegment {
  data class Crumb(val item: BreadcrumbItem) : WorkspaceBreadcrumbSegment

  data object Ellipsis : WorkspaceBreadcrumbSegment
}

private fun compressedWorkspaceBreadcrumbs(items: List<BreadcrumbItem>):
    List<WorkspaceBreadcrumbSegment> {
  return when {
    items.size <= 3 -> items.map { WorkspaceBreadcrumbSegment.Crumb(it) }
    else ->
        listOf(
            WorkspaceBreadcrumbSegment.Crumb(items.first()),
            WorkspaceBreadcrumbSegment.Ellipsis,
            WorkspaceBreadcrumbSegment.Crumb(items[items.lastIndex - 1]),
            WorkspaceBreadcrumbSegment.Crumb(items.last()),
        )
  }
}

@Composable
@OptIn(ExperimentalMaterial3Api::class)
private fun WorkspaceBreadcrumbRow(
    breadcrumbs: List<BreadcrumbItem>,
    onNavigateBreadcrumb: (String?) -> Unit,
) {
  val fullPath = breadcrumbs.joinToString(" / ") { it.label }
  val displaySegments = compressedWorkspaceBreadcrumbs(breadcrumbs)

  TooltipBox(
      positionProvider = TooltipDefaults.rememberTooltipPositionProvider(TooltipAnchorPosition.Above),
      tooltip = { PlainTooltip { Text(fullPath) } },
      state = rememberTooltipState(),
  ) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(5.dp),
    ) {
      displaySegments.forEachIndexed { index, segment ->
        if (index > 0) {
          Text(
              "/",
              color = AladinColor.InkMuted,
              style = MaterialTheme.typography.labelMedium,
          )
        }
        when (segment) {
          is WorkspaceBreadcrumbSegment.Crumb -> {
            val isLast = index == displaySegments.lastIndex
            Text(
                segment.item.label,
                style = MaterialTheme.typography.bodySmall,
                color = if (isLast) AladinColor.InkSecondary else AladinColor.InkMuted,
                fontWeight = if (isLast) FontWeight.SemiBold else FontWeight.Normal,
                modifier =
                    Modifier.aladinClickable(
                            enabled = !isLast,
                            shape = RoundedCornerShape(SharpRadius),
                            colors =
                                AladinInteractionDefaults.colors(
                                    hovered = AladinColor.ControlHover,
                                    pressed = AladinColor.ControlPressed,
                                ),
                            onClick = { onNavigateBreadcrumb(segment.item.id) },
                        )
                        .padding(horizontal = 2.dp),
            )
          }
          WorkspaceBreadcrumbSegment.Ellipsis ->
              Text(
                  "...",
                  color = AladinColor.InkMuted,
                  style = MaterialTheme.typography.bodySmall,
              )
        }
      }
    }
  }
}

@Composable
private fun WorkspaceSection(
    title: String,
    body: String,
) {
  Column(
      modifier = Modifier.fillMaxWidth(),
      verticalArrangement = Arrangement.spacedBy(12.dp),
  ) {
    HorizontalDivider(
        color = AladinColor.Divider,
        thickness = DividerThickness,
    )
    Column(
        modifier = Modifier.padding(top = 8.dp),
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
      Text(
          title.uppercase(),
          style = MaterialTheme.typography.labelMedium,
          color = AladinColor.Ink,
          fontWeight = FontWeight.Bold,
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
private fun AladinToolbarField(
    text: String,
    modifier: Modifier = Modifier,
) {
  Box(
      modifier =
          modifier
              .border(
                  DividerThickness,
                  AladinColor.Border,
                  RoundedCornerShape(ControlRadius),
              )
              .background(AladinColor.Panel, RoundedCornerShape(ControlRadius))
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
