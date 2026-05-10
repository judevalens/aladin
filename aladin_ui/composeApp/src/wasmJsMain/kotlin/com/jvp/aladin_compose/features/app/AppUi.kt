package com.jvp.aladin_compose.features.app

import androidx.compose.foundation.background
import androidx.compose.foundation.focusable
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.width
import androidx.compose.material3.VerticalDivider
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.ExperimentalComposeUiApi
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.FocusManager
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.BlendMode
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEvent
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.WebElementView
import androidx.compose.ui.window.ComposeViewport
import com.jvp.aladin_compose.features.app.artifactpane.ArtifactPane
import com.jvp.aladin_compose.features.app.artifactpane.ArtifactPaneState
import com.jvp.aladin_compose.features.app.browser.DocumentBrowser
import com.jvp.aladin_compose.features.app.browser.BrowserRenameDialogOverlay
import com.jvp.aladin_compose.features.app.browser.DocumentBrowserEvent
import com.jvp.aladin_compose.features.app.browser.DocumentBrowserState
import com.jvp.aladin_compose.features.app.browser.VoiceCaptureDialogOverlay
import com.jvp.aladin_compose.features.app.sidebar.AppSidebar
import com.jvp.aladin_compose.model.NavDestination
import com.jvp.aladin_compose.ui.screens.SourcesScreen
import com.jvp.aladin_compose.ui_lib.AladinColor
import com.slack.circuit.runtime.CircuitContext
import com.slack.circuit.runtime.screen.Screen
import com.slack.circuit.runtime.ui.Ui
import kotlinx.browser.document
import kotlinx.browser.window
import org.w3c.dom.HTMLDivElement

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
        var sourcesOverlay by remember { mutableStateOf<AppOverlayContent?>(null) }
        val rootFocusRequester = remember { FocusRequester() }
        val focusManager = LocalFocusManager.current

        fun dismissTopOverlay(): Boolean {
            sourcesOverlay?.let {
                sourcesOverlay = null
                return true
            }
            state.browser.activeVoiceCapture?.let {
                state.browser.eventSink(DocumentBrowserEvent.CancelVoiceCapture)
                return true
            }
            state.browser.activeRename?.let { rename ->
                state.browser.eventSink(DocumentBrowserEvent.CancelRename(rename.rowId))
                return true
            }
            overlayMenu?.let {
                overlayMenu = null
                return true
            }
            return false
        }

        fun handleShortcut(event: AppShortcutEvent): Boolean =
            when (event) {
                AppShortcutEvent.DismissTopOverlay -> dismissTopOverlay()
            }

        LaunchedEffect(Unit) { rootFocusRequester.requestFocus() }

        LaunchedEffect(state.navigation.destination) {
            if (state.navigation.destination != NavDestination.Sources) {
                sourcesOverlay = null
            }
        }

        Box(
            modifier =
                modifier.fillMaxSize()
                    .clearFocusOnAppTap(focusManager)
                    .focusRequester(rootFocusRequester)
                    .focusable()
                    .onPreviewKeyEvent { event -> handleAppKeyEvent(event, ::handleShortcut) }
                    .background(AladinColor.Canvas),
        ) {
            Row(modifier = Modifier.fillMaxSize()) {
                AppSidebar(
                    state = state.sidebar,
                    onOpenMenu = { overlayMenu = it },
                    modifier = Modifier.background(AladinColor.PanelMuted),
                )
                VerticalDivider(color = AladinColor.Divider, thickness = DividerThickness)
                PaneTwo(
                    destination = state.navigation.destination,
                    browser = state.browser,
                    onOpenRowMenu = { overlayMenu = it },
                    onDismissRowMenu = { overlayMenu = null },
                )
                VerticalDivider(color = AladinColor.Divider, thickness = DividerThickness)
                Row(modifier = Modifier.weight(1f)) {
                    PaneThree(
                        destination = state.navigation.destination,
                        state = state.artifactPane,
                        setSourcesOverlay = { sourcesOverlay = it },
                    )
                }
            }

            if (overlayMenu != null ||
                state.browser.activeRename != null ||
                state.browser.activeVoiceCapture != null ||
                sourcesOverlay != null
            ) {
                AppOverlayViewport(
                    request = overlayMenu,
                    onDismiss = { overlayMenu = null },
                    browser = state.browser,
                    sourcesOverlay = sourcesOverlay,
                    onShortcut = ::handleShortcut,
                    modifier = Modifier.fillMaxSize(),
                )
            }
        }
    }
}

private fun Modifier.clearFocusOnAppTap(focusManager: FocusManager): Modifier =
    pointerInput(focusManager) {
        detectTapGestures {
            focusManager.clearFocus()
        }
    }

private fun handleAppKeyEvent(
    event: KeyEvent,
    onShortcut: (AppShortcutEvent) -> Boolean,
): Boolean {
    if (event.type != KeyEventType.KeyDown) return false
    return when (event.key) {
        Key.Escape -> onShortcut(AppShortcutEvent.DismissTopOverlay)
        else -> false
    }
}

