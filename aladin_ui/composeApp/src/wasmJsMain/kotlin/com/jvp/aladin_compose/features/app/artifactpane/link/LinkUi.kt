package com.jvp.aladin_compose.features.app.artifactpane.link

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.jvp.aladin_compose.openUrl
import com.jvp.aladin_compose.ui_lib.AladinColor
import com.jvp.aladin_compose.ui_lib.AladinInteractionDefaults
import com.jvp.aladin_compose.ui_lib.aladinClickable

private val LinkCardWidth = 720.dp
private val LinkCardRadius = 6.dp
private val LinkControlRadius = 4.dp

@Composable
fun LinkUi(
    state: LinkArtifactState,
    modifier: Modifier = Modifier,
) {
    val content = state.content
    Box(
        modifier = modifier.fillMaxSize(),
        contentAlignment = Alignment.TopCenter,
    ) {
        Column(
            modifier =
                Modifier.widthIn(max = LinkCardWidth)
                    .fillMaxWidth()
                    .border(1.dp, AladinColor.Border, RoundedCornerShape(LinkCardRadius))
                    .background(AladinColor.Panel, RoundedCornerShape(LinkCardRadius))
                    .padding(20.dp),
            verticalArrangement = Arrangement.spacedBy(18.dp),
        ) {
            LinkHeader(state = state, content = content)

            if (content == null) {
                LinkSection(title = "Source", emphasis = true) {
                    MutedText("Preparing link content...")
                }
                return@Column
            }

            LinkSection(title = "AI Summary", eyebrow = "Generated context", emphasis = true) {
                Text(
                    text = content.aiSummary,
                    color = AladinColor.Ink,
                    style = MaterialTheme.typography.bodyLarge.copy(lineHeight = 25.sp),
                )
            }
            LinkSection(title = "User Notes", eyebrow = "Your read on this source") {
                Text(
                    text = content.userNotes,
                    color = AladinColor.InkSecondary,
                    style = MaterialTheme.typography.bodyMedium.copy(lineHeight = 23.sp),
                )
            }
            LinkSection(title = "Raw Capture", eyebrow = "Stored source material") {
                Text(
                    text = content.rawExcerpt,
                    color = AladinColor.InkSecondary,
                    style = MaterialTheme.typography.bodySmall.copy(lineHeight = 20.sp),
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
private fun LinkHeader(
    state: LinkArtifactState,
    content: com.jvp.aladin_compose.repo.LinkArtifactContent?,
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
                    SourcePill("LINK")
                    if (content != null) SourcePill(content.statusLabel)
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
                    text = content?.displayDomain ?: "Loading source",
                    color = AladinColor.InkMuted,
                    style = MaterialTheme.typography.bodySmall,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
            if (content != null) {
                LinkCommandButton("Open original") { openUrl(content.sourceUrl) }
            }
        }
        HorizontalDivider(color = AladinColor.Divider)
    }
}

@Composable
private fun LinkSection(
    title: String,
    eyebrow: String? = null,
    emphasis: Boolean = false,
    content: @Composable () -> Unit,
) {
    Column(
        modifier =
            Modifier.fillMaxWidth()
                .background(if (emphasis) AladinColor.Canvas else AladinColor.Panel)
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
private fun SourcePill(label: String) {
    Box(
        modifier =
            Modifier.background(AladinColor.CommandSurface, RoundedCornerShape(LinkControlRadius))
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

@Composable
private fun LinkCommandButton(label: String, onClick: () -> Unit) {
    Box(
        modifier =
            Modifier.border(1.dp, AladinColor.InkSurface, RoundedCornerShape(LinkControlRadius))
                .background(AladinColor.InkSurface, RoundedCornerShape(LinkControlRadius))
                .aladinClickable(
                    shape = RoundedCornerShape(LinkControlRadius),
                    colors =
                        AladinInteractionDefaults.colors(
                            rest = AladinColor.InkSurface,
                            hovered = AladinColor.InkSurfaceHover,
                            pressed = AladinColor.InkSurfaceHover,
                        ),
                    onClick = onClick,
                )
                .padding(horizontal = 12.dp, vertical = 7.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            text = label,
            color = AladinColor.OnInkSurface,
            style = MaterialTheme.typography.labelMedium,
            fontWeight = FontWeight.Medium,
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
