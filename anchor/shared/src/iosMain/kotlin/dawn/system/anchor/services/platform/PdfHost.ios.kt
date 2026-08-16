package dawn.system.anchor.services.platform

import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.Stable
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.getValue
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
 * The pool, backed by an access-ordered map so "least recently mounted" is free.
 *
 * Not a `@Composable` cache: it must outlive every composition that shows a document, which
 * is the whole reason it exists.
 */
@OptIn(ExperimentalForeignApi::class)
@Stable
actual class PdfHost internal constructor(private val cap: Int) {

    private val views = mutableMapOf<String, PDFView>()

    /** Least-recently mounted first. Kotlin/Native has no access-ordered map to borrow. */
    private val recency = mutableListOf<String>()

    /**
     * The view for [artifactId], built on first ask.
     *
     * Opening the `PDFDocument` is the expensive half — the xref and the page tree — and it
     * happens exactly once per artifact for as long as the view stays pooled.
     */
    internal fun viewFor(artifactId: String, filePath: String): PDFView {
        recency.remove(artifactId)
        recency.add(artifactId)

        views[artifactId]?.let { return it }

        val view = PDFView(frame = CGRectZero.readValue()).apply {
            document = PDFDocument(NSURL.fileURLWithPath(filePath))
            autoScales = true
            displayMode = kPDFDisplaySinglePageContinuous
            displayDirection = kPDFDisplayDirectionVertical
            backgroundColor = UIColor.clearColor
        }
        views[artifactId] = view

        // Trim after inserting, and from the *front*, so the document just asked for is
        // never the one evicted.
        while (recency.size > cap) {
            views.remove(recency.removeAt(0))
        }
        return view
    }

    actual fun retain(live: List<String>) {
        val keep = live.toSet()
        recency.retainAll(keep)
        views.keys.retainAll(keep)
    }
}

@Composable
actual fun rememberPdfHost(cap: Int): PdfHost = remember(cap) { PdfHost(cap) }

@OptIn(ExperimentalForeignApi::class)
@Composable
actual fun PdfSurface(
    host: PdfHost,
    artifactId: String,
    filePath: String,
    page: Int,
    onPageChanged: (page: Int) -> Unit,
    modifier: Modifier,
) {
    val changed by rememberUpdatedState(onPageChanged)
    val view = remember(artifactId, filePath) { host.viewFor(artifactId, filePath) }

    // A finger-scroll is how you usually change page, so the outline has to hear about it.
    // The notification centre holds observers weakly, so this composition owns it — and it is
    // scoped to the *mount*, not to the view: an unmounted document is not being read.
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

    // Seek only on a freshly built view. A resident one already holds the position the user
    // left it on, and re-seeking on every mount would undo exactly what residency buys.
    DisposableEffect(view, page) {
        if (view.currentPage == null) {
            view.document?.pageAtIndex(page.toULong())?.let(view::goToPage)
        }
        onDispose { }
    }

    UIKitView(
        factory = {
            // Returning a POOLED view, not a new one. Safe because only one document is
            // mounted at a time — a UIView has one superview, and attaching in two places
            // at once is a programming error rather than a race.
            view
        },
        modifier = modifier,
    )
}
