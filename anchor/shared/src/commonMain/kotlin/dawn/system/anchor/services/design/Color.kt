package dawn.system.anchor.services.design

import androidx.compose.runtime.Immutable
import androidx.compose.ui.graphics.Color

/**
 * Aladin's colour tokens — the SAME vocabulary the desktop app and the shard runtime use
 * (`aladin_react/src/index.css`, `backend_v2/internal/docsurface/tokens.go`), so a value
 * named here means the same thing on every surface Aladin renders.
 *
 * This is the only place a hex literal may appear. Features address colour by role
 * (`rail`, `panel`, `ink2`, `amber`), never by value — that is what lets a theme switch
 * without touching a feature.
 *
 * `learn` is the one token the iPad design adds: the learning purpose, and Pencil ink.
 */
@Immutable
data class AladinColors(
    val isLight: Boolean,
    // surfaces, back to front
    val rail: Color,
    val panel: Color,
    val bg: Color,
    val chrome: Color,
    val card: Color,
    val field: Color,
    val raise: Color,
    // ink ramp, strongest → faintest
    val ink: Color,
    val ink2: Color,
    val ink3: Color,
    val ink4: Color,
    // lines + states (already alpha-composited over the surface they sit on)
    val line: Color,
    val line2: Color,
    val hover: Color,
    val sel: Color,
    // accent
    val amber: Color,
    val amberSoft: Color,
    val amberLine: Color,
    val onAmber: Color,
    // the learning purpose + handwriting
    val learn: Color,
    val learnSoft: Color,
    val learnLine: Color,
    // semantic
    val forColor: Color,
    val against: Color,
) {
    /** Ink for content sitting on an amber fill. */
    val onAccent: Color get() = onAmber
}

private fun white(alpha: Float) = Color(0xFFFFFFFF).copy(alpha = alpha)
private fun black(alpha: Float) = Color(0xFF000000).copy(alpha = alpha)

/** Default. Matches the handoff's `[data-theme="dark"]`. */
val AladinDark = AladinColors(
    isLight = false,
    rail = Color(0xFF0B0B0E),
    panel = Color(0xFF0F0F12),
    bg = Color(0xFF0D0D10),
    chrome = Color(0xFF0B0B0E),
    card = Color(0xFF121215),
    field = Color(0xFF161619),
    raise = Color(0xFF17171C),
    ink = Color(0xFFECEAEF),
    ink2 = Color(0xFF9694A0),
    ink3 = Color(0xFF615F6B),
    ink4 = Color(0xFF403E48),
    line = white(0.07f),
    line2 = white(0.045f),
    hover = white(0.035f),
    sel = white(0.06f),
    amber = Color(0xFFC9925A),
    amberSoft = Color(0xFFC9925A).copy(alpha = 0.12f),
    amberLine = Color(0xFFC9925A).copy(alpha = 0.30f),
    onAmber = Color(0xFF0F0F12),
    learn = Color(0xFF8FB9E8),
    learnSoft = Color(0xFF8FB9E8).copy(alpha = 0.12f),
    learnLine = Color(0xFF8FB9E8).copy(alpha = 0.32f),
    forColor = Color(0xFF5CBA8F),
    against = Color(0xFFD8796B),
)

/**
 * Light. The handoff ships dark plus `apple-dark`/light variants and says to map them onto
 * the codebase's theme system; this is the light mapping of the same roles.
 */
val AladinLight = AladinColors(
    isLight = true,
    rail = Color(0xFFF2F1EE),
    panel = Color(0xFFF7F6F3),
    bg = Color(0xFFFBFAF8),
    chrome = Color(0xFFF2F1EE),
    card = Color(0xFFFFFFFF),
    field = Color(0xFFEFEEEA),
    raise = Color(0xFFE9E8E3),
    ink = Color(0xFF1A1A1E),
    ink2 = Color(0xFF56545E),
    ink3 = Color(0xFF87858F),
    ink4 = Color(0xFFA9A7B0),
    line = black(0.09f),
    line2 = black(0.055f),
    hover = black(0.04f),
    sel = black(0.06f),
    amber = Color(0xFF9C6B33),
    amberSoft = Color(0xFF9C6B33).copy(alpha = 0.12f),
    amberLine = Color(0xFF9C6B33).copy(alpha = 0.30f),
    onAmber = Color(0xFFFBFAF8),
    learn = Color(0xFF3E6FA3),
    learnSoft = Color(0xFF3E6FA3).copy(alpha = 0.12f),
    learnLine = Color(0xFF3E6FA3).copy(alpha = 0.32f),
    forColor = Color(0xFF2F7F5B),
    against = Color(0xFFB2483A),
)
