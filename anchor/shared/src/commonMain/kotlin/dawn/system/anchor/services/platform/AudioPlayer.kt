package dawn.system.anchor.services.platform

import androidx.compose.runtime.Composable
import androidx.compose.runtime.Stable

/**
 * Playback state for one audio file, driven by the platform's player.
 *
 * A *transport* rather than a video surface: `AVPlayerViewController` is built for video, so
 * it renders a black rectangle whose controls auto-hide — you have to tap it to discover it
 * is a player at all. A voice note wants what desktop's `<audio controls>` gives: visible
 * play/pause, elapsed time, and a scrubber, in the app's own type and colour.
 */
@Stable
interface AudioController {
    val playing: Boolean
    /** Seconds. Zero until the asset reports otherwise. */
    val position: Double
    val duration: Double
    /** True once the asset is loaded far enough to play. */
    val ready: Boolean

    /**
     * Why this file will never play, if it won't — an undecodable container, a missing
     * file. Null when there is nothing wrong.
     *
     * A player that can only be silent has to say so. Not every recording in the
     * workspace is playable here: the desktop capture falls back to WebM/Opus on
     * Chromium, and no Apple platform can decode it.
     */
    val failure: String?

    /** Playback speed: 1× · 1.25× · 1.5× · 2×. */
    val rate: Float

    fun toggle()
    /** Seeks to [fraction] of [duration], clamped to 0..1. */
    fun seek(fraction: Float)
    /** Seeks [seconds] relative to now — negative rewinds. Clamped to the recording. */
    fun skip(seconds: Double)
    /** Advances to the next speed in the cycle. */
    fun cycleRate()
}

/** The speeds the transport cycles through, in order. */
val PLAYBACK_RATES = listOf(1f, 1.25f, 1.5f, 2f)

/** `1×` · `1.25×`. Trailing zeros dropped — `1.50×` reads like a price. */
fun formatRate(rate: Float): String {
    val text = if (rate % 1f == 0f) rate.toInt().toString() else rate.toString().trimEnd('0').trimEnd('.')
    return "${text}×"
}

/**
 * Where each recording was left, so reopening a note resumes rather than restarts.
 *
 * In memory and process-scoped on purpose: a voice surface can be unmounted by the
 * keep-alive cap while you are elsewhere, which is exactly when the player object — and
 * with it the position — goes away. Persisting across launches is a later question; not
 * losing your place mid-session is the one that bites.
 */
object PlaybackPositions {
    private val byArtifact = mutableMapOf<String, Double>()

    fun remember(artifactId: String, seconds: Double) {
        byArtifact[artifactId] = seconds
    }

    fun of(artifactId: String): Double = byArtifact[artifactId] ?: 0.0
}

/**
 * Binds a player to [filePath] for as long as this stays in composition, releasing it on
 * the way out — an audio session left running after you navigate away keeps the now-playing
 * controls alive on the lock screen, which is a bug the user experiences much later.
 */
@Composable
expect fun rememberAudioController(filePath: String, startAt: Double = 0.0): AudioController

/** `01:23`. Negative or unknown durations render as `--:--` rather than nonsense. */
fun formatTimecode(seconds: Double): String {
    if (seconds.isNaN() || seconds < 0) return "--:--"
    val total = seconds.toLong()
    val minutes = total / 60
    val remainder = total % 60
    return "${minutes.toString().padStart(2, '0')}:${remainder.toString().padStart(2, '0')}"
}
