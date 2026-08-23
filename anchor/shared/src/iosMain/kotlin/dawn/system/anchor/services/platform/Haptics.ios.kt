package dawn.system.anchor.services.platform

import platform.UIKit.UIImpactFeedbackGenerator
import platform.UIKit.UIImpactFeedbackStyle
import platform.UIKit.UISelectionFeedbackGenerator

// Generators are cheap to keep and expensive to rebuild per tap; one of each for the app.
private val light by lazy { UIImpactFeedbackGenerator(UIImpactFeedbackStyle.UIImpactFeedbackStyleLight) }
private val medium by lazy { UIImpactFeedbackGenerator(UIImpactFeedbackStyle.UIImpactFeedbackStyleMedium) }
private val selection by lazy { UISelectionFeedbackGenerator() }

actual fun performHaptic(haptic: Haptic) {
    when (haptic) {
        Haptic.Light -> light.impactOccurred()
        Haptic.Medium -> medium.impactOccurred()
        Haptic.Select -> selection.selectionChanged()
    }
}
