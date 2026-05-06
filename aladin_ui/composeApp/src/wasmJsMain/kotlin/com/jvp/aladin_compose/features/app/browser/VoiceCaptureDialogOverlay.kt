package com.jvp.aladin_compose.features.app.browser

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.gestures.detectDragGestures
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.sharp.Pause
import androidx.compose.material.icons.sharp.PlayArrow
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.onGloballyPositioned
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.unit.IntSize
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.jvp.aladin_compose.ui_lib.AladinColor
import com.jvp.aladin_compose.ui_lib.AladinInteractionDefaults
import com.jvp.aladin_compose.ui_lib.aladinClickable
import kotlin.math.max

private val VoiceSheetWidth = 430.dp
private val VoiceSheetRadius = 6.dp
private val VoiceControlRadius = 4.dp
private val VoiceWaveHeight = 76.dp

@Composable
fun VoiceCaptureDialogOverlay(
    state: BrowserVoiceCaptureState,
    onBeginRecording: () -> Unit,
    onStopRecording: () -> Unit,
    onTogglePlayback: () -> Unit,
    onSeekPlayback: (Long) -> Unit,
    onTitleChanged: (String) -> Unit,
    onDescriptionChanged: (String) -> Unit,
    onSave: () -> Unit,
    onCancel: () -> Unit,
) {
    Box(
        modifier =
            Modifier.fillMaxSize()
                .background(AladinColor.Ink.copy(alpha = 0.16f))
                .aladinClickable(
                    shape = RoundedCornerShape(0.dp),
                    colors =
                        AladinInteractionDefaults.colors(
                            rest = Color.Transparent,
                            hovered = Color.Transparent,
                            pressed = Color.Transparent,
                        ),
                    onClick = {
                        if (state.phase != BrowserVoiceCapturePhase.Recording &&
                            state.phase != BrowserVoiceCapturePhase.Starting &&
                            state.phase != BrowserVoiceCapturePhase.Saving
                        ) {
                            onCancel()
                        }
                    },
                ),
        contentAlignment = Alignment.Center,
    ) {
        Column(
            modifier =
                Modifier.widthIn(max = VoiceSheetWidth)
                    .fillMaxWidth()
                    .aladinClickable(
                        shape = RoundedCornerShape(VoiceSheetRadius),
                        colors =
                            AladinInteractionDefaults.colors(
                                rest = Color.Transparent,
                                hovered = Color.Transparent,
                                pressed = Color.Transparent,
                            ),
                        onClick = {},
                    )
                    .border(1.dp, AladinColor.Border, RoundedCornerShape(VoiceSheetRadius))
                    .background(AladinColor.Panel, RoundedCornerShape(VoiceSheetRadius))
                    .padding(14.dp),
            verticalArrangement = Arrangement.spacedBy(13.dp),
        ) {
            VoiceCaptureHeader(state)
            HorizontalDivider(color = AladinColor.Divider)
            VoiceWaveform(
                levels = state.levels,
                active = state.phase == BrowserVoiceCapturePhase.Recording,
                playbackProgress =
                    if (state.phase == BrowserVoiceCapturePhase.Review) {
                        state.playbackPositionMs.toFloat() /
                            state.playbackDurationMs.coerceAtLeast(1).toFloat()
                    } else {
                        null
                    },
            )
            if (state.phase == BrowserVoiceCapturePhase.Review && state.playbackUrl != null) {
                VoicePlaybackControls(
                    state = state,
                    onTogglePlayback = onTogglePlayback,
                    onSeekPlayback = onSeekPlayback,
                )
            }
            VoiceCaptureControls(
                state = state,
                onBeginRecording = onBeginRecording,
                onStopRecording = onStopRecording,
                onSave = onSave,
                onCancel = onCancel,
            )
            if (state.phase == BrowserVoiceCapturePhase.Review ||
                state.phase == BrowserVoiceCapturePhase.Saving
            ) {
                HorizontalDivider(color = AladinColor.Divider)
                VoiceReviewFields(
                    title = state.title,
                    description = state.description,
                    enabled = state.phase != BrowserVoiceCapturePhase.Saving,
                    onTitleChanged = onTitleChanged,
                    onDescriptionChanged = onDescriptionChanged,
                )
            }
            state.errorMessage?.let {
                Text(
                    text = it,
                    color = MaterialTheme.colorScheme.error,
                    style = MaterialTheme.typography.bodySmall,
                )
            }
        }
    }
}

