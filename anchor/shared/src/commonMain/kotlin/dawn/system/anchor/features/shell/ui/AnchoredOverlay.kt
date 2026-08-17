package dawn.system.anchor.features.shell.ui

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.CubicBezierEasing
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.scaleIn
import androidx.compose.animation.scaleOut
import androidx.compose.animation.slideInVertically
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.clickable
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import dawn.system.anchor.services.design.AnchorShape
import dawn.system.anchor.services.design.AnchorTheme

/**
 * Where an overlay hangs.
 *
 * Two placements, and the second is not a detail: with the sidebar collapsed there is no icon
 * row to hang under, so every overlay re-anchors above the floating dock instead. Anything
 * anchored to a button that can disappear needs both.
 */
internal sealed interface OverlayAnchor {

    /** Under a button in the sidebar's icon row. [top] shifts down when filter pills show. */
    data class Below(val left: Dp, val top: Dp) : OverlayAnchor

    /** Above the floating dock, horizontally centred on it. */
    data class AboveDock(val bottom: Dp) : OverlayAnchor
}

/**
 * A panel hanging off a control — **not** a modal, and the difference is the whole design.
 *
 * The browser dropdown is the reason this exists: *"it must not read as a modal"*. Anchored to
 * its icon, undimmed, dismissing back into exactly what you were doing. A centred sheet over a
 * scrim says "you have left your document to do something else"; this says "your document is
 * still there, and here is a thing over it".
 *
 * So there is **no scrim**. What there is instead is a transparent full-frame catcher — an
 * invisible surface whose only job is to turn a tap anywhere else into a dismissal. It is not
 * dimming and it is not a barrier you have to acknowledge; it is the shape of "tap away to
 * leave", which every overlay here honours.
 *
 * ### The catcher and native views
 *
 * The content behind may be a `WKWebView` or a `PDFView`, and Compose's interop puts those in
 * the UIKit hierarchy rather than drawing them. A Compose catcher above one therefore has to
 * actually win the touch, or tap-outside-to-dismiss fails precisely when a document is open —
 * the main case. It does win: the catcher is a Compose node above the interop element in the
 * same composition, and interop hit-testing is bounded by its own layout. Verified on device
 * rather than assumed, for both halves: unpinned dismisses over a live editor, and pinned
 * leaves that editor scrollable.
 *
 * ### Ordering
 *
 * Compose this at the **shell's** top-level box, never inside a pager page: a parked page is
 * placed two window-widths away, and its overlay would park with it.
 */
@Composable
internal fun BoxScope.AnchoredOverlay(
    visible: Boolean,
    anchor: OverlayAnchor,
    onDismiss: () -> Unit,
    modifier: Modifier = Modifier,
    borderColor: Color? = null,
    elevation: Dp = OVERLAY_SHADOW,
    shape: Shape = AnchorShape.popover,
    content: @Composable () -> Unit,
) {
    val c = AnchorTheme.colors

    if (visible) {
        // No ripple and no indication: this is not a control, it is the absence of one.
        val interaction = remember { MutableInteractionSource() }
        Box(
            Modifier
                .fillMaxSize()
                .clickable(interaction, indication = null, onClick = onDismiss),
        )
    }

    val alignment = when (anchor) {
        is OverlayAnchor.Below -> Alignment.TopStart
        is OverlayAnchor.AboveDock -> Alignment.BottomCenter
    }

    Box(Modifier.align(alignment)) {
        AnimatedVisibility(
            visible = visible,
            enter = POP_IN_FADE + POP_IN_RISE + POP_IN_SCALE,
            exit = fadeOut(tween(POP_MILLIS)) + scaleOut(tween(POP_MILLIS), targetScale = 0.99f),
        ) {
            Box(
                modifier
                    .then(
                        when (anchor) {
                            is OverlayAnchor.Below -> Modifier.offset(x = anchor.left, y = anchor.top)
                            is OverlayAnchor.AboveDock -> Modifier.padding(bottom = anchor.bottom)
                        },
                    )
                    .shadow(elevation, shape)
                    .clip(shape)
                    .background(c.card)
                    .then(
                        borderColor?.let { Modifier.border(1.dp, it, shape) } ?: Modifier,
                    ),
            ) {
                content()
            }
        }
    }
}

/** `popIn`: rises and settles, fast enough to feel like a reveal rather than a transition. */
private const val POP_MILLIS = 160
private val POP_EASING = CubicBezierEasing(0.2f, 0.8f, 0.2f, 1f)
private val POP_IN_FADE = fadeIn(tween(POP_MILLIS, easing = POP_EASING))
private val POP_IN_RISE =
    slideInVertically(tween(POP_MILLIS, easing = POP_EASING)) { height -> (height * 0.02f).toInt() + 12 }
private val POP_IN_SCALE = scaleIn(tween(POP_MILLIS, easing = POP_EASING), initialScale = 0.99f)

/** The design's overlay shadow. Deep, because an undimmed overlay has only depth to separate it. */
private val OVERLAY_SHADOW = 24.dp
