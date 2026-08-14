package dawn.system.anchor.services.design

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import dawn.system.anchor.domain.Destination

/**
 * Rail and chrome icons, drawn rather than bundled.
 *
 * The handoff specifies 22px stroked glyphs at 1.75 weight and says to replace its inline
 * SVGs with the codebase's icon set. Until an icon dependency is chosen, these are drawn
 * to that weight — one place to swap, and no 2 MB icon font for eleven glyphs.
 */
private const val STROKE_RATIO = 0.075f

@Composable
fun destinationIcon(destination: Destination, tint: Color, size: Dp = 22.dp) {
    when (destination) {
        Destination.Home -> HomeIcon(tint, size = size)
        Destination.Markets -> MarketsIcon(tint, size = size)
        Destination.Folders -> FolderIcon(tint, size = size)
        Destination.Sources -> SourceIcon(tint, size = size)
        Destination.Insights -> InsightIcon(tint, size = size)
        Destination.Entities -> EntityIcon(tint, size = size)
        Destination.Graph -> GraphIcon(tint, size = size)
    }
}

private fun DrawScope.strokeWidth(): Float = this.size.minDimension * STROKE_RATIO

@Composable
fun HomeIcon(tint: Color, modifier: Modifier = Modifier, size: Dp = 22.dp) {
    Canvas(modifier.size(size)) {
        val s = this.size.minDimension
        val w = strokeWidth()
        val roofY = s * 0.46f
        drawLine(tint, Offset(s * 0.16f, roofY), Offset(s * 0.5f, s * 0.17f), w, StrokeCap.Round)
        drawLine(tint, Offset(s * 0.5f, s * 0.17f), Offset(s * 0.84f, roofY), w, StrokeCap.Round)
        drawRoundRect(
            tint,
            topLeft = Offset(s * 0.24f, roofY),
            size = Size(s * 0.52f, s * 0.37f),
            cornerRadius = CornerRadius(s * 0.07f),
            style = Stroke(w),
        )
    }
}

@Composable
fun MarketsIcon(tint: Color, modifier: Modifier = Modifier, size: Dp = 22.dp) {
    Canvas(modifier.size(size)) {
        val s = this.size.minDimension
        val w = strokeWidth()
        // Two candles — the market glyph in the design's rail.
        drawLine(tint, Offset(s * 0.34f, s * 0.18f), Offset(s * 0.34f, s * 0.82f), w, StrokeCap.Round)
        drawRoundRect(
            tint,
            topLeft = Offset(s * 0.24f, s * 0.32f),
            size = Size(s * 0.2f, s * 0.3f),
            cornerRadius = CornerRadius(s * 0.05f),
            style = Stroke(w),
        )
        drawLine(tint, Offset(s * 0.66f, s * 0.18f), Offset(s * 0.66f, s * 0.82f), w, StrokeCap.Round)
        drawRoundRect(
            tint,
            topLeft = Offset(s * 0.56f, s * 0.46f),
            size = Size(s * 0.2f, s * 0.26f),
            cornerRadius = CornerRadius(s * 0.05f),
            style = Stroke(w),
        )
    }
}

@Composable
fun FolderIcon(tint: Color, modifier: Modifier = Modifier, size: Dp = 22.dp) {
    Canvas(modifier.size(size)) {
        val s = this.size.minDimension
        val w = strokeWidth()
        drawLine(tint, Offset(s * 0.16f, s * 0.32f), Offset(s * 0.44f, s * 0.32f), w, StrokeCap.Round)
        drawLine(tint, Offset(s * 0.44f, s * 0.32f), Offset(s * 0.52f, s * 0.42f), w, StrokeCap.Round)
        drawRoundRect(
            tint,
            topLeft = Offset(s * 0.16f, s * 0.32f),
            size = Size(s * 0.68f, s * 0.42f),
            cornerRadius = CornerRadius(s * 0.08f),
            style = Stroke(w),
        )
    }
}

