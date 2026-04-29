package com.jvp.aladin_compose.ui_lib

import androidx.compose.runtime.Composable
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.graphics.Color
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography
import androidx.compose.ui.unit.sp

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
    val Canvas = Color(0xFFFFFFFF)
    val Panel = Color(0xFFFCFCFB)
    val PanelMuted = Color(0xFFF5F5F2)

    val RowHover = Color(0xFFF3F3F0)
    val RowSelected = Color(0xFFEDEDE9)
    val ControlHover = Color(0xFFF2F2EF)
    val ControlPressed = Color(0xFFE6E6E1)

    val Divider = Color(0xFFE7E7E2)
    val Border = Color(0xFFD8D8D2)

    val Ink = Color(0xFF111111)
    val InkSecondary = Color(0xFF4F4F4A)
    val InkMuted = Color(0xFF7C7C75)
    val InkDisabled = Color(0xFFB2B2AA)

    val InkSurface = Color(0xFF151515)
    val InkSurfaceHover = Color(0xFF0D0D0D)
    val OnInkSurface = Color(0xFFF7F7F4)

    val ActiveMarker = Color(0xFF111111)
    val CommandSurface = Color(0xFFF5F5F2)
    val CodeText = Color(0xFF32322E)
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

val AladinTypography =
    Typography(
        headlineLarge =
            TextStyle(
                fontFamily = FontFamily.SansSerif,
                fontWeight = FontWeight.Bold,
                fontSize = 32.sp,
                lineHeight = 36.sp,
                letterSpacing = (-0.8).sp,
            ),
        headlineSmall =
            TextStyle(
                fontFamily = FontFamily.SansSerif,
                fontWeight = FontWeight.Bold,
                fontSize = 24.sp,
                lineHeight = 28.sp,
                letterSpacing = (-0.4).sp,
            ),
        titleMedium =
            TextStyle(
                fontFamily = FontFamily.SansSerif,
                fontWeight = FontWeight.SemiBold,
                fontSize = 18.sp,
                lineHeight = 24.sp,
                letterSpacing = (-0.2).sp,
            ),
        titleSmall =
            TextStyle(
                fontFamily = FontFamily.SansSerif,
                fontWeight = FontWeight.SemiBold,
                fontSize = 15.sp,
                lineHeight = 20.sp,
            ),
        bodyLarge =
            TextStyle(
                fontFamily = FontFamily.SansSerif,
                fontWeight = FontWeight.Normal,
                fontSize = 17.sp,
                lineHeight = 26.sp,
            ),
        bodyMedium =
            TextStyle(
                fontFamily = FontFamily.SansSerif,
                fontWeight = FontWeight.Normal,
                fontSize = 15.sp,
                lineHeight = 23.sp,
            ),
        bodySmall =
            TextStyle(
                fontFamily = FontFamily.SansSerif,
                fontWeight = FontWeight.Normal,
                fontSize = 13.sp,
                lineHeight = 18.sp,
            ),
        labelLarge =
            TextStyle(
                fontFamily = FontFamily.SansSerif,
                fontWeight = FontWeight.Medium,
                fontSize = 14.sp,
                lineHeight = 18.sp,
            ),
        labelMedium =
            TextStyle(
                fontFamily = FontFamily.Monospace,
                fontWeight = FontWeight.Medium,
                fontSize = 12.sp,
                lineHeight = 16.sp,
                letterSpacing = 0.2.sp,
            ),
    )

@Composable
fun AladinTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = AladinLightColorScheme,
        typography  = AladinTypography,
        content     = content
    )
}
