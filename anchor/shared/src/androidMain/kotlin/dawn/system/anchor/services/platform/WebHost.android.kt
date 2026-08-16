package dawn.system.anchor.services.platform

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.Stable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier

/** Android has no web surface yet; the companion's v1 target is iPad. */
@Stable
actual class WebHostHandle internal constructor()

@Composable
actual fun rememberWebHost(
    filePath: String,
    readAccessDirPath: String,
    bootstrapJs: String,
    onMessage: (String) -> Unit,
    onContentProcessTerminated: () -> Unit,
): WebHostHandle = remember { WebHostHandle() }

@Composable
actual fun WebHostSurface(handle: WebHostHandle, command: String, modifier: Modifier) {
    Text("Web view is iOS-only for now", modifier)
}
