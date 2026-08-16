package dawn.system.anchor.services.platform

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.mutableStateOf
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
import platform.Foundation.NSFileSize
import platform.Foundation.NSNumber
import platform.Foundation.NSURL
import platform.Foundation.NSUserDomainMask
import platform.Foundation.dataWithBytes
import platform.Foundation.writeToFile
import platform.CoreGraphics.CGRectZero
import platform.PDFKit.PDFDocument
import platform.PDFKit.PDFView
import platform.PDFKit.PDFViewPageChangedNotification
import platform.Foundation.NSNotificationCenter
import platform.Foundation.NSOperationQueue
import platform.UIKit.UIColor
import platform.PDFKit.kPDFDisplayDirectionVertical
import platform.PDFKit.kPDFDisplaySinglePageContinuous
import platform.darwin.NSObject
import platform.UIKit.UIApplication

/**
 * PDFKit reader.
 *
 * PDFKit tiles and paginates a large document itself, and gives real text selection and
 * search for free — the reason the companion uses it instead of porting pdf.js.
 *
 * Two details are load-bearing:
 *
 *  - **`autoScales` and an explicit `scaleFactor` are mutually exclusive.** Leaving
 *    auto-scaling on and then setting a factor produces a page that snaps back on the next
 *    layout pass, so zoom is applied by turning it off and multiplying the *fit* factor —
 *    which is also what makes 100% mean the same thing on every device.
 *  - The page-changed notification is how a finger-scroll reaches the outline. Without it
 *    the contents rail only ever highlights pages you reached by tapping it.
 */
@OptIn(ExperimentalForeignApi::class)
@Composable
actual fun NativePdfReader(
    filePath: String,
    page: Int,
    zoom: Float,
    onDocumentLoaded: (pageCount: Int) -> Unit,
    onPageChanged: (page: Int) -> Unit,
    modifier: Modifier,
) {
    val loaded by rememberUpdatedState(onDocumentLoaded)
    val changed by rememberUpdatedState(onPageChanged)
    // The observer is held weakly by the notification centre, so composition owns it.
    val observer = remember { mutableStateOf<Any?>(null) }
    // Pinch-zoom is native to PDFView. Re-applying the chrome's zoom on every update would
    // snap a pinched page back the moment anything else changed — so it is applied only
    // when the chrome's own value actually moves.
    val appliedZoom = remember { mutableStateOf(Float.NaN) }

    DisposableEffect(Unit) {
        onDispose {
            observer.value?.let(NSNotificationCenter.defaultCenter::removeObserver)
            observer.value = null
        }
    }

    UIKitView(
        factory = {
            PDFView().apply {
                setAutoScales(true)
                setDisplayMode(kPDFDisplaySinglePageContinuous)
                setDisplayDirection(kPDFDisplayDirectionVertical)
                setBackgroundColor(UIColor.clearColor)

                // Kotlin/Native types this initializer as non-null and turns PDFKit's nil
                // — an unreadable or truncated file — into a thrown exception. Catching it
                // leaves an empty stage instead of taking the app down with the document.
                val document = runCatching {
                    PDFDocument(NSURL.fileURLWithPath(filePath))
                }.getOrNull()
                document?.let(::setDocument)
                loaded(document?.pageCount?.toInt() ?: 0)

                observer.value = NSNotificationCenter.defaultCenter.addObserverForName(
                    name = PDFViewPageChangedNotification,
                    `object` = this,
                    queue = NSOperationQueue.mainQueue,
                ) { _ ->
                    val current = currentPage ?: return@addObserverForName
                    val index = this.document?.indexForPage(current)?.toInt() ?: return@addObserverForName
                    changed(index + 1)
                }
            }
        },
        update = { view ->
            val document = view.document ?: return@UIKitView

            // Only when it actually differs: goToPage during a finger-scroll would fight
            // the gesture and jump the page back under the reader.
            val target = (page - 1).coerceIn(0, (document.pageCount.toInt() - 1).coerceAtLeast(0))
            val showing = view.currentPage?.let { document.indexForPage(it).toInt() } ?: -1
            if (target != showing) document.pageAtIndex(target.toULong())?.let(view::goToPage)

            val fit = view.scaleFactorForSizeToFit
            if (fit > 0.0 && zoom != appliedZoom.value) {
                // Explicit scaling and autoScales are mutually exclusive: leaving
                // auto-scaling on makes the factor snap back on the next layout pass.
                view.setAutoScales(false)
                view.setScaleFactor(fit * zoom)
                appliedZoom.value = zoom
            }
        },
        modifier = modifier,
    )
}

actual fun documentsDirPath(): String {
    val urls = NSFileManager.defaultManager.URLsForDirectory(NSDocumentDirectory, NSUserDomainMask)
    val url = urls.firstOrNull() as? NSURL
    return url?.path.orEmpty()
}

actual fun fileExists(path: String): Boolean =
    NSFileManager.defaultManager.fileExistsAtPath(path)

@OptIn(ExperimentalForeignApi::class)
actual fun fileSizeBytes(path: String): Long =
    (NSFileManager.defaultManager.attributesOfItemAtPath(path, null)?.get(NSFileSize) as? NSNumber)
        ?.longLongValue ?: 0L

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

actual fun openExternalUrl(url: String): Boolean {
    val target = NSURL.URLWithString(url) ?: return false
    if (!UIApplication.sharedApplication.canOpenURL(target)) return false
    UIApplication.sharedApplication.openURL(target)
    return true
}
