package dawn.system.anchor.services.platform

import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

/**
 * Platform-native panes embedded inside Compose. Both are iOS-first: the reader is
 * PDFKit (native text selection, search, tiled rendering) and the shard host is a
 * WKWebView, which is what a published Aladin shard needs — an opaque-origin frame
 * running inlined JS/CSS.
 *
 * Android actuals are stubs for now; the companion's v1 target is iPad.
 */

/**
 * Renders the PDF at [filePath] (an absolute on-device path) in a scrolling reader.
 *
 * Page and zoom are *inputs*, not internal state, so the reader's own chrome — the outline
 * rail, the page stepper, the zoom pill — drives one source of truth rather than trying to
 * stay in step with a view that is also steering itself. [onPageChanged] closes the loop
 * for the other direction: scrolling with a finger is how you usually change page, and the
 * outline has to follow that too.
 *
 * [zoom] is relative to fit-width, so 1.0 means "the page fills the pane" on any device.
 *
 */
@Composable
expect fun NativePdfReader(
    filePath: String,
    page: Int,
    zoom: Float,
    onDocumentLoaded: (pageCount: Int) -> Unit,
    onPageChanged: (page: Int) -> Unit,
    modifier: Modifier = Modifier,
)

/** Absolute path of the app's Documents directory — where spike fixtures are staged. */
expect fun documentsDirPath(): String

/** True when [path] exists on disk. */
expect fun fileExists(path: String): Boolean

/** Size of the file at [path] in bytes, or 0 when there is nothing there to measure. */
expect fun fileSizeBytes(path: String): Long

/** Writes [bytes] to [path], creating parent directories. Throws on failure. */
expect fun writeBytes(path: String, bytes: ByteArray)

/** Writes [text] as UTF-8 to [path], creating parent directories. Throws on failure. */
expect fun writeText(path: String, text: String)

/**
 * Hands [url] to the system browser. Returns false when the platform refused it — a
 * malformed or unsupported scheme — so the caller can say so instead of appearing to do
 * nothing.
 */
expect fun openExternalUrl(url: String): Boolean
