package dawn.system.anchor.services.platform

/**
 * The three haptic weights the companion plays. Sparse on purpose: a tick when a tap changed
 * something (tool, insert, flip), an impact for a bigger commit — never per stroke, never
 * per scroll. The board asks for these over the bridge; native surfaces call it directly.
 */
enum class Haptic { Light, Medium, Select }

/** Plays [haptic] on the device. A no-op where the platform has no engine. */
expect fun performHaptic(haptic: Haptic)
