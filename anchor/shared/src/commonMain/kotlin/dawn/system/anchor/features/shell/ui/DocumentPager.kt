package dawn.system.anchor.features.shell.ui

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.requiredWidth
import androidx.compose.runtime.Composable
import androidx.compose.runtime.key
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clipToBounds
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.unit.IntOffset

/**
 * The open documents, side by side, with the current one in the window.
 *
 * A **window and a filmstrip**. The box never moves: it measures how wide a column should be
 * and clips everything outside itself. The row is `pages.size` columns wide and is the only
 * thing that slides, which is why the offset belongs to it — put the offset on the box and you
 * move the viewport itself, clip region and all, rather than the content behind it.
 *
 * ### Why off-screen rather than hidden
 *
 * Every page stays composed, so no document is ever rebuilt. The two other ways to achieve that
 * — stacking them and picking with `zIndex`, or drawing them all and hiding with alpha — put
 * every page at the *same* coordinates and then lie about it in paint. Off-screen is a real
 * position, so nothing depends on interop draw-order behaving.
 *
 * It works because Compose Multiplatform's interop layout is built for exactly this. Each
 * element is a clipping viewport wrapping an "unclipped content container"
 * (`UIKitInteropElementLayout.ios.kt:38-52`), specifically so a native view can *"keep a stable
 * window position while the visible area is clipped by Compose, without requiring frequent Auto
 * Layout constraint updates for scrolling/positioning."* An off-screen page's `PDFView` keeps
 * its full size and layout and is merely clipped — and nothing hides it, because
 * `isVisible()`, the "is this child entirely clipped" check, is declared and never called.
 * PDFKit therefore keeps its backing store.
 *
 * ### Two modifier choices that carry the whole behaviour
 *
 *  - **`offset { }`, the lambda overload.** It applies during *placement*, so switching pages
 *    skips recomposition and re-measure and only re-places. The value-taking `offset(x, y)`
 *    recomposes the row on every change, which is the cost this exists to avoid.
 *  - **A layout offset, never `graphicsLayer { translationX }`.** The interop holder derives
 *    its clipped and unclipped rects from `LayoutCoordinates`, so a native view has to be
 *    *placed* elsewhere, not merely *drawn* elsewhere. A draw-time translation would slide the
 *    Compose content and leave the `PDFView` sitting where it was.
 *
 * ### Adding and closing without jitter
 *
 * [activeIndex] must be **derived** from [pages] by the caller, never stored. Closing a page to
 * the left shifts every later index *and* the offset, and the two cancel exactly — but only if
 * they change in the same composition:
 *
 * ```
 * [A, B, C]  active C  index 2  offset -2w
 * close A →
 * [B, C]     active C  index 1  offset -1w      ← C has not moved
 * ```
 *
 * A stored index is one frame of a short list against a stale offset, which the reader sees as
 * a jump of a full column.
 *
 * Closing never resizes the survivors either: [requiredWidth] means a column's size never
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
    BoxWithConstraints(modifier.clipToBounds()) {
        val pageWidth = maxWidth
        val step = with(LocalDensity.current) { pageWidth.roundToPx() }

        Row(Modifier.fillMaxSize().offset { IntOffset(x = -activeIndex * step, y = 0) }) {
            pages.forEach { item ->
                // Stable keys, so removing a column makes Compose MOVE the remaining nodes
                // rather than re-map them positionally. Without this, closing one page hands
                // its neighbour's document to the wrong viewer and rebuilds both.
                key(key(item)) {
                    // requiredWidth, not width: each column is a viewport wide even though the
                    // row it sits in is only one viewport wide. The columns overflow, and the
                    // box above clips them.
                    Box(Modifier.requiredWidth(pageWidth).fillMaxHeight()) { page(item) }
                }
            }
        }
    }
}
