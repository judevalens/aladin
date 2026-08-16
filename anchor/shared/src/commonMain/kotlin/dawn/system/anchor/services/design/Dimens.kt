package dawn.system.anchor.services.design

import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.unit.dp

/**
 * Shell metrics from the iPad handoff (PRD rev 2 — the no-rail shell). These are
 * *structural*: the handoff says to match the column structure and the touch minimums
 * exactly, while paddings and radii may be adapted. So the numbers that carry meaning live
 * here, named, and a feature never picks one.
 */
object AladinMetrics {
    /**
     * The sidebar. Collapses to zero; its content keeps this width so text does not reflow
     * mid-animation. Rev 2 has no rail — this column carries the whole navigation stack.
     */
    val sidebarWidth = 352.dp
    /** The copilot's right overflow pane. Conditional; never collapses. */
    val copilotPaneWidth = 452.dp
    /**
     * The detail header *below* the status bar. The design quotes 103 including a 31pt
     * inset; the inset is a device fact, so only the remainder is a constant here.
     */
    val detailHeaderHeight = 72.dp

    /** Accessibility floor for every control. */
    val touchTarget = 44.dp
    /** Small icon button — sidebar back, hide, hero controls. */
    val smallControl = 38.dp
    /** Root-menu section row: icon + label, one line. */
    val rootRowHeight = 50.dp
    /** Minimum height of a browser row (title + chip + meta). */
    val listRowMinHeight = 64.dp
    /** Minimum height of a drill row (kind icon + title + meta + chevron). */
    val drillRowMinHeight = 58.dp
    /** The copilot hero expanded — a squarish card at the sidebar's foot. */
    val heroFullHeight = 316.dp
    /** The copilot hero collapsed — one segmented capsule. */
    val heroMiniHeight = 54.dp
    /** The path popover ("You are in"). */
    val pathPopoverWidth = 326.dp

    /** The sidebar collapse. */
    const val COLUMN_COLLAPSE_MILLIS = 220
    /** The sidebar cross-fading between stack levels — a mark, not a narration. */
    const val SIDEBAR_LEVEL_MILLIS = 140
    /** The copilot hero folding between its card and its capsule. */
    const val HERO_COLLAPSE_MILLIS = 260
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