@Composable
private fun VoiceCaptureHeader(state: BrowserVoiceCaptureState) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.Top,
    ) {
        Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(
                text = "Voice note",
                color = AladinColor.Ink,
                style =
                    MaterialTheme.typography.titleLarge.copy(
                        lineHeight = 26.sp,
                        letterSpacing = (-0.2).sp,
                    ),
                fontWeight = FontWeight.Bold,
            )
            Text(
                text =
                    when (state.phase) {
                        BrowserVoiceCapturePhase.Ready -> "Capture a thought before it disappears"
                        BrowserVoiceCapturePhase.Starting -> "Requesting microphone"
                        BrowserVoiceCapturePhase.Recording -> "Recording"
                        BrowserVoiceCapturePhase.Review -> "Review and save"
                        BrowserVoiceCapturePhase.Saving -> "Saving voice note"
                        BrowserVoiceCapturePhase.Failed -> "Recording failed"
                    },
                color = AladinColor.InkMuted,
                style = MaterialTheme.typography.bodySmall,
            )
        }
        Text(
            text = formatElapsed(state.elapsedMs),
            color = if (state.phase == BrowserVoiceCapturePhase.Recording) AladinColor.Ink else AladinColor.InkMuted,
            style = MaterialTheme.typography.titleMedium,
            fontWeight = FontWeight.SemiBold,
        )
    }
}

@Composable
private fun VoiceWaveform(
    levels: List<Float>,
    active: Boolean,
    playbackProgress: Float?,
) {
    val hasLevels = levels.isNotEmpty()
    val bars = if (hasLevels) levels.takeLast(42) else List(42) { 0f }
    val progressIndex = ((playbackProgress ?: -1f).coerceIn(0f, 1f) * bars.size).toInt()
    Row(
        modifier =
            Modifier.fillMaxWidth()
                .height(VoiceWaveHeight)
                .background(AladinColor.CommandSurface, RoundedCornerShape(VoiceControlRadius))
                .padding(horizontal = 12.dp, vertical = 10.dp),
        horizontalArrangement = Arrangement.spacedBy(4.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        bars.forEachIndexed { index, level ->
            val normalized = level.coerceIn(0f, 1f)
            val height =
                if (hasLevels) {
                    (3 + (normalized * 52)).dp
                } else {
                    2.dp
                }
            Box(
                modifier =
                    Modifier.weight(1f)
                        .height(height)
                        .background(
                            when {
                                active -> AladinColor.Ink
                                playbackProgress != null && index < progressIndex -> AladinColor.Ink
                                else -> AladinColor.InkMuted.copy(alpha = 0.62f)
                            },
                            RoundedCornerShape(999.dp),
                        )
            )
        }
    }
}

@Composable
private fun VoicePlaybackControls(
    state: BrowserVoiceCaptureState,
    onTogglePlayback: () -> Unit,
    onSeekPlayback: (Long) -> Unit,
) {
    val duration = state.playbackDurationMs.coerceAtLeast(1L)
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        VoicePlaybackIconButton(
            playing = state.isPlaying,
            enabled = state.phase == BrowserVoiceCapturePhase.Review,
            onClick = onTogglePlayback,
        )
        Text(
            text = "${formatElapsed(state.playbackPositionMs)} / ${formatElapsed(state.playbackDurationMs)}",
            color = AladinColor.InkMuted,
            style = MaterialTheme.typography.labelSmall,
            fontWeight = FontWeight.Medium,
        )
        VoiceSeekBar(
            positionMs = state.playbackPositionMs,
            durationMs = duration,
            modifier = Modifier.weight(1f),
            onSeekPlayback = onSeekPlayback,
        )
    }
}

