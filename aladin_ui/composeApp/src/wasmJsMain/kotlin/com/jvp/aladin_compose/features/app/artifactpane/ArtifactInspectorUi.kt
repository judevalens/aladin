package com.jvp.aladin_compose.features.app.artifactpane

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.jvp.aladin_compose.repo.ArtifactInsight
import com.jvp.aladin_compose.ui_lib.AladinColor

val ArtifactInspectorWidth = 320.dp

@Composable
fun ArtifactInspector(
    insight: ArtifactInsight?,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier =
            modifier.width(ArtifactInspectorWidth)
                .fillMaxHeight()
                .border(1.dp, AladinColor.Divider, RoundedCornerShape(6.dp))
                .background(AladinColor.Panel, RoundedCornerShape(6.dp))
                .padding(12.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Column(verticalArrangement = Arrangement.spacedBy(5.dp)) {
            Text(
                text = "CONTEXT",
                color = AladinColor.CodeText,
                style = MaterialTheme.typography.labelSmall.copy(letterSpacing = 0.45.sp),
                fontWeight = FontWeight.SemiBold,
            )
            Text(
                text = "System view of this artifact",
                color = AladinColor.InkMuted,
                style = MaterialTheme.typography.labelSmall.copy(letterSpacing = 0.15.sp),
            )
        }
        InspectorSection("Summary") {
            MutedOrBody(insight?.summary, fallback = "Summary unavailable")
        }
        InspectorSection("Key Points") {
            BulletList(insight?.keyPoints.orEmpty(), emptyText = "No key points yet")
        }
        InspectorSection("Related") {
            val related = insight?.relatedItems.orEmpty()
            if (related.isEmpty()) {
                MutedText("No related items yet")
            } else {
                Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    related.forEach { item ->
                        InspectorTokenRow(item.title)
                    }
                }
            }
        }
        InspectorSection("Entities") {
            BulletList(insight?.entities.orEmpty(), emptyText = "No entities detected")
        }
        InspectorSection("Metadata") {
            val metadata = insight?.metadata.orEmpty()
            if (metadata.isEmpty()) {
                MutedText("No metadata available")
            } else {
                Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    metadata.forEach { item ->
                        MetadataRow(label = item.label, value = item.value)
                    }
                }
            }
        }
    }
}

@Composable
private fun InspectorSection(
    title: String,
    content: @Composable () -> Unit,
) {
    Column(
        modifier =
            Modifier.fillMaxWidth()
                .background(AladinColor.Canvas, RoundedCornerShape(4.dp))
                .padding(vertical = 4.dp),
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        Text(
            text = title.uppercase(),
            color = AladinColor.CodeText,
            style = MaterialTheme.typography.labelSmall.copy(letterSpacing = 0.35.sp),
            fontWeight = FontWeight.SemiBold,
        )
        HorizontalDivider(color = AladinColor.Divider)
        content()
    }
}

@Composable
private fun BulletList(items: List<String>, emptyText: String) {
    if (items.isEmpty()) {
        MutedText(emptyText)
    } else {
        Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
            items.forEach { item ->
                InspectorTokenRow(item)
            }
        }
    }
}

@Composable
private fun InspectorTokenRow(text: String) {
    Box(
        modifier =
            Modifier.fillMaxWidth()
                .padding(vertical = 2.dp)
    ) {
        Text(
            text = text,
            color = AladinColor.InkSecondary,
            style = MaterialTheme.typography.bodySmall.copy(lineHeight = 19.sp),
            maxLines = 2,
            overflow = TextOverflow.Ellipsis,
        )
    }
}

@Composable
private fun MetadataRow(label: String, value: String) {
    Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
        Text(
            text = label.uppercase(),
            color = AladinColor.CodeText,
            style = MaterialTheme.typography.labelSmall.copy(letterSpacing = 0.25.sp),
            fontWeight = FontWeight.SemiBold,
        )
        Text(
            text = value,
            color = AladinColor.InkSecondary,
            style = MaterialTheme.typography.bodySmall.copy(lineHeight = 18.sp),
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
    }
}

@Composable
private fun MutedOrBody(value: String?, fallback: String) {
    if (value.isNullOrBlank()) {
        MutedText(fallback)
    } else {
        Text(
            text = value,
            color = AladinColor.InkSecondary,
            style = MaterialTheme.typography.bodySmall.copy(lineHeight = 19.sp),
        )
    }
}

@Composable
private fun MutedText(text: String) {
    Text(
        text = text,
        color = AladinColor.InkMuted,
        style = MaterialTheme.typography.bodySmall.copy(lineHeight = 19.sp),
    )
}
