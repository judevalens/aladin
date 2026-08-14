package dawn.system.anchor.services.design

import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.unit.dp

/**
 * Shell metrics from the iPad handoff. These are *structural* — the handoff says to match
 * the three-column structure and the touch minimums exactly, while paddings and radii may
 * be adapted. So the numbers that carry meaning live here, named.
 */
object AladinMetrics {
    /** Column 1. Fixed, never collapses. */
    val railWidth = 76.dp
    /** Column 2. Collapses to zero; its content keeps this width so text doesn't reflow. */
    val listWidth = 352.dp
    /** Column 3's header. Flush with the pane — no bar, no border. */
    val detailHeaderHeight = 72.dp

    /** Rail destination cell: icons only, so the cell itself must stay comfortably tappable. */
    val railCell = 56.dp
    /** Accessibility floor for every other control. */
    val touchTarget = 44.dp
    /** Minimum height of a list row. */
    val listRowMinHeight = 64.dp

    /** The collapse animation the handoff specifies. */
    const val COLUMN_COLLAPSE_MILLIS = 220
}

val LocalAladinMetrics = staticCompositionLocalOf { AladinMetrics }

/**
 * Spacing scale. Steps rather than ad-hoc values, so density stays consistent across
 * surfaces built at different times.
 */
object AnchorSpace {
    val xxs = 4.dp
    val xs = 8.dp
    val sm = 12.dp
    val md = 16.dp
    val lg = 20.dp
    val xl = 24.dp
    val xxl = 30.dp

    /** Gutter for panel content (list column, detail pane). */
    val gutter = 20.dp
    /** Gap between stacked rows/cards. */
    val rowGap = 11.dp
}

val LocalAnchorSpace = staticCompositionLocalOf { AnchorSpace }
