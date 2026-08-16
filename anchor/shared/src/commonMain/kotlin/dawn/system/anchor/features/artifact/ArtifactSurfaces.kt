package dawn.system.anchor.features.artifact

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.onSizeChanged
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import dawn.system.anchor.domain.WorkspaceNode
import dawn.system.anchor.services.data.ArtifactResourceStore
import dawn.system.anchor.services.data.CachedResource
import dawn.system.anchor.services.design.AnchorShape
import dawn.system.anchor.services.design.AnchorTheme
import dawn.system.anchor.services.design.ExternalLinkIcon
import dawn.system.anchor.services.design.MetaStyle
import dawn.system.anchor.services.design.SectionLabelStyle
import dawn.system.anchor.services.platform.PlaybackPositions
import dawn.system.anchor.services.platform.fileSizeBytes
import dawn.system.anchor.services.platform.formatRate
import dawn.system.anchor.services.platform.formatTimecode
import dawn.system.anchor.services.platform.rememberAudioController
import dawn.system.anchor.services.platform.openExternalUrl

/**
 * A `link` artifact: what it points at, a way to get there, and what was captured
 * (VIEWERS.md Â§3).
 *
 * Two sections divided by one hairline â the desktop `LinkArtifactUI` shape, scaled up. The
 * host gets an eyebrow of its own because it is the part you actually recognise when
 * deciding whether to follow a link.
 */
@Composable
fun LinkArtifactView(node: WorkspaceNode, modifier: Modifier = Modifier) {
    val c = AnchorTheme.colors
    var refused by remember(node.id) { mutableStateOf(false) }
    val url = node.sourceUrl

    Box(modifier.fillMaxSize().verticalScroll(rememberScrollState())) {
        Column(
            Modifier
                .widthIn(max = MEASURE)
                .align(Alignment.TopCenter)
                .padding(horizontal = 32.dp),
        ) {
            Spacer(Modifier.height(6.dp))
            SectionLabel(listOfNotNull("source", url?.let(::hostOf)).joinToString(" · "))
            Spacer(Modifier.height(10.dp))
            Text(
                node.title.ifBlank { "Untitled link" },
                style = MaterialTheme.typography.headlineMedium,
                color = c.ink,
            )

            if (url == null) {
                Spacer(Modifier.height(14.dp))
                Text(
                    "This link has no target — the artifact carries no source URL.",
                    style = MetaStyle,
                    color = c.ink4,
                )
            } else {
                Spacer(Modifier.height(20.dp))
                Row(
                    Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(14.dp),
                ) {
                    Row(
                        Modifier
                            .height(48.dp)
                            .clip(AnchorShape.field)
                            .background(c.amber)
                            .clickable { refused = !openExternalUrl(url) }
                            .padding(horizontal = 18.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(9.dp),
                    ) {
                        ExternalLinkIcon(c.onAmber, size = 17.dp)
                        Text(
                            "Open link",
                            style = MaterialTheme.typography.titleMedium,
                            color = c.onAmber,
                            maxLines = 1,
                        )
                    }
                    Text(
                        url,
                        style = MetaStyle,
                        color = c.amber,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.weight(1f),
                    )
                }
                if (refused) {
                    Spacer(Modifier.height(10.dp))
                    Text("The system wouldn't open that URL.", style = MetaStyle, color = c.against)
                }
            }

            node.summary?.takeIf { it.isNotBlank() }?.let {
                Spacer(Modifier.height(20.dp))
                Text(it, style = MaterialTheme.typography.bodyLarge, color = c.ink2)
            }

            Spacer(Modifier.height(28.dp))
            Box(Modifier.fillMaxWidth().height(1.dp).background(c.line))
            Spacer(Modifier.height(20.dp))

            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                Box(Modifier.weight(1f)) { SectionLabel("excerpt") }
                // The honesty note is the point of this section: a capture is a snapshot,
                // and saying so is what stops it being read as the live page.
                Text("the live page may have changed", style = MetaStyle, color = c.ink4)
            }
            Spacer(Modifier.height(10.dp))
            // Desktop's placeholder copy, kept. The captured body is not on the companion's
            // wire yet — a light node carries a summary, not the captured text.
            Text(
                "Captured text will appear here when it is available.",
                style = MetaStyle,
                color = c.ink4,
            )
            Spacer(Modifier.height(36.dp))
        }
    }
}

