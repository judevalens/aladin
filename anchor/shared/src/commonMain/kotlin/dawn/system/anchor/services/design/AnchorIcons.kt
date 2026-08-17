package dawn.system.anchor.services.design

import androidx.compose.foundation.layout.size
import androidx.compose.material3.Icon
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import compose.icons.FeatherIcons
import compose.icons.feathericons.Bell
import compose.icons.feathericons.ChevronsLeft
import compose.icons.feathericons.ChevronsRight
import compose.icons.feathericons.FilePlus
import compose.icons.feathericons.FolderPlus
import compose.icons.feathericons.Minus
import compose.icons.feathericons.Plus
import compose.icons.feathericons.Sidebar
import compose.icons.feathericons.Trash2
import compose.icons.feathericons.ChevronDown
import compose.icons.feathericons.ChevronLeft
import compose.icons.feathericons.ChevronRight
import compose.icons.feathericons.ChevronUp
import compose.icons.feathericons.Columns
import compose.icons.feathericons.Edit3
import compose.icons.feathericons.ExternalLink
import compose.icons.feathericons.FileText
import compose.icons.feathericons.Folder
import compose.icons.feathericons.Globe
import compose.icons.feathericons.Grid
import compose.icons.feathericons.Home
import compose.icons.feathericons.Layout
import compose.icons.feathericons.Link
import compose.icons.feathericons.Mic
import compose.icons.feathericons.Search
import compose.icons.feathericons.Share2
import compose.icons.feathericons.Star
import compose.icons.feathericons.Maximize2
import compose.icons.feathericons.MoreHorizontal
import compose.icons.feathericons.Paperclip
import compose.icons.feathericons.Sliders
import compose.icons.feathericons.TrendingUp
import compose.icons.feathericons.X
import compose.icons.feathericons.Zap
import dawn.system.anchor.domain.Destination

/**
 * Icons, by **role**.
 *
 * The art comes from **Feather** (`compose-icons`) — the handoff asks for the codebase's
 * Lucide set, and Lucide is a fork of Feather, so the two share their geometry and their
 * 24×24 / round-cap drawing conventions. This file is the only place that knows which glyph
 * stands for which role: a feature asks for "the icon for this artifact kind", never for a
 * specific glyph, so re-pointing a role at a different icon is a one-line change here.
 *
 * Nothing is hand-drawn and nothing is vendored from another package's build output.
 */
private val DEFAULT_SIZE = 20.dp

/** The larger glyph a file card leads with. */
val FileGlyphSize = 26.dp

@Composable
private fun Glyph(icon: ImageVector, tint: Color, size: Dp, modifier: Modifier = Modifier) {
    Icon(imageVector = icon, contentDescription = null, tint = tint, modifier = modifier.size(size))
}

// ── Sections (the sidebar's root menu) ───────────────────────────────────────

/** The four iPad destinations. Each surface owns its own shape; only Browser is a tree. */
@Composable
fun destinationIcon(destination: Destination, tint: Color, size: Dp = 22.dp) {
    Glyph(
        icon = when (destination) {
            Destination.Home -> FeatherIcons.Home
            Destination.Markets -> FeatherIcons.TrendingUp
            Destination.Graph -> FeatherIcons.Share2
        },
        tint = tint,
        size = size,
    )
}

// ── Artifact kinds (browser rows) ────────────────────────────────────────────

/**
 * Icon per artifact kind (PRD rev 2 §Artifact icons) — what replaced the abstract dot.
 * Tint is the caller's: a folder is amber because of what it *is*, and that role belongs to
 * the surface drawing the row, not to the glyph.
 */
@Composable
fun artifactKindIcon(artifactType: String?, isContainer: Boolean, tint: Color, size: Dp = 18.dp) {
    Glyph(
        icon = when {
            isContainer -> FeatherIcons.Folder
            artifactType == "app" -> FeatherIcons.Layout
            artifactType == "page" || artifactType == "note" -> FeatherIcons.Edit3
            artifactType == "voice" -> FeatherIcons.Mic
            artifactType == "link" -> FeatherIcons.Link
            else -> FeatherIcons.FileText
        },
        tint = tint,
        size = size,
    )
}

// ── Chrome ───────────────────────────────────────────────────────────────────

enum class ChevronDirection { Left, Right, Up, Down }

/**
 * Chevron, pointing where the tap goes — Back, row disclosure, the sidebar title's
 * path-popover caret and the hero's expand/collapse are one family, so they read as the
 * same affordance rather than four different controls.
 */
@Composable
fun ChevronIcon(
    tint: Color,
    modifier: Modifier = Modifier,
    size: Dp = DEFAULT_SIZE,
    direction: ChevronDirection = ChevronDirection.Right,
) {
    Glyph(
        icon = when (direction) {
            ChevronDirection.Left -> FeatherIcons.ChevronLeft
            ChevronDirection.Right -> FeatherIcons.ChevronRight
            ChevronDirection.Up -> FeatherIcons.ChevronUp
            ChevronDirection.Down -> FeatherIcons.ChevronDown
        },
        tint = tint,
        size = size,
        modifier = modifier,
    )
}

