package com.jvp.aladin_compose.ui_lib

import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography

object AladinDark {
    val Background = Color(0xFF050505)
    val Surface = Color(0xFF0D0D0D)
    val SurfaceRaised = Color(0xFF181818)
    val Border = Color(0xFF2A2A2A)
    val BorderStrong = Color(0xFFFFFFFF)
    val TextMuted = Color(0xFF7A7A7A)
    val TextSecondary = Color(0xFFBDBDBD)
    val TextPrimary = Color(0xFFFFFFFF)
    val TextInverse = Color(0xFF050505)
}

object AladinLight {
    val Background = Color(0xFFFAFAFA)
    val Surface = Color(0xFFFDFDFD)
    val SurfaceRaised = Color(0xFFF1F1F1)
    val Border = Color(0xFFE2E2E2)
    val BorderStrong = Color(0xFF000000)
    val TextMuted = Color(0xFF8A8A8A)
    val TextSecondary = Color(0xFF555555)
    val TextPrimary = Color(0xFF000000)
    val TextInverse = Color(0xFFF7F7F7)
}

object AladinColor {
    val Canvas = Color(0xFFFAFAFA)
    val Panel = Color(0xFFFDFDFD)
    val PanelMuted = Color(0xFFF7F7F7)

    val RowHover = Color(0xFFF3F3F3)
    val RowSelected = Color(0xFFEDEDED)
    val ControlHover = Color(0xFFF1F1F1)
    val ControlPressed = Color(0xFFE7E7E7)

    val Divider = Color(0xFFE8E8E8)
    val Border = Color(0xFFDADADA)

    val Ink = Color(0xFF000000)
    val InkSecondary = Color(0xFF555555)
    val InkMuted = Color(0xFF8A8A8A)
    val InkDisabled = Color(0xFFB8B8B8)

    val InkSurface = Color(0xFF333333)
    val InkSurfaceHover = Color(0xFF242424)
    val OnInkSurface = Color(0xFFF7F7F7)
}

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

    error                = AladinDark.TextPrimary,
    onError              = AladinDark.TextInverse,
    errorContainer       = AladinDark.SurfaceRaised,
    onErrorContainer     = AladinDark.TextPrimary,

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
    primary              = AladinColor.Ink,
    onPrimary            = AladinColor.OnInkSurface,
    primaryContainer     = AladinColor.RowSelected,
    onPrimaryContainer   = AladinColor.Ink,

    secondary            = AladinColor.InkSecondary,
    onSecondary          = AladinColor.OnInkSurface,
    secondaryContainer   = AladinColor.ControlHover,
    onSecondaryContainer = AladinColor.Ink,

    tertiary             = AladinColor.InkSecondary,
    onTertiary           = AladinColor.OnInkSurface,
    tertiaryContainer    = AladinColor.ControlHover,
    onTertiaryContainer  = AladinColor.Ink,

    error                = AladinColor.Ink,
    onError              = AladinColor.OnInkSurface,
    errorContainer       = AladinColor.ControlHover,
    onErrorContainer     = AladinColor.Ink,

    background           = AladinColor.Canvas,
    onBackground         = AladinColor.Ink,

    surface              = AladinColor.Panel,
    onSurface            = AladinColor.Ink,
    surfaceVariant       = AladinColor.ControlHover,
    onSurfaceVariant     = AladinColor.InkSecondary,

    outline              = AladinColor.Border,
    outlineVariant       = AladinColor.Divider,

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