/**
 * A `voice` artifact: the recording, its player, and the transcript when there is one
 * (VIEWERS.md §2).
 *
 * A 720dp measure, centred — the same reading width the link viewer uses, so the two
 * artifact surfaces feel like one app rather than two layouts.
 */
@Composable
fun VoiceArtifactView(
    node: WorkspaceNode,
    resources: ArtifactResourceStore,
    modifier: Modifier = Modifier,
) {
    val c = AnchorTheme.colors
    // Same rule as the reader: a cached recording resolves on the first frame rather than
    // flashing a loading line at anyone who switches back to it.
    var state by remember(node.id) {
        mutableStateOf<AudioState>(
            resources.cached(node.id)?.let(AudioState::Ready) ?: AudioState.Loading,
        )
    }

    LaunchedEffect(node.id) {
        if (state is AudioState.Ready) return@LaunchedEffect
        state = try {
            AudioState.Ready(resources.resource(node.id))
        } catch (failure: Throwable) {
            AudioState.Failed(failure.message ?: "could not fetch the recording")
        }
    }

    Box(modifier.fillMaxSize().verticalScroll(rememberScrollState())) {
        Column(
            Modifier
                .widthIn(max = MEASURE)
                .align(Alignment.TopCenter)
                .padding(horizontal = 32.dp),
        ) {
            Spacer(Modifier.height(6.dp))
            SectionLabel(
                listOfNotNull(
                    "voice note",
                    (state as? AudioState.Ready)?.resource?.let(::audioFileMeta),
                ).joinToString(" · "),
            )
            Spacer(Modifier.height(10.dp))
            Text(
                node.title.ifBlank { "Untitled recording" },
                style = MaterialTheme.typography.headlineMedium,
                color = c.ink,
            )
            Spacer(Modifier.height(20.dp))

            when (val current = state) {
                AudioState.Loading -> Text("Loading audio…", style = MetaStyle, color = c.ink4)

                is AudioState.Failed -> Text(
                    "Couldn't load the audio. ${current.message}",
                    style = MetaStyle,
                    color = c.against,
                )

                is AudioState.Ready -> if (current.resource.isPlayableAudio) {
                    PlayerCard(artifactId = node.id, filePath = current.resource.path)
                } else {
                    UnplayableNotice(current.resource.contentType)
                }
            }

            node.summary?.takeIf { it.isNotBlank() }?.let {
                Spacer(Modifier.height(26.dp))
                SectionLabel("summary")
                Spacer(Modifier.height(8.dp))
                Text(it, style = MaterialTheme.typography.bodyLarge, color = c.ink2)
            }

            Spacer(Modifier.height(26.dp))
            SectionLabel("transcript")
            Spacer(Modifier.height(8.dp))
            // Desktop's placeholder copy, kept verbatim — an empty section reads as broken,
            // a placeholder reads as "not yet".
            Text("Not synced to the companion yet.", style = MetaStyle, color = c.ink4)

            Spacer(Modifier.height(24.dp))
            Text(
                "The original audio remains the source record for this note; the " +
                    "transcript is the machine layer.",
                style = MetaStyle,
                color = c.ink4,
            )
            Spacer(Modifier.height(36.dp))
        }
    }
}

/** `m4a · 2.1 MB` — what the file actually is, from what the server said and what is on disk. */
private fun audioFileMeta(resource: CachedResource): String? {
    val extension = resource.path.substringAfterLast('.', "").takeIf { it.length in 2..4 }
    val size = fileSizeBytes(resource.path).takeIf { it > 0 }?.let { bytes ->
        val mb = bytes / 1_048_576.0
        if (mb >= 0.1) "${(mb * 10).toLong() / 10.0} MB" else "${bytes / 1024} KB"
    }
    return listOfNotNull(extension, size).joinToString(" · ").takeIf { it.isNotBlank() }
}

