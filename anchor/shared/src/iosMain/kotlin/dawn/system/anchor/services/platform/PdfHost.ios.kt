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
import platform.UIKit.UIView
import platform.UIKit.UIViewAutoresizingFlexibleHeight
import platform.UIKit.UIViewAutoresizingFlexibleWidth

/**
 * The pool, backed by an access-ordered map so "least recently mounted" is free.
 *
 * Not a `@Composable` cache: it must outlive every composition that shows a document, which
 * is the whole reason it exists.
 */
@OptIn(ExperimentalForeignApi::class)
@Stable
actual class PdfHost internal constructor(private val cap: Int) {

    /**
     * The one view Compose ever hosts.
     *
     * Every resident document is a subview of this, and switching **brings one to front** —
     * nothing is added, removed or re-laid-out. That is what removes the flash: the earlier
     * shape handed `UIKitView` a different `PDFView` per document, so Compose tore the interop
     * node down and built a new one, and for a frame or two nothing was painted at all. The
     * pooled views survived that; the host did not.
     *
     * It also means a backgrounded document never leaves the view hierarchy, so WebKit-style
     * backing-store eviction does not apply to it either.
     */
    internal val container = UIView(frame = CGRectZero.readValue()).apply {
        clipsToBounds = true
    }

    private val views = mutableMapOf<String, PDFView>()

    /** Least-recently shown first. Kotlin/Native has no access-ordered map to borrow. */
    private val recency = mutableListOf<String>()

    /**
     * Brings [artifactId] to the front, building its view on first ask.
     *
     * Opening the `PDFDocument` is the expensive half — the xref and the page tree — and it
     * happens exactly once per artifact for as long as the view stays pooled.
     */
    internal fun show(artifactId: String, filePath: String): PDFView {
        recency.remove(artifactId)
        recency.add(artifactId)

        val view = views.getOrPut(artifactId) {
            PDFView(frame = container.bounds).apply {
                document = PDFDocument(NSURL.fileURLWithPath(filePath))
                autoScales = true
                displayMode = kPDFDisplaySinglePageContinuous
                displayDirection = kPDFDisplayDirectionVertical
                // OPAQUE, deliberately. The documents behind this one are still in the
                // hierarchy, so a clear background would let them show through. Black rather
                // than a theme token because this layer knows nothing about the theme — and
                // the app ships dark, so it reads as the page ground either way.
                backgroundColor = UIColor.blackColor
                autoresizingMask = UIViewAutoresizingFlexibleWidth or UIViewAutoresizingFlexibleHeight
                container.addSubview(this)
            }
        }
        view.setFrame(container.bounds)
        container.bringSubviewToFront(view)

        // Trim after inserting, and from the front, so the document just asked for is never
        // the one evicted.
        while (recency.size > cap) {
            views.remove(recency.removeAt(0))?.removeFromSuperview()
        }
        return view
    }

    actual fun retain(live: List<String>) {
        val keep = live.toSet()
        recency.retainAll(keep)
        views.keys.filterNot(keep::contains).forEach { id ->
            views.remove(id)?.removeFromSuperview()
        }
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
    val view = remember(artifactId, filePath) { host.show(artifactId, filePath) }

    // A finger-scroll is how you usually change page, so the outline has to hear about it.
    // The notification centre holds observers weakly, so this composition owns it. Scoped to
    // the document being *shown*: a backgrounded one is still alive but is not being read.
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
    // left it on, and re-seeking on every switch would undo exactly what residency buys.
    DisposableEffect(view, page) {
        if (view.currentPage == null) {
            view.document?.pageAtIndex(page.toULong())?.let(view::goToPage)
        }
        onDispose { }
    }

    UIKitView(
        // The CONTAINER, which is the same object for the life of the pool — so this interop
        // node is never torn down and rebuilt, and there is no frame with nothing in it.
        factory = { host.container },
        // Switching documents is a z-order change inside a hierarchy that never moves.
        update = { host.show(artifactId, filePath) },
        modifier = modifier,
    )
}