@Composable
private fun VoicePlaybackIconButton(
    playing: Boolean,
    enabled: Boolean,
    onClick: () -> Unit,
) {
    val shape = RoundedCornerShape(999.dp)
    Box(
        modifier =
            Modifier.width(32.dp)
                .height(32.dp)
                .background(if (enabled) AladinColor.Panel else AladinColor.ControlHover, shape)
                .then(
                    if (enabled) {
                        Modifier.aladinClickable(
                            shape = shape,
                            colors =
                                AladinInteractionDefaults.colors(
                                    rest = Color.Transparent,
                                    hovered = AladinColor.ControlHover,
                                    pressed = AladinColor.ControlPressed,
                                ),
                            onClick = onClick,
                        )
                    } else {
                        Modifier
                    }
                ),
        contentAlignment = Alignment.Center,
    ) {
        Icon(
            imageVector = if (playing) Icons.Sharp.Pause else Icons.Sharp.PlayArrow,
            contentDescription = if (playing) "Pause recording preview" else "Play recording preview",
            tint = if (enabled) AladinColor.Ink else AladinColor.InkDisabled,
            modifier = Modifier.width(18.dp).height(18.dp),
        )
    }
}

@Composable
private fun VoiceSeekBar(
    positionMs: Long,
    durationMs: Long,
    modifier: Modifier = Modifier,
    onSeekPlayback: (Long) -> Unit,
) {
    var size by remember { mutableStateOf(IntSize.Zero) }
    val progress = (positionMs.toFloat() / durationMs.coerceAtLeast(1).toFloat()).coerceIn(0f, 1f)
    val density = LocalDensity.current
    val handleOffset =
        with(density) {
            val trackWidth = size.width.toDp()
            ((trackWidth * progress) - 5.dp).coerceIn(0.dp, (trackWidth - 10.dp).coerceAtLeast(0.dp))
        }

    fun seekAt(x: Float) {
        val width = size.width.coerceAtLeast(1)
        val nextProgress = (x / width).coerceIn(0f, 1f)
        onSeekPlayback((durationMs * nextProgress).toLong())
    }

    Box(
        modifier =
            modifier.height(24.dp)
                .onGloballyPositioned { size = it.size }
                .pointerInput(durationMs, size) {
                    detectTapGestures { offset ->
                        seekAt(offset.x)
                    }
                }
                .pointerInput(durationMs, size) {
                    detectDragGestures { change, _ ->
                        seekAt(change.position.x)
                    }
                },
        contentAlignment = Alignment.CenterStart,
    ) {
        Box(
            modifier =
                Modifier.fillMaxWidth()
                    .height(2.dp)
                    .background(AladinColor.Divider, RoundedCornerShape(999.dp))
        )
        Box(
            modifier =
                Modifier.fillMaxWidth(progress)
                    .height(2.dp)
                    .background(AladinColor.Ink, RoundedCornerShape(999.dp))
        )
        Box(
            modifier =
                Modifier.offset(x = handleOffset)
                    .width(10.dp)
                    .height(10.dp)
                    .background(AladinColor.Ink, RoundedCornerShape(999.dp))
        )
    }
}

@Composable
private fun VoiceCaptureControls(
    state: BrowserVoiceCaptureState,
    onBeginRecording: () -> Unit,
    onStopRecording: () -> Unit,
    onSave: () -> Unit,
    onCancel: () -> Unit,
) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        VoiceSecondaryButton(
            label = if (state.phase == BrowserVoiceCapturePhase.Review) "Discard" else "Cancel",
            enabled = state.phase != BrowserVoiceCapturePhase.Saving,
            onClick = onCancel,
        )
        when (state.phase) {
            BrowserVoiceCapturePhase.Ready,
            BrowserVoiceCapturePhase.Failed ->
                VoicePrimaryButton("Record", enabled = true, onClick = onBeginRecording)
            BrowserVoiceCapturePhase.Starting ->
                VoicePrimaryButton("Starting", enabled = false, onClick = {})
            BrowserVoiceCapturePhase.Recording ->
                VoicePrimaryButton("Stop", enabled = true, onClick = onStopRecording)
            BrowserVoiceCapturePhase.Review ->
                VoicePrimaryButton("Save voice note", enabled = true, onClick = onSave)
            BrowserVoiceCapturePhase.Saving ->
                VoicePrimaryButton("Saving", enabled = false, onClick = {})
        }
    }
}

