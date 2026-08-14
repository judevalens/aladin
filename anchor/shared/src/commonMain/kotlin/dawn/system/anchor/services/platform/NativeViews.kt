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

/** Renders the PDF at [filePath] (an absolute on-device path) in a scrolling reader. */
@Composable
expect fun NativePdfReader(filePath: String, modifier: Modifier = Modifier)

/**
 * Loads [filePath] (an absolute on-device path to an .html file) in a web view.
 * [readAccessDirPath] is the directory the web view may read siblings from.
 *
 * [onContentProcessTerminated] fires when the web content process dies — the page's own
 * process, not the app's. Without this the only symptom is a silently blank pane, which
 * is exactly how the simulator's JIT crashes hid. The view reloads itself afterwards.
 */
@Composable
expect fun NativeWebView(
    filePath: String,
    readAccessDirPath: String,
    modifier: Modifier = Modifier,
    onContentProcessTerminated: () -> Unit = {},
)

/** Absolute path of the app's Documents directory — where spike fixtures are staged. */
expect fun documentsDirPath(): String

/** True when [path] exists on disk. */
expect fun fileExists(path: String): Boolean

/** Writes [bytes] to [path], creating parent directories. Throws on failure. */
expect fun writeBytes(path: String, bytes: ByteArray)

/** Writes [text] as UTF-8 to [path], creating parent directories. Throws on failure. */
expect fun writeText(path: String, text: String)
