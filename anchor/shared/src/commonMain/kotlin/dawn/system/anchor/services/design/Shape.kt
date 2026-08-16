package dawn.system.anchor.services.design

import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Shapes
import androidx.compose.ui.unit.dp

/**
 * Radii from the handoff: 11–12 small controls · 13–15 buttons and rows · 17 capsules ·
 * 18–22 cards and popovers · 26 sheets. Named by role so a feature never picks a number.
 */
object AnchorShape {
    val control = RoundedCornerShape(11.dp)
    val chip = RoundedCornerShape(12.dp)
    val field = RoundedCornerShape(13.dp)
    /** Root-menu section row. */
    val menuRow = RoundedCornerShape(14.dp)
    val row = RoundedCornerShape(15.dp)
    /** The copilot hero, collapsed to one capsule. */
    val capsule = RoundedCornerShape(17.dp)
    val card = RoundedCornerShape(18.dp)
    val popover = RoundedCornerShape(18.dp)
    /** The copilot hero, expanded. */
    val heroCard = RoundedCornerShape(22.dp)
    val sheet = RoundedCornerShape(topStart = 26.dp, topEnd = 26.dp)
    val pill = RoundedCornerShape(percent = 50)
}

val AnchorShapes = Shapes(
    extraSmall = AnchorShape.control,
    small = AnchorShape.field,
    medium = AnchorShape.row,
    large = AnchorShape.card,
    extraLarge = AnchorShape.popover,
)
