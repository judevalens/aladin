package dawn.system.anchor.services.design

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.staticCompositionLocalOf

/**
 * Theme entry point. Wrap the app once; read tokens through [AnchorTheme].
 *
 *   AnchorTheme.colors.panel · AnchorTheme.metrics.railWidth · AnchorShape.railCell
 *
 * A subset of the tokens is mapped onto a Material3 scheme so stock Material components
 * (text fields, buttons) inherit the theme instead of fighting it. Features should still
 * address [AnchorTheme.colors] directly — the Material mapping is a compatibility shim,
 * not the vocabulary.
 */
private val LocalAladinColors = staticCompositionLocalOf { AladinDark }

object AnchorTheme {
    val colors: AladinColors
        @Composable get() = LocalAladinColors.current
    val metrics: AladinMetrics
        @Composable get() = LocalAladinMetrics.current
    val space: AnchorSpace
        @Composable get() = LocalAnchorSpace.current
}

@Composable
fun AnchorTheme(
    darkTheme: Boolean = isSystemInDarkTheme(),
    palette: AladinColors? = null,
    content: @Composable () -> Unit,
) {
    val colors = palette ?: if (darkTheme) AladinDark else AladinLight

    val material = with(colors) {
        if (isLight) {
            lightColorScheme(
                primary = amber, onPrimary = onAmber,
                primaryContainer = amberSoft, onPrimaryContainer = ink,
                secondary = learn, onSecondary = onAmber,
                secondaryContainer = learnSoft, onSecondaryContainer = ink,
                background = bg, onBackground = ink,
                surface = card, onSurface = ink,
                surfaceVariant = field, onSurfaceVariant = ink2,
                error = against, onError = onAmber,
                outline = line, outlineVariant = line2,
            )
        } else {
            darkColorScheme(
                primary = amber, onPrimary = onAmber,
                primaryContainer = amberSoft, onPrimaryContainer = ink,
                secondary = learn, onSecondary = onAmber,
                secondaryContainer = learnSoft, onSecondaryContainer = ink,
                background = bg, onBackground = ink,
                surface = card, onSurface = ink,
                surfaceVariant = field, onSurfaceVariant = ink2,
                error = against, onError = onAmber,
                outline = line, outlineVariant = line2,
            )
        }
    }

    CompositionLocalProvider(
        LocalAladinColors provides colors,
        LocalAladinMetrics provides AladinMetrics,
        LocalAnchorSpace provides AnchorSpace,
    ) {
        MaterialTheme(
            colorScheme = material,
            typography = AnchorTypography,
            shapes = AnchorShapes,
            content = content,
        )
    }
}
