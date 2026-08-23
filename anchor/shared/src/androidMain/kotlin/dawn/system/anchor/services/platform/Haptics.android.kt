package dawn.system.anchor.services.platform

/** Android has no haptic wiring yet; the companion's v1 target is iPad. */
actual fun performHaptic(haptic: Haptic) = Unit
