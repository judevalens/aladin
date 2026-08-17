package dawn.system.anchor.services.design

import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.unit.dp

/**
 * Shell metrics from the iPad navigation handoff.
 *
 * These are *structural*: the handoff fixes the column structure and the touch minimums
 * exactly, while paddings and radii may be adapted. So the numbers that carry meaning live
 * here, named, and a feature never picks one.
 *
 * The handoff expresses its sizes as a `[data-touch="ipad"]` layer overriding the desktop's
 * scale, because the React app has a desktop baseline to override. **This app has none —
 * these simply are its metrics.**
 */
object AladinMetrics {
    /** The sidebar. Collapses to zero; its content keeps this width so text does not reflow. */
    val sidebarWidth = 300.dp

    /** The copilot's right sidebar. Slides content narrower; never overlaps it. */
    val copilotPaneWidth = 368.dp

    /**
     * The content toolbar, including its 34pt safe inset. Carries back/forward, the
     * breadcrumb and the current document's meta.
     */
    val toolbarHeight = 82.dp
    val toolbarSafeInset = 34.dp

    /** The sidebar's own top inset — above the status bar rather than below it. */
    val sidebarSafeInset = 42.dp

    /** Accessibility floor. Nothing interactive is smaller, including a row's close control. */
    val touchTarget = 44.dp

    /** The sidebar's icon-row buttons: search, filter, hide. */
    val iconButton = 42.dp

    /** A destination row — the four the sidebar offers. */
    val destinationRow = 48.dp

    /** An Open-list row: status dot, title, close. */
    val openRow = 44.dp

    /** Browser columns: folders, then items. The third takes what is left. */
    val browserFolderColumn = 318.dp
    val browserItemColumn = 392.dp

    /** Browser rows, and the detail column's action buttons. */
    val browserRow = 48.dp
    val detailAction = 46.dp

    /** The floating dock shown while the sidebar is collapsed. */
    val dockBottomInset = 20.dp
    val dockCell = 48.dp

    /**
     * The browser dropdown: 1010 x 556, hung under its icon at left 14.
     *
     * [overlayTop] is where every icon-row overlay hangs — the dropdown and the filter popover
     * share it so the two read as siblings rather than as two unrelated panels.
     */
    val browserDropdownWidth = 1010.dp

    /** Rail columns: three fit the dropdown's 1010pt exactly; the tab's are wider. */
    val browserDropdownColumn = 336.dp
    val browserTabColumn = 352.dp
    val browserDropdownHeight = 556.dp
    val overlayTop = 92.dp

    /** The Open expander popup. */
    val openPopupWidth = 620.dp
    val openPopupTop = 150.dp
    val openPopupMaxHeight = 600.dp
    val openPopupRow = 56.dp

    /** The filter popover, anchored under the sidebar's icon row. */
    val filterPopoverWidth = 288.dp
    val filterRow = 40.dp
    val filterCheckbox = 19.dp

    /** The reader's measure — a column, not the full width. */
    val readerMeasure = 680.dp

    /**
     * Surface padding. Grows while the dock is showing so nothing hides beneath it.
     */
    val surfacePaddingBottom = 34.dp
    val surfacePaddingBottomWithDock = 112.dp

    /** The sidebar and copilot collapse. */
    const val COLUMN_COLLAPSE_MILLIS = 250
    /** A surface arriving: fade up, eight points. */
    const val SURFACE_IN_MILLIS = 220
    /** Overlays and the dock: pop. */
    const val POP_IN_MILLIS = 180
}

val LocalAladinMetrics = staticCompositionLocalOf { AladinMetrics }

/**
 * Spacing scale. Steps rather than ad-hoc values, so density stays consistent across
 * surfaces built at different times. The handoff's step is 4.
 */
object AnchorSpace {
    val xxs = 4.dp
    val xs = 8.dp
    val sm = 12.dp
    val md = 16.dp
    val lg = 20.dp
    val xl = 24.dp
    val xxl = 30.dp

    /** Gutter for panel content. */
    val gutter = 20.dp
    /** A surface's own horizontal padding. */
    val surfaceGutter = 34.dp
    /** Gap between stacked rows/cards. */
    val rowGap = 11.dp
}

val LocalAnchorSpace = staticCompositionLocalOf { AnchorSpace }
