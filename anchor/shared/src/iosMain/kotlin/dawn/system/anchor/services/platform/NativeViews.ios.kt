package dawn.system.anchor.services.platform

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.ui.Modifier
import androidx.compose.ui.viewinterop.UIKitView
import kotlinx.cinterop.ExperimentalForeignApi
import kotlinx.cinterop.addressOf
import kotlinx.cinterop.readValue
import kotlinx.cinterop.usePinned
import platform.Foundation.NSData
import platform.Foundation.NSDocumentDirectory
import platform.Foundation.NSFileManager
import platform.Foundation.NSURL
import platform.Foundation.NSUserDomainMask
import platform.Foundation.dataWithBytes
import platform.Foundation.writeToFile
import platform.CoreGraphics.CGRectZero
import platform.PDFKit.PDFDocument
import platform.PDFKit.PDFView
import platform.PDFKit.kPDFDisplayDirectionVertical
import platform.PDFKit.kPDFDisplaySinglePageContinuous
import platform.darwin.NSObject
import platform.WebKit.WKNavigationDelegateProtocol
import platform.WebKit.WKWebView
import platform.WebKit.WKWebViewConfiguration

/**
 * PDFKit reader. `autoScales` fits the page width and PDFKit tiles/paginates a large
 * document itself — the reason the companion uses it instead of porting pdf.js.
 */
@OptIn(ExperimentalForeignApi::class)
@Composable
actual fun NativePdfReader(filePath: String, modifier: Modifier) {
    UIKitView(
        factory = {
            PDFView().apply {
                setAutoScales(true)
                setDisplayMode(kPDFDisplaySinglePageContinuous)
                setDisplayDirection(kPDFDisplayDirectionVertical)
                setDocument(PDFDocument(NSURL.fileURLWithPath(filePath)))
            }
        },
        modifier = modifier,
    )
}

/**
 * WKWebView host for shard-shaped content. `allowsInlineMediaPlayback` and friends stay
 * at defaults; what matters here is that the page is loaded from a file URL with read
 * access to its directory, so an inlined shard document behaves as it does on desktop.
 */
@OptIn(ExperimentalForeignApi::class)
@Composable
actual fun NativeWebView(
    filePath: String,
    readAccessDirPath: String,
    modifier: Modifier,
    onContentProcessTerminated: () -> Unit,
) {
    val onTerminated by rememberUpdatedState(onContentProcessTerminated)
    // The delegate is held weakly by WKWebView, so composition has to own it.
    val delegate = remember { WebContentWatchdog { onTerminated() } }

    // Keyed on the document: the factory runs once per UIKitView, so without this a
    // second call site with a different path silently keeps showing the first page —
    // two tabs claiming different documents while rendering the same one.
    key(filePath) {
        UIKitView(
            factory = {
                WKWebView(frame = CGRectZero.readValue(), configuration = WKWebViewConfiguration()).apply {
                    setOpaque(false)
                    navigationDelegate = delegate
                    loadFileURL(
                        URL = NSURL.fileURLWithPath(filePath),
                        allowingReadAccessToURL = NSURL.fileURLWithPath(readAccessDirPath, isDirectory = true),
                    )
                }
            },
            modifier = modifier,
        )
    }
}

/**
 * Reports web-content process death and brings the page back. WebKit leaves the view
 * blank otherwise, which reads as "the canvas broke" with no way to tell it apart from a
 * rendering bug.
 */
private class WebContentWatchdog(
    private val onTerminated: () -> Unit,
) : NSObject(), WKNavigationDelegateProtocol {
    override fun webViewWebContentProcessDidTerminate(webView: WKWebView) {
        onTerminated()
        webView.reload()
    }
}

actual fun documentsDirPath(): String {
    val urls = NSFileManager.defaultManager.URLsForDirectory(NSDocumentDirectory, NSUserDomainMask)
    val url = urls.firstOrNull() as? NSURL
    return url?.path.orEmpty()
}

actual fun fileExists(path: String): Boolean =
    NSFileManager.defaultManager.fileExistsAtPath(path)

@OptIn(ExperimentalForeignApi::class)
actual fun writeBytes(path: String, bytes: ByteArray) {
    ensureParentDir(path)
    // dataWithBytes copies, so the pinned buffer only has to outlive this call.
    val data = if (bytes.isEmpty()) {
        NSData()
    } else {
        bytes.usePinned { pinned ->
            NSData.dataWithBytes(pinned.addressOf(0), bytes.size.toULong())
        }
    }
    if (!data.writeToFile(path, atomically = true)) {
        error("failed to write $path")
    }
}

actual fun writeText(path: String, text: String) = writeBytes(path, text.encodeToByteArray())

@OptIn(ExperimentalForeignApi::class)
private fun ensureParentDir(path: String) {
    val parent = path.substringBeforeLast('/', "")
    if (parent.isEmpty()) return
    NSFileManager.defaultManager.createDirectoryAtPath(
        path = parent,
        withIntermediateDirectories = true,
        attributes = null,
        error = null,
    )
}
