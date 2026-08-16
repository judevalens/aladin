package dawn.system.anchor.services.platform

import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.ui.Modifier
import androidx.compose.ui.viewinterop.UIKitView
import kotlinx.cinterop.ExperimentalForeignApi
import kotlinx.cinterop.readValue
import platform.CoreGraphics.CGRectZero
import platform.Foundation.NSNotificationCenter
import platform.Foundation.NSOperationQueue
import platform.Foundation.NSURL
import platform.PDFKit.PDFDocument
import platform.PDFKit.PDFView
import platform.PDFKit.PDFViewPageChangedNotification
import platform.PDFKit.kPDFDisplayDirectionVertical
import platform.PDFKit.kPDFDisplaySinglePageContinuous
import platform.UIKit.UIColor

/**
 * PDFKit, one view per document, built once.
 *
 * `remember(filePath)` is the whole lifetime rule: the view is created on first composition
 * and kept for as long as this stays in the tree. The `UIKitView` factory hands it straight
 * back, so Compose never rebuilds the interop node either.
 *
 * Note what is deliberately absent: no `update` lambda, no frame assignment, no
 * `bringSubviewToFront`. Every one of those asks UIKit to do work on a switch, and the point
 * of this shape is that a switch asks for nothing at all.
 */
@OptIn(ExperimentalForeignApi::class)
@Composable
actual fun PdfViewer(
    filePath: String,
    page: Int,
    onPageChanged: (page: Int) -> Unit,
    modifier: Modifier,
) {
    val changed by rememberUpdatedState(onPageChanged)

    val view = remember(filePath) {
        PDFView(frame = CGRectZero.readValue()).apply {
            document = PDFDocument(NSURL.fileURLWithPath(filePath))
            autoScales = true
            displayMode = kPDFDisplaySinglePageContinuous
            displayDirection = kPDFDisplayDirectionVertical
            // Opaque: viewers are stacked, and a clear background would let the one behind
            // show through. Black rather than a theme token — this layer knows nothing about
            // the theme, and the app ships dark.
            backgroundColor = UIColor.blackColor
        }
    }

    // A finger-scroll is how you usually change page, so the outline has to hear about it.
    // The notification centre holds observers weakly, so the composition owns this one.
    DisposableEffect(view) {
        val observer = NSNotificationCenter.defaultCenter.addObserverForName(
            name = PDFViewPageChangedNotification,
            `object` = view,
            queue = NSOperationQueue.mainQueue,
            usingBlock = {
                val current = view.currentPage ?: return@addObserverForName
                changed(view.document?.indexForPage(current)?.toInt() ?: 0)
            },
        )
        onDispose { NSNotificationCenter.defaultCenter.removeObserver(observer) }
    }

    // Seek once, on a freshly built view. Re-seeking would undo the position the user left it
    // on, which is the thing staying composed is meant to preserve.
    DisposableEffect(view) {
        view.document?.pageAtIndex(page.toULong())?.let(view::goToPage)
        onDispose { }
    }

    UIKitView(factory = { view }, modifier = modifier)
}