@Composable
fun SourceIcon(tint: Color, modifier: Modifier = Modifier, size: Dp = 22.dp) {
    Canvas(modifier.size(size)) {
        val s = this.size.minDimension
        val w = strokeWidth()
        drawRoundRect(
            tint,
            topLeft = Offset(s * 0.24f, s * 0.16f),
            size = Size(s * 0.52f, s * 0.68f),
            cornerRadius = CornerRadius(s * 0.08f),
            style = Stroke(w),
        )
        listOf(0.36f, 0.5f, 0.64f).forEach { f ->
            drawLine(
                tint,
                Offset(s * 0.36f, s * f),
                Offset(s * 0.64f, s * f),
                w * 0.85f,
                StrokeCap.Round,
            )
        }
    }
}

@Composable
fun InsightIcon(tint: Color, modifier: Modifier = Modifier, size: Dp = 22.dp) {
    Canvas(modifier.size(size)) {
        val s = this.size.minDimension
        val w = strokeWidth()
        drawCircle(tint, radius = s * 0.24f, center = Offset(s * 0.5f, s * 0.42f), style = Stroke(w))
        drawLine(tint, Offset(s * 0.42f, s * 0.72f), Offset(s * 0.58f, s * 0.72f), w, StrokeCap.Round)
        drawLine(tint, Offset(s * 0.44f, s * 0.83f), Offset(s * 0.56f, s * 0.83f), w, StrokeCap.Round)
    }
}

@Composable
fun EntityIcon(tint: Color, modifier: Modifier = Modifier, size: Dp = 22.dp) {
    Canvas(modifier.size(size)) {
        val s = this.size.minDimension
        val w = strokeWidth()
        drawCircle(tint, radius = s * 0.15f, center = Offset(s * 0.5f, s * 0.34f), style = Stroke(w))
        drawRoundRect(
            tint,
            topLeft = Offset(s * 0.26f, s * 0.56f),
            size = Size(s * 0.48f, s * 0.26f),
            cornerRadius = CornerRadius(s * 0.13f),
            style = Stroke(w),
        )
    }
}

@Composable
fun GraphIcon(tint: Color, modifier: Modifier = Modifier, size: Dp = 22.dp) {
    Canvas(modifier.size(size)) {
        val s = this.size.minDimension
        val w = strokeWidth()
        val a = Offset(s * 0.26f, s * 0.7f)
        val b = Offset(s * 0.52f, s * 0.28f)
        val cpt = Offset(s * 0.76f, s * 0.62f)
        drawLine(tint, a, b, w * 0.8f, StrokeCap.Round)
        drawLine(tint, b, cpt, w * 0.8f, StrokeCap.Round)
        listOf(a, b, cpt).forEach { drawCircle(tint, radius = s * 0.09f, center = it, style = Stroke(w)) }
    }
}

/** Open items — the grid that replaces the desktop tab strip. */
@Composable
fun OpenItemsIcon(tint: Color, modifier: Modifier = Modifier, size: Dp = 22.dp) {
    Canvas(modifier.size(size)) {
        val s = this.size.minDimension
        val w = strokeWidth()
        val cell = s * 0.3f
        val gap = s * 0.1f
        val start = (s - (cell * 2 + gap)) / 2f
        listOf(0 to 0, 1 to 0, 0 to 1, 1 to 1).forEach { (col, row) ->
            drawRoundRect(
                tint,
                topLeft = Offset(start + col * (cell + gap), start + row * (cell + gap)),
                size = Size(cell, cell),
                cornerRadius = CornerRadius(s * 0.06f),
                style = Stroke(w * 0.9f),
            )
        }
    }
}

/** The list-column collapse toggle. */
@Composable
fun ColumnsIcon(tint: Color, modifier: Modifier = Modifier, size: Dp = 20.dp) {
    Canvas(modifier.size(size)) {
        val s = this.size.minDimension
        val w = strokeWidth()
        drawRoundRect(
            tint,
            topLeft = Offset(s * 0.16f, s * 0.22f),
            size = Size(s * 0.68f, s * 0.56f),
            cornerRadius = CornerRadius(s * 0.08f),
            style = Stroke(w),
        )
        drawLine(tint, Offset(s * 0.42f, s * 0.22f), Offset(s * 0.42f, s * 0.78f), w * 0.9f)
    }
}
