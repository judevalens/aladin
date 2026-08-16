package dawn.system.anchor.features.shell.ui

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.offset
import androidx.compose.runtime.Composable
import androidx.compose.runtime.key
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clipToBounds
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.unit.IntOffset

/**
 * Shows one page of many, and **never rebuilds the ones it is not showing**.
 *
 * A general primitive: it knows nothing about what a page contains. Give it a list, say which
 * entry is current, say how to identify one, and say how to draw one.
 *
 * ```
 * ResidentPager(pages, activeIndex, key = { it.id }) { page -> … }
 * ```
 *
 * ### What it is for
 *
 * Native views — a `PDFView`, a `WKWebView` — are expensive to build and hold state the
 * composition cannot see: a parsed document, a scroll position, a live socket, unsaved edits.
 * Swapping the current one out of the composition destroys all of it, and coming back is a
 * reload. This keeps every page composed so nothing is ever rebuilt, and moves the ones you
 * are not looking at somewhere you cannot see them.
 *
 * ### Why off-screen rather than hidden
 *
 * The alternatives leave every page on the *same* coordinates and then lie about it in paint:
 * stack them and pick with `zIndex`, or draw them all and hide with alpha. Both make
 * correctness depend on draw order or opacity reaching the interop views, which is a different
 * mechanism from Compose's own. Off-screen is a real position and needs nothing to cooperate.
 *
 * A parked page stays alive because Compose Multiplatform's interop layout is built for it:
 * each element is a clipping viewport wrapping an "unclipped content container"
 * (`UIKitInteropElementLayout.ios.kt:38-52`), so a native view can *"keep a stable window
 * position while the visible area is clipped by Compose."* Nothing hides it either —
 * `isVisible()`, the "is this child entirely clipped" check, is declared and never called.
 *
 * ### The contract, and it is one line
 *
 * **[activeIndex] must be derived from [pages], never stored.** Position is
 * `index - activeIndex`, so the current page sits at zero *by definition* — it is pinned to
 * the origin, not moved there. Everything else is invisible, so nothing else's position can
 * matter. The single way a reader ever sees movement is [activeIndex] disagreeing with which
 * page is actually current, which a stored index allows for a frame and a derived one cannot.
 *
 * That is also why adding and closing are free: closing a page shifts both `index` and
 * `activeIndex`, and the current page was never going anywhere regardless.
 *
 * ### Two things that will break it
 *
 *  - **Wrapping a page in a condition.** `if (active) …` reads like an optimisation and is a
 *    teardown; it is the exact thing this exists to prevent.
 *  - **Using a positional key.** [key] must identify the *page*, not its slot, or removing one
 *    hands its neighbour's content to the wrong node and rebuilds both.
 *
 * ### Not a scroller
 *
 * There is no gesture and no animation: parked pages are two windows apart, so a slide would
 * cross a window of empty space. Switching is a jump. A slide would mean interpolating toward
 * contiguous spacing for the duration of the transition and parking again at rest — worth
 * doing only if a design asks for it.
 */
@Composable
internal fun <T> ResidentPager(
    pages: List<T>,
    activeIndex: Int,
    key: (T) -> Any,
    modifier: Modifier = Modifier,
    page: @Composable (T) -> Unit,
) {
    BoxWithConstraints(modifier.clipToBounds()) {
        // Derived from the window rather than guessed. Pages are window-sized, so one window
        // of travel already clears it and two leaves a full window of gap — enough that no
        // rounding or clipping imprecision can put a neighbour back on screen.
        //
        // The fallback matters: a zero or not-yet-measured width would park every page at
        // zero, which is every page stacked and visible — the exact failure being avoided.
        // An unbounded constraint arrives as a huge number and would overflow on multiply.
        val windowPx = with(LocalDensity.current) { maxWidth.roundToPx() }
        val park = if (windowPx in 1..MAX_SANE_WINDOW_PX) windowPx * 2 else FALLBACK_PARK_PX

        pages.forEachIndexed { index, item ->
            key(key(item)) {
                val distance = index - activeIndex
                Box(
                    Modifier
                        .fillMaxSize()
                        // The LAMBDA overload, and a LAYOUT offset. The lambda applies at
                        // placement, so a switch skips recomposition and re-measure entirely.
                        // And it must be layout, never graphicsLayer { translationX } — the
                        // interop holder derives its rects from LayoutCoordinates, so a native
                        // view has to be *placed* elsewhere, not merely *drawn* elsewhere.
                        .offset { IntOffset(x = distance * park, y = 0) },
                ) { page(item) }
            }
        }
    }
}

/**
 * Above this, the measurement is not a window — it is an unbounded constraint reported as a
 * number. Multiplying it would overflow rather than park anything.
 */
private const val MAX_SANE_WINDOW_PX = 100_000

/** Used only when the window has not been measured yet, or reports nonsense. */
private const val FALLBACK_PARK_PX = 20_000