/**
 * A visible transport: play/pause, elapsed and total time, and a scrubber.
 *
 * `AVPlayerViewController` was the first attempt and was wrong — it is a *video* surface, so
 * a voice note rendered as a black rectangle whose controls auto-hid until tapped. Nothing
 * about it said "player". This is the audio equivalent of desktop's `<audio controls>`, in
 * the app's own type and colour.
 */
@Composable
private fun PlayerCard(artifactId: String, filePath: String) {
    val c = AnchorTheme.colors
    val audio = rememberAudioController(filePath, startAt = PlaybackPositions.of(artifactId))
    val fraction = if (audio.duration > 0) (audio.position / audio.duration).toFloat() else 0f

    // Where you were, kept for when you come back — the surface itself may be unmounted by
    // the keep-alive cap long before you have finished listening.
    LaunchedEffect(audio.position) { PlaybackPositions.remember(artifactId, audio.position) }

    Column(
        Modifier
            .fillMaxWidth()
            .clip(AnchorShape.heroCard)
            .background(c.card)
            .padding(horizontal = 22.dp, vertical = 20.dp),
    ) {
        Row(
            Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(18.dp),
        ) {
            Box(
                Modifier
                    .size(64.dp)
                    .clip(AnchorShape.pill)
                    .background(c.amber)
                    // Deliberately not gated on readiness. The first version disabled this
                    // until the asset reported a duration, which deadlocked: the asset only
                    // loaded once playback started, and playback could not start.
                    .clickable { audio.toggle() },
                contentAlignment = Alignment.Center,
            ) {
                // Two glyphs drawn from primitives rather than icons: Feather's play is a
                // triangle outline, which reads as a button *outline* on a filled pill.
                if (audio.playing) PauseGlyph(c.onAmber, 18.dp) else PlayGlyph(c.onAmber, 20.dp)
            }

            Column(Modifier.weight(1f)) {
                Waveform(seed = artifactId, played = fraction, onSeek = audio::seek)
                Spacer(Modifier.height(9.dp))
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                    Text(formatTimecode(audio.position), style = MetaStyle, color = c.amber)
                    Text(
                        if (audio.ready) formatTimecode(audio.duration) else "--:--",
                        style = MetaStyle,
                        color = c.ink4,
                    )
                }
            }
        }

        Spacer(Modifier.height(16.dp))
        Row(
            Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Spacer(Modifier.weight(1f))
            TransportButton("−15") { audio.skip(-SKIP_SECONDS) }
            TransportButton("+15") { audio.skip(SKIP_SECONDS) }
            TransportButton(formatRate(audio.rate), active = audio.rate != 1f) { audio.cycleRate() }
        }

        audio.failure?.let {
            Spacer(Modifier.height(12.dp))
            Text("Playback failed — $it", style = MetaStyle, color = c.against)
        }
    }
}

/**
 * The scrubber, drawn as bars.
 *
 * The heights are **deterministic from the artifact id, not the audio** — the design asks
 * for exactly that, and it is the honest option: nothing on the wire carries an amplitude
 * envelope, and decoding one on open would cost a full pass over the file before the first
 * tap. So this is a scrubber with a texture, stable per recording; a real envelope would
 * have to come from the server alongside the transcript.
 */
@Composable
private fun Waveform(seed: String, played: Float, onSeek: (Float) -> Unit) {
    val c = AnchorTheme.colors
    val heights = remember(seed) { waveformHeights(seed) }
    var width by remember { mutableStateOf(1) }

    Row(
        Modifier
            .fillMaxWidth()
            .height(WAVEFORM_HEIGHT)
            .onSizeChanged { width = it.width.coerceAtLeast(1) }
            .pointerInput(Unit) { detectTapGestures { offset -> onSeek(offset.x / width) } },
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(3.dp),
    ) {
        val playedBars = (played.coerceIn(0f, 1f) * heights.size).toInt()
        heights.forEachIndexed { index, height ->
            Box(
                Modifier
                    .weight(1f)
                    .fillMaxHeight(height)
                    .clip(AnchorShape.pill)
                    .background(if (index <= playedBars) c.amber else c.ink4),
            )
        }
    }
}

