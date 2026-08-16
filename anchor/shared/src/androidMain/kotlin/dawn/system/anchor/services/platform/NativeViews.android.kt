package dawn.system.anchor.services.platform

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

/**
 * Android stubs. The companion's v1 target is iPad; when Android gets a reader it will
 * use PdfRenderer (or a WebView) rather than PDFKit.
 */

@Composable
actual fun NativePdfReader(
    filePath: String,
    page: Int,
    zoom: Float,
    onDocumentLoaded: (pageCount: Int) -> Unit,
    onPageChanged: (page: Int) -> Unit,
    modifier: Modifier,
) {
    Text("PDF reader is iOS-only for now", modifier)
}

actual fun documentsDirPath(): String = ""

actual fun fileExists(path: String): Boolean = java.io.File(path).exists()

actual fun fileSizeBytes(path: String): Long = java.io.File(path).takeIf { it.exists() }?.length() ?: 0L

actual fun writeBytes(path: String, bytes: ByteArray) {
    java.io.File(path).apply { parentFile?.mkdirs() }.writeBytes(bytes)
}

actual fun writeText(path: String, text: String) {
    java.io.File(path).apply { parentFile?.mkdirs() }.writeText(text)
}

actual fun openExternalUrl(url: String): Boolean = false