@OptIn(ExperimentalComposeUiApi::class)
@Composable
private fun AppOverlayViewport(
    request: BrowserRowMenuRequest?,
    onDismiss: () -> Unit,
    browser: DocumentBrowserState,
    sourcesOverlay: AppOverlayContent?,
    onShortcut: (AppShortcutEvent) -> Boolean,
    modifier: Modifier = Modifier,
) {
    val currentRequest by rememberUpdatedState(request)
    val currentOnDismiss by rememberUpdatedState(onDismiss)
    val currentBrowser by rememberUpdatedState(browser)
    val currentSourcesOverlay by rememberUpdatedState(sourcesOverlay)
    val currentOnShortcut by rememberUpdatedState(onShortcut)

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
                        val overlayFocusRequester = remember { FocusRequester() }
                        LaunchedEffect(Unit) { overlayFocusRequester.requestFocus() }
                        Box(
                            modifier =
                                Modifier.fillMaxSize()
                                    .focusRequester(overlayFocusRequester)
                                    .focusable()
                                    .onPreviewKeyEvent { event ->
                                        handleAppKeyEvent(event, currentOnShortcut)
                                    }
                                    .drawBehind {
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
                            currentBrowser.activeRename?.let { rename ->
                                BrowserRenameDialogOverlay(
                                    rename = rename,
                                    errorMessage = currentBrowser.errorMessage,
                                    onDraftChanged = { title ->
                                        currentBrowser.eventSink(
                                            DocumentBrowserEvent.RenameDraftChanged(
                                                rename.rowId,
                                                title,
                                            )
                                        )
                                    },
                                    onCommit = {
                                        currentBrowser.eventSink(
                                            DocumentBrowserEvent.CommitRename(rename.rowId)
                                        )
                                    },
                                    onCancel = {
                                        currentBrowser.eventSink(
                                            DocumentBrowserEvent.CancelRename(rename.rowId)
                                        )
                                    },
                                )
                            }
                            currentBrowser.activeVoiceCapture?.let { capture ->
                                VoiceCaptureDialogOverlay(
                                    state = capture,
                                    onBeginRecording = {
                                        currentBrowser.eventSink(
                                            DocumentBrowserEvent.BeginVoiceRecording
                                        )
                                    },
                                    onStopRecording = {
                                        currentBrowser.eventSink(
                                            DocumentBrowserEvent.StopVoiceRecording
                                        )
                                    },
                                    onTogglePlayback = {
                                        currentBrowser.eventSink(
                                            DocumentBrowserEvent.ToggleVoicePlayback
                                        )
                                    },
                                    onSeekPlayback = { positionMs ->
                                        currentBrowser.eventSink(
                                            DocumentBrowserEvent.SeekVoicePlayback(positionMs)
                                        )
                                    },
                                    onTitleChanged = { title ->
                                        currentBrowser.eventSink(
                                            DocumentBrowserEvent.VoiceCaptureTitleChanged(title)
                                        )
                                    },
                                    onDescriptionChanged = { description ->
                                        currentBrowser.eventSink(
                                            DocumentBrowserEvent.VoiceCaptureDescriptionChanged(
                                                description,
                                            )
                                        )
                                    },
                                    onSave = {
                                        currentBrowser.eventSink(
                                            DocumentBrowserEvent.SaveVoiceCapture
                                        )
                                    },
                                    onCancel = {
                                        currentBrowser.eventSink(
                                            DocumentBrowserEvent.CancelVoiceCapture
                                        )
                                    },
                                )
                            }
                            currentSourcesOverlay?.invoke()
                        }
                    }
                }
            }
        },
    )
}

@Composable
private fun PaneTwo(
    destination: NavDestination,
    browser: DocumentBrowserState,
    onOpenRowMenu: (BrowserRowMenuRequest) -> Unit,
    onDismissRowMenu: () -> Unit,
) {
    Box(modifier = Modifier.width(300.dp).fillMaxHeight()) {
        when (destination) {
            NavDestination.Home,
            NavDestination.Folders ->
                DocumentBrowser(
                    state = browser,
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
private fun androidx.compose.foundation.layout.RowScope.PaneThree(
    destination: NavDestination,
    state: ArtifactPaneState,
    setSourcesOverlay: (AppOverlayContent?) -> Unit,
) {
    when (destination) {
        NavDestination.Home,
        NavDestination.Folders -> ArtifactPane(state = state, modifier = Modifier)

        NavDestination.Signals ->
            Box(modifier = Modifier.weight(1f).fillMaxSize()) {
                PlaceholderPane(
                    "Signals",
                    "Signal triage will be wired once the first folder and artifact endpoints land.",
                )
            }

        NavDestination.Sources ->
            Box(modifier = Modifier.weight(1f).fillMaxWidth()) {
                SourcesScreen(setAppOverlay = setSourcesOverlay)
            }

        NavDestination.Graph ->
            Box(modifier = Modifier.weight(1f).fillMaxSize()) {
                PlaceholderPane("Graph", "Workspace-wide graph exploration will live here.")
            }
    }
}
