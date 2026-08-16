package dawn.system.anchor.features.shell.ui

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.offset
import androidx.compose.runtime.Composable
import androidx.compose.runtime.key
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clipToBounds
import androidx.compose.ui.unit.IntOffset

/**
 * Every open document, composed at once, with the current one on screen and the rest parked.
 *
 * All pages are the **same size and the same place** — full viewport, at the origin — and the
 * only thing that distinguishes them is a layout offset. The current page has an offset of
 * zero; everything else is pushed [PARKING_DISTANCE_PX] away per step and is simply not
 * somewhere you can look.
 *
 * ### Why off-screen rather than hidden
 *
 * Every page stays composed, so no document is ever rebuilt. The two other ways to get that —
 * stacking them and picking with `zIndex`, or drawing them all and hiding with alpha — leave
 * every page on the *same* coordinates and then lie about it in paint, which makes correctness
 * depend on draw order or opacity reaching the interop views. Off-screen is a real position and
 * needs nothing to cooperate.
 *
 * It stays alive off-screen because Compose Multiplatform's interop layout is built for it: each
 * element is a clipping viewport wrapping an "unclipped content container"
 * (`UIKitInteropElementLayout.ios.kt:38-52`), so a native view can *"keep a stable window
 * position while the visible area is clipped by Compose."* A parked `PDFView` keeps its full
 * size and layout, and nothing hides it — `isVisible()`, the "is this child entirely clipped"
 * check, is declared and never called. PDFKit keeps its backing store.
 *
 * ### The modifier choice that carries the behaviour
 *
 * **`offset { }`, the lambda overload, and a *layout* offset.** The lambda form applies during
 * placement, so switching pages skips recomposition and re-measure and only re-places; the
 * value-taking `offset(x, y)` would recompose on every change. And it must be a layout offset,
 * never `graphicsLayer { translationX }` — the interop holder derives its rects from
 * `LayoutCoordinates`, so a native view has to be *placed* elsewhere rather than *drawn*
 * elsewhere. A draw-time translation would slide the Compose content and leave the `PDFView`
 * exactly where it was.
 *
 * ### Adding and closing without jitter
 *
 * [activeIndex] must be **derived** from [pages] by the caller, never stored. Closing a page to
 * the left shifts every later index *and* every offset, and the two cancel exactly — but only
 * if they change in the same composition:
 *
 * ```
 * [A, B, C]  active C  index 2  C parked at 0
 * close A →
 * [B, C]     active C  index 1  C parked at 0      ← C has not moved
 * ```
 *
 * A stored index is one frame of a short list against a stale offset, which the reader sees as
 * the document jumping off screen and back.
 *
 * Closing never resizes the survivors either: every page is `fillMaxSize`, so its size never
 * depended on how many siblings it had.
 */
@Composable
internal fun <T> DocumentPager(
    pages: List<T>,
    activeIndex: Int,
    key: (T) -> Any,
    modifier: Modifier = Modifier,
    page: @Composable (T) -> Unit,
) {
    Box(modifier.clipToBounds()) {
        pages.forEachIndexed { index, item ->
            // Stable keys, so removing a page makes Compose MOVE the remaining nodes rather
            // than re-map them positionally. Without this, closing one page hands its
            // neighbour's document to the wrong viewer and rebuilds both.
            key(key(item)) {
                val distance = index - activeIndex
                Box(
                    Modifier
                        .fillMaxSize()
                        .offset { IntOffset(x = distance * PARKING_DISTANCE_PX, y = 0) },
                ) { page(item) }
            }
        }
    }
}

/**
 * How far a non-current page is parked, in pixels.
 *
 * **Deliberately far larger than any viewport**, rather than exactly one page width.
 *
 * The first version placed page *i+1* at exactly the content pane's right edge and trusted
 * `clipToBounds` to hide it. That put every neighbour on the one coordinate where being off by
 * a fraction of a point is visible, and it made correctness depend on Compose's draw-time clip
 * reaching the interop views — which is a different mechanism from the `InteropWrappingView`'s
 * own `clipsToBounds`. Neighbours showed up beside the current page, cropped.
 *
 * Parking them thousands of points away removes the dependency instead of tuning it: an
 * off-screen page is off-screen whether or not the clip is exact. The page stays composed,
 * laid out and rendered — it is simply somewhere you cannot look.
 *
 * The cost is that a slide transition is no longer free, since the pages are no longer a
 * contiguous filmstrip. Document switching is a jump in this design, so nothing is lost.
 */
private const val PARKING_DISTANCE_PX = 20_000
