package dawn.system.anchor.services.platform

import androidx.compose.foundation.layout.Box
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.Stable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier

/** The companion's v1 target is iPad; Android has no PDF surface yet. */
@Stable
actual class PdfHost internal constructor(@Suppress("unused") private val cap: Int) {
    actual fun retain(live: List<String>) = Unit
}

@Composable
actual fun rememberPdfHost(cap: Int): PdfHost = remember(cap) { PdfHost(cap) }

@Composable
actual fun PdfSurface(
    host: PdfHost,
    artifactId: String,
    filePath: String,
    page: Int,
    onPageChanged: (page: Int) -> Unit,
    modifier: Modifier,
) {
    Box(modifier) { Text("The reader is iOS-only for now") }
}
