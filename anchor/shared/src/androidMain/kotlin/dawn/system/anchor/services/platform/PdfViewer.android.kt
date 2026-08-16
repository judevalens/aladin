package dawn.system.anchor.services.platform

import androidx.compose.foundation.layout.Box
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

/** The companion's v1 target is iPad; Android has no PDF surface yet. */
@Composable
actual fun PdfViewer(
    filePath: String,
    page: Int,
    onPageChanged: (page: Int) -> Unit,
    modifier: Modifier,
) {
    Box(modifier) { Text("The reader is iOS-only for now") }
}