/**
 * 84 bars from a hash of the id. Any stable scramble would do; what matters is that the
 * same recording draws the same shape every time it opens, so the waveform reads as a
 * property of the note rather than as noise redrawn on every recomposition.
 */
private fun waveformHeights(seed: String): List<Float> {
    var state = seed.fold(17) { acc, char -> acc * 31 + char.code }.toLong() or 1L
    return List(WAVEFORM_BARS) {
        state = (state * 6364136223846793005L + 1442695040888963407L)
        val unit = ((state ushr 33).toInt() and 0xFFFF) / 65535f
        // Never a zero-height bar: a gap in the middle of a scrubber looks like damage.
        0.22f + unit * 0.78f
    }
}

@Composable
private fun TransportButton(label: String, active: Boolean = false, onClick: () -> Unit) {
    val c = AnchorTheme.colors
    Box(
        Modifier
            .heightIn(min = 44.dp)
            .widthIn(min = 52.dp)
            .clip(AnchorShape.field)
            .background(if (active) c.amberSoft else c.field)
            .clickable(onClick = onClick)
            .padding(horizontal = 12.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(label, style = MetaStyle, color = if (active) c.amber else c.ink3, maxLines = 1)
    }
}

/**
 * A recording this device cannot decode. Says which format, because the answer ("re-record
 * it from the desktop app rather than a Chromium browser") depends on knowing that.
 */
@Composable
private fun UnplayableNotice(contentType: String?) {
    val c = AnchorTheme.colors

    Column(
        Modifier
            .fillMaxWidth()
            .clip(AnchorShape.card)
            .background(c.card)
            .padding(horizontal = 18.dp, vertical = 16.dp),
    ) {
        Text(
            "This device can't play ${contentType ?: "this format"}.",
            style = MaterialTheme.typography.titleMedium,
            color = c.ink2,
        )
        Spacer(Modifier.height(6.dp))
        Text(
            "Apple platforms ship no WebM decoder, so a note captured in a Chromium " +
                "browser can only be played on the desktop.",
            style = MetaStyle,
            color = c.ink4,
        )
    }
}

@Composable
private fun PlayGlyph(tint: Color, glyphSize: Dp = 16.dp) {
    Canvas(Modifier.size(glyphSize)) {
        val path = Path().apply {
            moveTo(size.width * 0.22f, 0f)
            lineTo(size.width, size.height / 2f)
            lineTo(size.width * 0.22f, size.height)
            close()
        }
        drawPath(path, tint)
    }
}

@Composable
private fun PauseGlyph(tint: Color, glyphSize: Dp = 14.dp) {
    Canvas(Modifier.size(glyphSize)) {
        val bar = size.width * 0.34f
        drawRect(tint, topLeft = Offset(0f, 0f), size = Size(bar, size.height))
        drawRect(tint, topLeft = Offset(size.width - bar, 0f), size = Size(bar, size.height))
    }
}

/** The reading measure both artifact surfaces use. */
private val MEASURE = 720.dp
private val WAVEFORM_HEIGHT = 54.dp
private const val WAVEFORM_BARS = 84
private const val SKIP_SECONDS = 15.0

private sealed interface AudioState {
    data object Loading : AudioState
    data class Ready(val resource: CachedResource) : AudioState
    data class Failed(val message: String) : AudioState
}

/** `https://arxiv.org/abs/123` → `arxiv.org`. Null when there is nothing recognisable. */
internal fun hostOf(url: String): String? = url
    .substringAfter("://", missingDelimiterValue = url)
    .substringBefore('/')
    .substringBefore(':')
    .takeIf { it.isNotBlank() && it.contains('.') }

@Composable
private fun SectionLabel(label: String) {
    Text(label.uppercase(), style = SectionLabelStyle, color = AnchorTheme.colors.ink4)
}
