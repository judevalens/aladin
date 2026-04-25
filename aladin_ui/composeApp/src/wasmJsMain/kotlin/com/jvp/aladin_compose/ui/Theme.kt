package com.jvp.aladin_compose.ui

import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme

// ─── Anchor values ───────────────────────────────────────────────────────────
// Background : #0D0D0D
// Surface    : #262626
// Text       : #BFBDB8

// ─── Dark ────────────────────────────────────────────────────────────────────
object AladinDark {
    val Background    = Color(0xFF111111)  // anchor
    val Surface       = Color(0xFF121212)  // midpoint between bg and raised
    val SurfaceRaised = Color(0xFF262626)  // anchor
    val Border        = Color(0xFF303030)
    val BorderStrong  = Color(0xFF3D3D3D)
    val TextMuted     = Color(0xFFA3A3A3)  // warm muted
    val TextSecondary = Color(0xFFCCCBCB)  // warm mid
    val TextPrimary   = Color(0xFFEEEEEE)  // anchor
    val TextInverse   = Color(0xFF0D0D0D)
}

// ─── Light ───────────────────────────────────────────────────────────────────
// Derived from the same warm temperature, inverted
object AladinLight {
    val Background    = Color(0xFFF8F9FA)  // warm white
    val Surface       = Color(0xFFE9ECEF)
    val SurfaceRaised = Color(0xFFDEE2E6)  // warm off-white
    val Border        = Color(0xFFD8D5CE)
    val BorderStrong  = Color(0xFFB9B4A9)
    val TextMuted     = Color(0xFF8F8A82)
    val TextSecondary = Color(0xFF343A40)  // shared mid
    val TextPrimary   = Color(0xFF212529)  // warm near-black
    val TextInverse   = Color(0xFFE9ECEF)
}

// ─── Material3 Dark ──────────────────────────────────────────────────────────
val AladinDarkColorScheme = darkColorScheme(
    primary              = AladinDark.TextPrimary,
    onPrimary            = AladinDark.TextInverse,
    primaryContainer     = AladinDark.SurfaceRaised,
    onPrimaryContainer   = AladinDark.TextPrimary,

    secondary            = AladinDark.TextSecondary,
    onSecondary          = AladinDark.TextInverse,
    secondaryContainer   = AladinDark.Border,
    onSecondaryContainer = AladinDark.TextPrimary,

    tertiary             = AladinDark.TextSecondary,
    onTertiary           = AladinDark.TextInverse,
    tertiaryContainer    = AladinDark.SurfaceRaised,
    onTertiaryContainer  = AladinDark.TextPrimary,

    error                = Color(0xFFFF6B6B),
    onError              = AladinDark.TextInverse,
    errorContainer       = Color(0xFF2E1111),
    onErrorContainer     = Color(0xFFFFAAAA),

    background           = AladinDark.Background,
    onBackground         = AladinDark.TextPrimary,

    surface              = AladinDark.Surface,
    onSurface            = AladinDark.TextPrimary,
    surfaceVariant       = AladinDark.SurfaceRaised,
    onSurfaceVariant     = AladinDark.TextSecondary,

    outline              = AladinDark.Border,
    outlineVariant       = AladinDark.BorderStrong,

    scrim                = Color(0xCC000000),
    inverseSurface       = AladinLight.Surface,
    inverseOnSurface     = AladinLight.TextPrimary,
    inversePrimary       = AladinLight.TextPrimary,
)

// ─── Material3 Light ─────────────────────────────────────────────────────────
val AladinLightColorScheme = lightColorScheme(
    primary              = AladinLight.TextPrimary,
    onPrimary            = AladinLight.TextInverse,
    primaryContainer     = AladinLight.SurfaceRaised,
    onPrimaryContainer   = AladinLight.TextPrimary,

    secondary            = AladinLight.TextSecondary,
    onSecondary          = AladinLight.TextInverse,
    secondaryContainer   = AladinLight.Border,
    onSecondaryContainer = AladinLight.TextPrimary,

    tertiary             = AladinLight.TextSecondary,
    onTertiary           = AladinLight.TextInverse,
    tertiaryContainer    = AladinLight.SurfaceRaised,
    onTertiaryContainer  = AladinLight.TextPrimary,

    error                = Color(0xFFBB1111),
    onError              = AladinLight.TextInverse,
    errorContainer       = Color(0xFFFFEAEA),
    onErrorContainer     = Color(0xFF6E0000),

    background           = AladinLight.Background,
    onBackground         = AladinLight.TextPrimary,

    surface              = AladinLight.Surface,
    onSurface            = AladinLight.TextPrimary,
    surfaceVariant       = AladinLight.SurfaceRaised,
    onSurfaceVariant     = AladinLight.TextSecondary,

    outline              = AladinLight.Border,
    outlineVariant       = AladinLight.BorderStrong,

    scrim                = Color(0x80000000),
    inverseSurface       = AladinDark.Surface,
    inverseOnSurface     = AladinDark.TextPrimary,
    inversePrimary       = AladinDark.TextPrimary,
)

@Composable
fun AladinTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = AladinLightColorScheme,
        typography  = Typography(),
        content     = content
    )
}