@Composable
private fun VoiceReviewFields(
    title: String,
    description: String,
    enabled: Boolean,
    onTitleChanged: (String) -> Unit,
    onDescriptionChanged: (String) -> Unit,
) {
    Column(verticalArrangement = Arrangement.spacedBy(11.dp)) {
        VoiceTextField(
            label = "TITLE",
            value = title,
            enabled = enabled,
            singleLine = true,
            minHeight = 38.dp,
            onValueChange = onTitleChanged,
        )
        VoiceTextField(
            label = "DESCRIPTION",
            value = description,
            enabled = enabled,
            singleLine = false,
            minHeight = 82.dp,
            onValueChange = onDescriptionChanged,
        )
    }
}

@Composable
private fun VoiceTextField(
    label: String,
    value: String,
    enabled: Boolean,
    singleLine: Boolean,
    minHeight: androidx.compose.ui.unit.Dp,
    onValueChange: (String) -> Unit,
) {
    var inputValue by remember(label) {
        mutableStateOf(
            TextFieldValue(
                text = value,
                selection = TextRange(value.length),
            )
        )
    }
    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
        Text(
            text = label,
            color = AladinColor.CodeText,
            style = MaterialTheme.typography.labelSmall.copy(letterSpacing = 0.35.sp),
            fontWeight = FontWeight.SemiBold,
        )
        BasicTextField(
            value = inputValue,
            onValueChange = { nextValue ->
                inputValue = nextValue
                onValueChange(nextValue.text)
            },
            enabled = enabled,
            singleLine = singleLine,
            textStyle =
                TextStyle(
                    color = if (enabled) AladinColor.Ink else AladinColor.InkMuted,
                    fontSize = 14.sp,
                    lineHeight = 21.sp,
                    fontWeight = FontWeight.Medium,
                ),
            modifier =
                Modifier.fillMaxWidth()
                    .height(minHeight)
                    .background(AladinColor.CommandSurface, RoundedCornerShape(VoiceControlRadius))
                    .padding(horizontal = 9.dp, vertical = 8.dp),
        )
    }
}

@Composable
private fun VoicePrimaryButton(label: String, enabled: Boolean, onClick: () -> Unit) {
    VoiceButton(label = label, enabled = enabled, primary = true, onClick = onClick)
}

@Composable
private fun VoiceSecondaryButton(label: String, enabled: Boolean, onClick: () -> Unit) {
    VoiceButton(label = label, enabled = enabled, primary = false, onClick = onClick)
}

@Composable
private fun VoiceButton(
    label: String,
    enabled: Boolean,
    primary: Boolean,
    onClick: () -> Unit,
) {
    val shape = RoundedCornerShape(VoiceControlRadius)
    Box(
        modifier =
            Modifier.height(32.dp)
                .background(
                    when {
                        !enabled -> AladinColor.ControlHover
                        primary -> AladinColor.InkSurface
                        else -> AladinColor.Panel
                    },
                    shape,
                )
                .then(
                    if (enabled) {
                        Modifier.aladinClickable(
                            shape = shape,
                            colors =
                                AladinInteractionDefaults.colors(
                                    rest = Color.Transparent,
                                    hovered =
                                        if (primary) AladinColor.InkSurfaceHover
                                        else AladinColor.ControlHover,
                                    pressed =
                                        if (primary) AladinColor.InkSurfaceHover
                                        else AladinColor.ControlPressed,
                                ),
                            onClick = onClick,
                        )
                    } else {
                        Modifier
                    }
                )
                .padding(horizontal = 13.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            text = label,
            color =
                when {
                    !enabled -> AladinColor.InkDisabled
                    primary -> AladinColor.OnInkSurface
                    else -> AladinColor.InkSecondary
                },
            style = MaterialTheme.typography.labelMedium,
            fontWeight = FontWeight.Medium,
        )
    }
}

private fun formatElapsed(elapsedMs: Long): String {
    val seconds = max(0, (elapsedMs / 1000).toInt())
    val minutes = seconds / 60
    val remainder = seconds % 60
    return "$minutes:${remainder.toString().padStart(2, '0')}"
}
