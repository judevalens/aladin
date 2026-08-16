package dawn.system.anchor.services.platform

import androidx.compose.runtime.Composable
import androidx.compose.runtime.Stable
import androidx.compose.ui.Modifier

/**
 * The pool of PDF views.
 *
 * **Created above the surface swap, never inside a surface.** That placement is the entire
 * point. A `PDFView` built inside the surface that shows it is destroyed the moment you
 * switch away, and coming back re-opens and re-parses the document, re-seeks and re-renders —
 * which is exactly the reload this exists to avoid. The same rule the web host already
 * follows: *own the resource above the recycler.*
 *
 * ### Residency and retention are different things
 *
 * Retaining a *value* — page 94, zoom 1.4 — makes a document come back to the right **place**.
 * It does not stop it going away, because the expensive part is the document parse and the
 * tile render, not the number. Only keeping the view alive does that.
 *
 * So: up to [cap] documents stay **resident** and switching between them is free. Past the
 * cap the least-recently-used view is dropped, and that document falls back to retention —
 * it rebuilds on return, at the right page. A visible rebuild, but only for the cold ones.
 *
 * ### Only one is mounted
 *
 * Views are held here but attached one at a time, because *a `UIView` has one superview* and
 * attaching in two places is a programming error. Detaching does not destroy: the pool holds
 * the strong reference, so the document stays parsed and the view keeps its own page, zoom and
 * scroll offset natively — there is no ViewState to save and restore.
 */
@Stable
expect class PdfHost {
    /**
     * Drops every view whose artifact is not in [live], and trims what remains to the cap,
     * least-recently-mounted first.
     */
    fun retain(live: List<String>)
}

/** Creates the pool. Call from the shell, above the content slot. */
@Composable
expect fun rememberPdfHost(cap: Int = 3): PdfHost

/**
 * Mounts [artifactId]'s pooled view, creating it on first use.
 *
 * [page] and [zoom] are the chrome's inputs for a *freshly built* view. A view that is already
 * resident keeps whatever the user left it on, because the view is the source of truth for its
 * own scroll position — feeding it back a stale page on every re-mount would undo the very
 * thing residency buys.
 */
@Composable
expect fun PdfSurface(
    host: PdfHost,
    artifactId: String,
    filePath: String,
    page: Int,
    onPageChanged: (page: Int) -> Unit,
    modifier: Modifier = Modifier,
)
