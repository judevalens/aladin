package dawn.system.anchor.services.platform

import androidx.compose.runtime.Composable

/** Android has no audio surface yet; the companion's v1 target is iPad. */
@Composable
actual fun rememberAudioController(filePath: String, startAt: Double): AudioController =
    object : AudioController {
        override val playing = false
        override val position = 0.0
        override val duration = 0.0
        override val ready = false
        override val failure: String? = "audio playback is not implemented on Android"
        override val rate = 1f
        override fun toggle() = Unit
        override fun seek(fraction: Float) = Unit
        override fun skip(seconds: Double) = Unit
        override fun cycleRate() = Unit
    }
