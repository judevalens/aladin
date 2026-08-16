package dawn.system.anchor.services.platform

import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import kotlinx.cinterop.BetaInteropApi
import kotlinx.cinterop.ExperimentalForeignApi
import kotlinx.cinterop.ObjCObjectVar
import kotlinx.cinterop.alloc
import kotlinx.cinterop.memScoped
import kotlinx.cinterop.ptr
import kotlinx.cinterop.value
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import platform.AVFAudio.AVAudioPlayer
import platform.AVFAudio.AVAudioSession
import platform.AVFAudio.AVAudioSessionCategoryPlayback
import platform.AVFAudio.setActive
import platform.Foundation.NSError
import platform.Foundation.NSURL
import kotlin.coroutines.coroutineContext

/**
 * `AVAudioPlayer`, not `AVPlayer`, and the difference is the whole bug.
 *
 * `AVPlayer` loads asynchronously: its duration is unknown until the item becomes ready,
 * and its periodic time observer only really ticks once playback has started. Gating the
 * play button on "ready" therefore deadlocked — the transport could not be tapped until
 * the asset loaded, and the asset would not load until it was tapped.
 *
 * `AVAudioPlayer` is the local-file player: duration is available the moment it is
 * constructed, so the scrubber is honest before the first tap, and — the part that matters
 * most here — construction *fails with an error* on a container iOS cannot decode instead
 * of succeeding and then being silent forever.
 */
@OptIn(ExperimentalForeignApi::class)
@Composable
actual fun rememberAudioController(filePath: String, startAt: Double): AudioController {
    val controller = remember(filePath) { IosAudioController(filePath, startAt) }

    DisposableEffect(controller) {
        // Releasing matters: a live player keeps the audio session — and the lock-screen
        // now-playing controls — alive after you have navigated away.
        onDispose { controller.release() }
    }

    // The player has no callback for "time advanced", so the transport ticks instead. This
    // is a 4 Hz UI animation over an in-process player, not a network poll: it exists only
    // while a recording is actually playing, and stops the moment it isn't.
    LaunchedEffect(controller) {
        while (coroutineContext.isActive) {
            if (controller.playing) controller.tick()
            delay(TICK_MILLIS)
        }
    }

    return controller
}

private const val TICK_MILLIS = 250L

@OptIn(ExperimentalForeignApi::class)
private class IosAudioController(filePath: String, startAt: Double) : AudioController {

    private val player: AVAudioPlayer?

    override var playing: Boolean by mutableStateOf(false)
        private set
    override var position: Double by mutableStateOf(0.0)
        private set
    override var duration: Double by mutableStateOf(0.0)
        private set
    override var ready: Boolean by mutableStateOf(false)
        private set
    override var failure: String? by mutableStateOf(null)
        private set
    override var rate: Float by mutableStateOf(PLAYBACK_RATES.first())
        private set

    init {
        // Without the playback category, iOS honours the silent switch and a voice note is
        // simply inaudible with nothing on screen to explain it.
        runCatching {
            AVAudioSession.sharedInstance().setCategory(AVAudioSessionCategoryPlayback, null)
            AVAudioSession.sharedInstance().setActive(true, null)
        }

        val (opened, error) = open(filePath)
        player = opened
        if (opened == null) {
            failure = error ?: "this recording is in a format this device cannot play"
        } else {
            opened.prepareToPlay()
            duration = opened.duration
            ready = true
        }
    }

    /**
     * Opens the file, keeping the `NSError` and the failure in the same scope.
     *
     * Kotlin/Native types this initializer as non-null and turns a nil return into a thrown
     * exception, so both outcomes have to be caught here — and the error has to be read
     * before [memScoped] frees it.
     */
    @OptIn(BetaInteropApi::class)
    private fun open(filePath: String): Pair<AVAudioPlayer?, String?> = memScoped {
        val error = alloc<ObjCObjectVar<NSError?>>()
        val opened = runCatching {
            AVAudioPlayer(contentsOfURL = NSURL.fileURLWithPath(filePath), error = error.ptr)
        }.getOrNull()
        opened to error.value?.localizedDescription
    }

    override fun toggle() {
        val player = player ?: return
        if (playing) {
            player.pause()
            playing = false
        } else {
            // A finished recording restarts rather than doing nothing at the end.
            if (player.currentTime >= player.duration - END_EPSILON) player.currentTime = 0.0
            playing = player.play()
            // `play()` resets the rate, so the chosen speed has to be re-applied after it
            // rather than before.
            if (playing) player.setRate(rate) else failure = "playback would not start"
        }
        position = player.currentTime
    }

    override fun seek(fraction: Float) {
        val player = player ?: return
        player.currentTime = fraction.coerceIn(0f, 1f) * player.duration
        position = player.currentTime
    }

    override fun skip(seconds: Double) {
        val player = player ?: return
        player.currentTime = (player.currentTime + seconds).coerceIn(0.0, player.duration)
        position = player.currentTime
    }

    override fun cycleRate() {
        val next = PLAYBACK_RATES[(PLAYBACK_RATES.indexOf(rate) + 1) % PLAYBACK_RATES.size]
        rate = next
        player?.setRate(next)
    }

    /** One transport tick: pull the clock, and notice when the recording has run out. */
    fun tick() {
        val player = player ?: return
        position = player.currentTime
        if (!player.playing) playing = false
    }

    fun release() {
        player?.stop()
        playing = false
        runCatching { AVAudioSession.sharedInstance().setActive(false, null) }
    }

    private companion object {
        const val END_EPSILON = 0.1
    }
}
