package com.jvp.aladin_compose.features.app.artifactpane.voice

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.ExperimentalComposeUiApi
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.WebElementView
import com.jvp.aladin_compose.ui_lib.AladinColor
import kotlinx.browser.document
import org.w3c.dom.HTMLElement

private val VoiceCardWidth = 720.dp
private val VoiceControlRadius = 4.dp

@Composable
fun VoiceUi(
    state: VoiceArtifactState,
    modifier: Modifier = Modifier,
) {
    val content = state.content
    Box(
        modifier = modifier.fillMaxSize(),
        contentAlignment = Alignment.TopCenter,
    ) {
        Column(
            modifier =
                Modifier.widthIn(max = VoiceCardWidth)
                    .fillMaxWidth()
                    .padding(horizontal = 20.dp),
            verticalArrangement = Arrangement.spacedBy(18.dp),
        ) {
            VoiceHeader(state = state, content = content)

            if (content == null) {
                VoiceSection(title = "Recording") {
                    MutedText("Preparing voice note...")
                }
                return@Column
            }

            VoiceSection(title = "Playback", eyebrow = "Audio is the source of truth") {
                AudioPlayer(resourceUrl = content.resourceUrl)
            }
            VoiceSection(title = "Transcript", eyebrow = "Searchable derived text") {
                Text(
                    text = content.transcript,
                    color = AladinColor.Ink,
                    style = MaterialTheme.typography.bodyLarge.copy(lineHeight = 25.sp),
                )
            }
            VoiceSection(title = "User Notes", eyebrow = "Your interpretation") {
                Text(
                    text = content.userNotes,
                    color = AladinColor.InkSecondary,
                    style = MaterialTheme.typography.bodyMedium.copy(lineHeight = 23.sp),
                )
            }
            Text(
                text = content.statusLabel,
                color = AladinColor.InkMuted,
                style = MaterialTheme.typography.labelSmall,
            )
        }
    }
}

@Composable
private fun VoiceHeader(
    state: VoiceArtifactState,
    content: com.jvp.aladin_compose.repo.VoiceArtifactContent?,
) {
    Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.Top,
        ) {
            Column(
                modifier = Modifier.weight(1f),
                verticalArrangement = Arrangement.spacedBy(7.dp),
            ) {
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    VoicePill("VOICE")
                    if (content != null) VoicePill(content.durationLabel)
                    if (content != null) VoicePill(content.statusLabel)
                }
                Text(
                    text = state.artifact.title,
                    color = AladinColor.Ink,
                    style =
                        MaterialTheme.typography.headlineMedium.copy(
                            lineHeight = 34.sp,
                            letterSpacing = (-0.35).sp,
                        ),
                    fontWeight = FontWeight.Bold,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                )
                Text(
                    text = content?.recordedAtLabel ?: "Loading recording",
                    color = AladinColor.InkMuted,
                    style = MaterialTheme.typography.bodySmall,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
        }
        HorizontalDivider(color = AladinColor.Divider)
    }
}

@Composable
private fun VoiceSection(
    title: String,
    eyebrow: String? = null,
    content: @Composable () -> Unit,
) {
    Column(
        modifier =
            Modifier.fillMaxWidth()
                .padding(vertical = 2.dp),
        verticalArrangement = Arrangement.spacedBy(9.dp),
    ) {
        Column(verticalArrangement = Arrangement.spacedBy(3.dp)) {
            Text(
                text = title.uppercase(),
                color = AladinColor.CodeText,
                style = MaterialTheme.typography.labelSmall.copy(letterSpacing = 0.35.sp),
                fontWeight = FontWeight.SemiBold,
            )
            if (eyebrow != null) {
                Text(
                    text = eyebrow,
                    color = AladinColor.InkMuted,
                    style = MaterialTheme.typography.labelSmall.copy(letterSpacing = 0.2.sp),
                )
            }
        }
        HorizontalDivider(color = AladinColor.Divider)
        content()
    }
}

@Composable
private fun VoicePill(label: String) {
    Box(
        modifier =
            Modifier.background(AladinColor.CommandSurface, RoundedCornerShape(VoiceControlRadius))
                .padding(horizontal = 7.dp, vertical = 3.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            text = label.uppercase(),
            color = AladinColor.CodeText,
            style = MaterialTheme.typography.labelSmall.copy(letterSpacing = 0.35.sp),
            fontWeight = FontWeight.SemiBold,
        )
    }
}

@OptIn(ExperimentalComposeUiApi::class)
@Composable
private fun AudioPlayer(resourceUrl: String?) {
    if (resourceUrl == null) {
        MutedText("Audio resource unavailable")
        return
    }
    Box(
        modifier =
            Modifier.fillMaxWidth()
                .height(54.dp)
                .background(AladinColor.CommandSurface, RoundedCornerShape(VoiceControlRadius))
                .padding(horizontal = 10.dp, vertical = 9.dp),
        contentAlignment = Alignment.Center,
    ) {
        WebElementView(
            factory = {
                (document.createElement("audio") as HTMLElement).apply {
                    setAttribute("controls", "true")
                    setAttribute("src", resourceUrl)
                    setAttribute("style", "width: 100%; height: 36px; display: block;")
                }
            },
            modifier = Modifier.fillMaxSize(),
        )
    }
}

@Composable
private fun MutedText(text: String) {
    Text(
        text = text,
        color = AladinColor.InkMuted,
        style = MaterialTheme.typography.bodySmall.copy(lineHeight = 20.sp),
    )
}
