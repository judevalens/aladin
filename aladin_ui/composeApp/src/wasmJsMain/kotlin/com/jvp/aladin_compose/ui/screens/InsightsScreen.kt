package com.jvp.aladin_compose.ui.screens

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.jvp.aladin_compose.api.ApiClient
import com.jvp.aladin_compose.api.Insight
import com.jvp.aladin_compose.api.InsightStats
import com.jvp.aladin_compose.ui_lib.AladinDark
import com.jvp.aladin_compose.ui_lib.EmptyState
import com.jvp.aladin_compose.ui_lib.ErrorState
import com.jvp.aladin_compose.ui_lib.LoadingState
import kotlinx.coroutines.launch

private val INSIGHT_TYPES = listOf(
    null to "All",
    "bridge" to "Bridge",
    "convergence" to "Convergence",
    "trend" to "Trend",
    "contradiction" to "Contradiction",
)

@Composable
fun InsightsScreen() {
    val scope = rememberCoroutineScope()
    var insights by remember { mutableStateOf<List<Insight>>(emptyList()) }
    var total by remember { mutableStateOf(0) }
    var loading by remember { mutableStateOf(true) }
    var error by remember { mutableStateOf<String?>(null) }
    var statusFilter by remember { mutableStateOf("pending") }
    var typeFilter by remember { mutableStateOf<String?>(null) }
    var stats by remember { mutableStateOf(InsightStats()) }

    suspend fun load() {
        loading = true
        error = null
        try {
            val resp = ApiClient.getInsights(status = statusFilter, type = typeFilter)
            insights = resp.items
            total = resp.total
        } catch (e: Exception) {
            error = e.message ?: "Failed to load insights"
        } finally {
            loading = false
        }
    }

    LaunchedEffect(statusFilter, typeFilter) { load() }

    LaunchedEffect(Unit) {
        stats = ApiClient.getInsightStats()
    }

    Column(modifier = Modifier.fillMaxSize()) {
        // Header
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 24.dp, vertical = 16.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceBetween
        ) {
            Column {
                Row(
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text(
                        "Insights",
                        style = MaterialTheme.typography.headlineSmall,
                        fontWeight = FontWeight.SemiBold,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                    val pendingCount = stats.byStatus["pending"] ?: 0
                    if (pendingCount > 0) {
                        Box(
                            modifier = Modifier
                                .clip(RoundedCornerShape(10.dp))
                                .background(MaterialTheme.colorScheme.surfaceVariant)
                                .padding(horizontal = 8.dp, vertical = 3.dp)
                        ) {
                            Text(
                                "$pendingCount new",
                                style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                                fontWeight = FontWeight.SemiBold
                            )
                        }
                    }
                }
                if (total > 0) {
                    Text("$total insights", style = MaterialTheme.typography.bodySmall, color = AladinDark.TextMuted)
                }
            }
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                StatusChip("pending", "Pending", statusFilter) { statusFilter = it }
                StatusChip("accepted", "Accepted", statusFilter) { statusFilter = it }
            }
        }

        // Type filter
        LazyRow(
            contentPadding = PaddingValues(horizontal = 24.dp, vertical = 4.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp)
        ) {
            items(INSIGHT_TYPES) { (value, label) ->
                InsightTypeChip(value, label, typeFilter) { typeFilter = it }
            }
        }

        Spacer(Modifier.height(8.dp))

        when {
            loading -> LoadingState()
            error != null -> ErrorState(error!!) { scope.launch { load() } }
            insights.isEmpty() -> EmptyState("No $statusFilter insights")
            else -> {
                LazyColumn(
                    contentPadding = PaddingValues(horizontal = 24.dp, vertical = 8.dp),
                    verticalArrangement = Arrangement.spacedBy(10.dp),
                    modifier = Modifier.fillMaxSize()
                ) {
                    items(insights, key = { it.id }) { insight ->
                        InsightCard(
                            insight = insight,
                            onAccept = if (insight.userStatus == "pending") ({
                                scope.launch {
                                    try {
                                        ApiClient.acceptInsight(insight.id)
                                        insights = insights.filter { it.id != insight.id }
                                        total--
                                    } catch (_: Exception) {}
                                }
                            }) else null,
                            onDismiss = if (insight.userStatus == "pending") ({
                                scope.launch {
                                    try {
                                        ApiClient.dismissInsight(insight.id)
                                        insights = insights.filter { it.id != insight.id }
                                        total--
                                    } catch (_: Exception) {}
                                }
                            }) else null
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun InsightCard(
    insight: Insight,
    onAccept: (() -> Unit)?,
    onDismiss: (() -> Unit)?
) {
    val statusColor = when (insight.type) {
        "contradiction" -> MaterialTheme.colorScheme.error
        "trend"         -> MaterialTheme.colorScheme.scrim
        else            -> MaterialTheme.colorScheme.onSurfaceVariant
    }

    Card(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(10.dp),
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant)
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.Top
            ) {
                Column(modifier = Modifier.weight(1f)) {
                    Row(
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Box(
                            modifier = Modifier
                                .clip(RoundedCornerShape(4.dp))
                                .background(MaterialTheme.colorScheme.surface)
                                .padding(horizontal = 8.dp, vertical = 3.dp)
                        ) {
                            Text(
                                insight.type.replace("_", " "),
                                style = MaterialTheme.typography.labelSmall,
                                color = statusColor,
                                fontWeight = FontWeight.SemiBold
                            )
                        }
                        val tag = insight.entity ?: insight.topic
                        if (tag != null) {
                            Text(
                                tag,
                                style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                                fontWeight = FontWeight.Medium
                            )
                        }
                    }
                    Spacer(Modifier.height(6.dp))
                    Text(
                        insight.title,
                        style = MaterialTheme.typography.bodyLarge,
                        fontWeight = FontWeight.Medium,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                }
                ConfidenceBadge(insight.confidence)
            }

            Spacer(Modifier.height(8.dp))
            Text(
                insight.body,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                maxLines = 4,
                overflow = TextOverflow.Ellipsis
            )

            if (insight.artifactIds.isNotEmpty()) {
                Spacer(Modifier.height(6.dp))
                Text(
                    "${insight.artifactIds.size} sources",
                    style = MaterialTheme.typography.labelSmall,
                    color = AladinDark.TextMuted
                )
            }

            if (onAccept != null || onDismiss != null) {
                Spacer(Modifier.height(12.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    if (onAccept != null) {
                        Button(
                            onClick = onAccept,
                            contentPadding = PaddingValues(horizontal = 16.dp, vertical = 6.dp),
                            colors = ButtonDefaults.buttonColors(containerColor = MaterialTheme.colorScheme.onSurface)
                        ) {
                            Text("Accept", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.surface)
                        }
                    }
                    if (onDismiss != null) {
                        TextButton(
                            onClick = onDismiss,
                            contentPadding = PaddingValues(horizontal = 12.dp, vertical = 6.dp)
                        ) {
                            Text("Dismiss", color = AladinDark.TextMuted, style = MaterialTheme.typography.labelMedium)
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun ConfidenceBadge(confidence: Double) {
    val pct = (confidence * 100).toInt().coerceIn(0, 100)
    Column(horizontalAlignment = Alignment.CenterHorizontally) {
        Text(
            "$pct%",
            style = MaterialTheme.typography.labelMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            fontWeight = FontWeight.Bold
        )
        Text("conf.", style = MaterialTheme.typography.labelSmall, color = AladinDark.TextMuted)
    }
}

@Composable
private fun InsightTypeChip(value: String?, label: String, current: String?, onSelect: (String?) -> Unit) {
    val selected = value == current
    Box(
        modifier = Modifier
            .clip(RoundedCornerShape(6.dp))
            .clickable { onSelect(value) }
            .background(if (selected) MaterialTheme.colorScheme.primaryContainer else MaterialTheme.colorScheme.surfaceVariant)
            .padding(horizontal = 12.dp, vertical = 6.dp)
    ) {
        Text(
            label,
            style = MaterialTheme.typography.labelMedium,
            color = if (selected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant
        )
    }
}

@Composable
private fun StatusChip(value: String, label: String, current: String, onSelect: (String) -> Unit) {
    val selected = value == current
    Surface(
        onClick = { onSelect(value) },
        shape = RoundedCornerShape(6.dp),
        color = if (selected) MaterialTheme.colorScheme.primaryContainer else MaterialTheme.colorScheme.surfaceVariant
    ) {
        Text(
            label,
            modifier = Modifier.padding(horizontal = 12.dp, vertical = 6.dp),
            style = MaterialTheme.typography.labelMedium,
            color = if (selected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant
        )
    }
}
