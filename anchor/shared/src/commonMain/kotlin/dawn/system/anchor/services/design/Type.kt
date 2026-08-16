package dawn.system.anchor.services.design

import androidx.compose.material3.Typography
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp

/**
 * Type scale from the iPad handoff.
 *
 * The handoff calls for Space Grotesk (titles), JetBrains Mono (metadata and citations),
 * Caveat (handwriting) and Georgia (source text and formulas — the "printed" voice).
 * **Those families are not bundled yet**, so [Display] falls back to the platform sans and
 * [Mono] to the platform monospace. The *scale, weight and tracking* are the design's, so
 * hierarchy and density are right and only the letterforms differ; dropping the real
 * families into `composeResources/font/` later changes these two properties and nothing
 * else.
 */
val Display: FontFamily = FontFamily.Default
val Body: FontFamily = FontFamily.Default
val Mono: FontFamily = FontFamily.Monospace
val Serif: FontFamily = FontFamily.Serif

val AnchorTypography = Typography(
    // Sidebar title ("Folders"). The handoff draws this at 25/700 in Space Grotesk, which
    // is narrower than the platform sans standing in for it — at 25sp a folder name eats
    // the row before the Back chevron and the count get their share. 21 keeps the
    // hierarchy (still the largest thing on screen) at a width the sidebar actually has.
    headlineMedium = TextStyle(
        fontFamily = Display, fontWeight = FontWeight.Bold,
        fontSize = 21.sp, lineHeight = 26.sp, letterSpacing = (-0.4).sp,
    ),
    // Detail header title — one step below the sidebar's, as in the handoff.
    headlineSmall = TextStyle(
        fontFamily = Display, fontWeight = FontWeight.SemiBold,
        fontSize = 19.sp, lineHeight = 24.sp, letterSpacing = (-0.3).sp,
    ),
    // Stat card value — 23, tabular feel.
    titleLarge = TextStyle(
        fontFamily = Display, fontWeight = FontWeight.SemiBold,
        fontSize = 23.sp, lineHeight = 27.sp, letterSpacing = (-0.2).sp,
    ),
    // Row title — 16/500.
    titleMedium = TextStyle(
        fontFamily = Body, fontWeight = FontWeight.Medium,
        fontSize = 16.sp, lineHeight = 21.sp,
    ),
    // A single prose statement (hypothesis, objective) — 19.
    bodyLarge = TextStyle(
        fontFamily = Body, fontWeight = FontWeight.Normal,
        fontSize = 19.sp, lineHeight = 27.sp,
    ),
    // Body / secondary lines.
    bodyMedium = TextStyle(
        fontFamily = Body, fontWeight = FontWeight.Normal,
        fontSize = 14.sp, lineHeight = 20.sp,
    ),
    bodySmall = TextStyle(
        fontFamily = Body, fontWeight = FontWeight.Normal,
        fontSize = 12.5.sp, lineHeight = 17.sp,
    ),
    // Buttons and pills.
    labelLarge = TextStyle(
        fontFamily = Body, fontWeight = FontWeight.Medium,
        fontSize = 14.sp, lineHeight = 18.sp,
    ),
)

/** Metadata, counts, citations: mono, small, wide-tracked. */
val MetaStyle = TextStyle(
    fontFamily = Mono, fontWeight = FontWeight.Normal,
    fontSize = 11.5.sp, lineHeight = 15.sp, letterSpacing = 0.2.sp,
)

/** Section labels: mono, uppercase, .09em tracking per the handoff. */
val SectionLabelStyle = TextStyle(
    fontFamily = Mono, fontWeight = FontWeight.Medium,
    fontSize = 10.5.sp, lineHeight = 14.sp, letterSpacing = 0.95.sp,
)

/**
 * Source text — a captured excerpt, a quotation, a page's body.
 *
 * Serif on purpose: the handoff gives Georgia to everything that is *printed* rather than
 * authored by the app, so the reader can tell a captured page from Aladin's own chrome
 * without being told.
 */
val SourceTextStyle = TextStyle(
    fontFamily = Serif, fontWeight = FontWeight.Normal,
    fontSize = 17.sp, lineHeight = 29.75.sp,
)

/** Chips (purpose, kind): mono, uppercase, tiny. */
val ChipStyle = TextStyle(
    fontFamily = Mono, fontWeight = FontWeight.Medium,
    fontSize = 9.5.sp, lineHeight = 12.sp, letterSpacing = 0.7.sp,
)