/** The sidebar's search field. */
@Composable
fun SearchIcon(tint: Color, modifier: Modifier = Modifier, size: Dp = 17.dp) =
    Glyph(FeatherIcons.Search, tint, size, modifier)

/** Dismiss — an open-item row, the copilot pane, a cleared search. */
@Composable
fun CloseIcon(tint: Color, modifier: Modifier = Modifier, size: Dp = 14.dp) =
    Glyph(FeatherIcons.X, tint, size, modifier)

/** The sidebar collapse toggle. */
@Composable
fun ColumnsIcon(tint: Color, modifier: Modifier = Modifier, size: Dp = DEFAULT_SIZE) =
    Glyph(FeatherIcons.Columns, tint, size, modifier)

/** Open items — what replaced the desktop tab strip. */
@Composable
fun OpenItemsIcon(tint: Color, modifier: Modifier = Modifier, size: Dp = 18.dp) =
    Glyph(FeatherIcons.Grid, tint, size, modifier)

/** The copilot. Feather has no sparkle; the star carries the same "assistant" reading. */
@Composable
fun SparkleIcon(tint: Color, modifier: Modifier = Modifier, size: Dp = 19.dp) =
    Glyph(FeatherIcons.Star, tint, size, modifier)

/** Notifications, in the copilot hero's header row. */
/** The "make something new" affordance — the sidebar's + and its menu. */
@Composable
fun PlusIcon(tint: Color, modifier: Modifier = Modifier, size: Dp = DEFAULT_SIZE) =
    Glyph(FeatherIcons.Plus, tint, size, modifier)

@Composable
fun MinusIcon(tint: Color, modifier: Modifier = Modifier, size: Dp = DEFAULT_SIZE) =
    Glyph(FeatherIcons.Minus, tint, size, modifier)

@Composable
fun NewFolderIcon(tint: Color, modifier: Modifier = Modifier, size: Dp = DEFAULT_SIZE) =
    Glyph(FeatherIcons.FolderPlus, tint, size, modifier)

@Composable
fun NewPageIcon(tint: Color, modifier: Modifier = Modifier, size: Dp = DEFAULT_SIZE) =
    Glyph(FeatherIcons.FilePlus, tint, size, modifier)

/** Rename. Feather's pencil, the same glyph desktop uses for an editable title. */
@Composable
fun RenameIcon(tint: Color, modifier: Modifier = Modifier, size: Dp = DEFAULT_SIZE) =
    Glyph(FeatherIcons.Edit3, tint, size, modifier)

@Composable
fun DeleteIcon(tint: Color, modifier: Modifier = Modifier, size: Dp = DEFAULT_SIZE) =
    Glyph(FeatherIcons.Trash2, tint, size, modifier)

/** Leaving the app: the system browser, a share sheet. */
@Composable
fun ExternalLinkIcon(tint: Color, modifier: Modifier = Modifier, size: Dp = DEFAULT_SIZE) =
    Glyph(FeatherIcons.ExternalLink, tint, size, modifier)

/** The reader's contents-rail toggle. */
@Composable
fun SidebarIcon(tint: Color, modifier: Modifier = Modifier, size: Dp = DEFAULT_SIZE) =
    Glyph(FeatherIcons.Sidebar, tint, size, modifier)

@Composable
fun BellIcon(tint: Color, modifier: Modifier = Modifier, size: Dp = 18.dp) =
    Glyph(FeatherIcons.Bell, tint, size, modifier)

/**
 * The filter facets. Feather's `Sliders` — lucide's `sliders` in the handoff, same glyph.
 */
@Composable
fun FilterIcon(tint: Color, modifier: Modifier = Modifier, size: Dp = DEFAULT_SIZE) =
    Glyph(FeatherIcons.Sliders, tint, size, modifier)

/** Everything not in the breadcrumb lives behind this. */
@Composable
fun MoreIcon(tint: Color, modifier: Modifier = Modifier, size: Dp = DEFAULT_SIZE) =
    Glyph(FeatherIcons.MoreHorizontal, tint, size, modifier)

/**
 * Pin — "stays open while you work".
 *
 * **A substitution, and a visible one.** The design asks for a thumbtack; Feather has no
 * thumbtack, so this is `Paperclip` — the nearest glyph meaning "kept attached". Same call as
 * `sparkles` → `Star`: take a real glyph from the icon set rather than draw one, and say so.
 * Worth raising with the designer, since the pin is one of only three buttons in the dropdown's
 * header.
 */
@Composable
fun PinIcon(tint: Color, modifier: Modifier = Modifier, size: Dp = DEFAULT_SIZE) =
    Glyph(FeatherIcons.Paperclip, tint, size, modifier)

/** Promote the dropdown to a full-width tab. */
@Composable
fun MaximizeIcon(tint: Color, modifier: Modifier = Modifier, size: Dp = DEFAULT_SIZE) =
    Glyph(FeatherIcons.Maximize2, tint, size, modifier)
