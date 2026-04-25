package com.jvp.aladin_compose.ui.screens

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.jvp.aladin_compose.api.ApiClient
import com.jvp.aladin_compose.api.FeedItem
import com.jvp.aladin_compose.openUrl
import com.jvp.aladin_compose.ui_lib.AladinDark
import com.jvp.aladin_compose.ui_lib.EmptyState
import com.jvp.aladin_compose.ui_lib.ErrorState
import com.jvp.aladin_compose.ui_lib.LoadingState
import kotlinx.coroutines.launch

private val SOURCE_TYPE_FILTERS = listOf(
    null to "All",
    "reddit" to "Reddit",
    "bluesky" to "Bluesky",
    "twitter" to "X / Twitter",
)

@Composable
fun FeedScreen(savedOnly: Boolean = false) {
    val scope = rememberCoroutineScope()
    var items by remember { mutableStateOf<List<FeedItem>>(emptyList()) }
    var total by remember { mutableStateOf(0) }
    var loading by remember { mutableStateOf(true) }
    var refreshing by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    var topics by remember { mutableStateOf<List<String>>(emptyList()) }
    var selectedTopic by remember { mutableStateOf<String?>(null) }
    var selectedSourceType by remember { mutableStateOf<String?>(null) }
    var sort by remember { mutableStateOf("recent") }

    suspend fun load(replace: Boolean = true) {
        if (replace && items.isEmpty()) loading = true else refreshing = true
        error = null
        try {
            val resp = ApiClient.getFeed(
                limit = 20,
                offset = if (replace) 0 else items.size,
                topic = selectedTopic,
                sourceType = selectedSourceType,
                savedOnly = savedOnly,
                sort = sort
            )
            items = if (replace) resp.items else items + resp.items
            total = resp.total
        } catch (e: Exception) {
            error = e.message ?: "Failed to load feed"
        } finally {
            loading = false
            refreshing = false
        }
    }

    LaunchedEffect(selectedTopic, selectedSourceType, sort, savedOnly) {
        load(replace = true)
        if (topics.isEmpty() && !savedOnly) {
            topics = ApiClient.getFeedTopics()
        }
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
                Text(
                    text = if (savedOnly) "Saved" else "Feed",
                    style = MaterialTheme.typography.headlineSmall,
                    fontWeight = FontWeight.SemiBold,
                    color = MaterialTheme.colorScheme.onSurface
                )
                if (total > 0) {
                    Text(
                        text = "$total items",
                        style = MaterialTheme.typography.bodySmall,
                        color = AladinDark.TextMuted
                    )
                }
            }
            if (!savedOnly) {
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    SortChip("recent", "Recent", sort) { sort = it }
                    SortChip("signal", "Signal", sort) { sort = it }
                }
            }
        }

        if (refreshing) {
            LinearProgressIndicator(
                modifier = Modifier.fillMaxWidth(),
                color = MaterialTheme.colorScheme.primary,
                trackColor = MaterialTheme.colorScheme.surface
            )
        }

        // Source type filter
        if (!savedOnly) {
            LazyRow(
                contentPadding = PaddingValues(horizontal = 24.dp, vertical = 4.dp),
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                items(SOURCE_TYPE_FILTERS) { (value, label) ->
                    FilterChip(value, label, selectedSourceType) { selectedSourceType = it }
                }
            }
        }

        // Topic filter
        if (topics.isNotEmpty() && !savedOnly) {
            LazyRow(
                contentPadding = PaddingValues(horizontal = 24.dp, vertical = 4.dp),
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                item {
                    TopicChip(null, "All topics", selectedTopic) { selectedTopic = null }
                }
                items(topics) { topic ->
                    TopicChip(topic, topic, selectedTopic) { selectedTopic = it }
                }
            }
            Spacer(Modifier.height(4.dp))
        }

        when {
            loading -> LoadingState()
            error != null -> ErrorState(error!!) { scope.launch { load() } }
            items.isEmpty() -> EmptyState(if (savedOnly) "No saved items" else "Feed is empty")
            else -> {
                val listState = rememberLazyListState()
                LazyColumn(
                    state = listState,
                    contentPadding = PaddingValues(horizontal = 24.dp, vertical = 8.dp),
                    verticalArrangement = Arrangement.spacedBy(10.dp),
                    modifier = Modifier.fillMaxSize()
                ) {
                    items(items, key = { it.id }) { item ->
                        FeedCard(
                            item = item,
                            onSave = {
                                scope.launch {
                                    try {
                                        if (item.userStatus == "saved") ApiClient.unsaveFeedItem(item.id)
                                        else ApiClient.saveFeedItem(item.id)
                                        load()
                                    } catch (_: Exception) {}
                                }
                            },
                            onDismiss = {
                                scope.launch {
                                    try {
                                        ApiClient.dismissFeedItem(item.id)
                                        items = items.filter { it.id != item.id }
                                        total--
                                    } catch (_: Exception) {}
                                }
                            }
                        )
                    }
                    if (items.size < total) {
                        item {
                            Box(
                                Modifier.fillMaxWidth().padding(16.dp),
                                contentAlignment = Alignment.Center
                            ) {
                                TextButton(onClick = { scope.launch { load(replace = false) } }) {
                                    Text(
                                        "Load more (${total - items.size} remaining)",
                                        color = MaterialTheme.colorScheme.primary
                                    )
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun FeedCard(item: FeedItem, onSave: () -> Unit, onDismiss: () -> Unit) {
    val isSaved = item.userStatus == "saved"
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
                Column(modifier = Modifier.weight(1f).padding(end = 12.dp)) {
                    Text(
                        text = item.label,
                        style = MaterialTheme.typography.bodyLarge,
                        fontWeight = FontWeight.Medium,
                        color = MaterialTheme.colorScheme.onSurface,
                        maxLines = 2,
                        overflow = TextOverflow.Ellipsis
                    )
                    if (item.content.isNotBlank()) {
                        Spacer(Modifier.height(6.dp))
                        Text(
                            text = item.content,
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            maxLines = 3,
                            overflow = TextOverflow.Ellipsis,
                            lineHeight = 20.sp
                        )
                    }
                }
                if (item.signalScore > 0) {
                    SignalBadge(item.signalScore)
                }
            }

            Spacer(Modifier.height(12.dp))

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Row(
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    if (item.sourceName != null) {
                        SourceTag(item.sourceName, item.sourceType)
                    }
                    Text(
                        text = item.createdAt.take(10),
                        style = MaterialTheme.typography.labelSmall,
                        color = AladinDark.TextMuted
                    )
                }
                Row(horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                    TextButton(
                        onClick = onSave,
                        contentPadding = PaddingValues(horizontal = 10.dp, vertical = 4.dp)
                    ) {
                        Text(
                            if (isSaved) "Unsave" else "Save",
                            color = if (isSaved) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant,
                            style = MaterialTheme.typography.labelMedium
                        )
                    }
                    TextButton(
                        onClick = onDismiss,
                        contentPadding = PaddingValues(horizontal = 10.dp, vertical = 4.dp)
                    ) {
                        Text("Dismiss", color = AladinDark.TextMuted, style = MaterialTheme.typography.labelMedium)
                    }
                    if (item.sourceUrl != null) {
                        TextButton(
                            onClick = { openUrl(item.sourceUrl) },
                            contentPadding = PaddingValues(horizontal = 10.dp, vertical = 4.dp)
                        ) {
                            Text("Open", color = MaterialTheme.colorScheme.onSurfaceVariant, style = MaterialTheme.typography.labelMedium)
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun SignalBadge(score: Double) {
    val pct = (score * 100).toInt().coerceIn(0, 100)
    val color = when {
        pct >= 70 -> MaterialTheme.colorScheme.tertiary
        pct >= 40 -> MaterialTheme.colorScheme.tertiaryFixed
        else -> AladinDark.TextMuted
    }
    Box(
        modifier = Modifier
            .clip(RoundedCornerShape(6.dp))
            .background(color.copy(alpha = 0.15f))
            .padding(horizontal = 8.dp, vertical = 4.dp)
    ) {
        Text("$pct", style = MaterialTheme.typography.labelSmall, color = color, fontWeight = FontWeight.Bold)
    }
}

@Composable
private fun SourceTag(name: String, type: String?) {
    val prefix = when (type) {
        "reddit" -> "r/"
        "bluesky" -> "bsky"
        "twitter" -> "x"
        else -> null
    }
    Row(
        horizontalArrangement = Arrangement.spacedBy(4.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        if (prefix != null) {
            Text(prefix, style = MaterialTheme.typography.labelSmall, color = AladinDark.TextMuted, fontWeight = FontWeight.Bold)
        }
        Text(name, style = MaterialTheme.typography.labelSmall, color = AladinDark.TextMuted)
    }
}

@Composable
private fun FilterChip(value: String?, label: String, current: String?, onSelect: (String?) -> Unit) {
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
private fun SortChip(value: String, label: String, current: String, onSelect: (String) -> Unit) {
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
private fun TopicChip(value: String?, label: String, current: String?, onSelect: (String?) -> Unit) {
    val selected = value == current
    Box(
        modifier = Modifier
            .clip(RoundedCornerShape(16.dp))
            .clickable { onSelect(value) }
            .background(if (selected) MaterialTheme.colorScheme.primaryContainer else MaterialTheme.colorScheme.surface)
            .padding(horizontal = 12.dp, vertical = 6.dp)
    ) {
        Text(
            label,
            style = MaterialTheme.typography.labelSmall,
            color = if (selected) MaterialTheme.colorScheme.primary else AladinDark.TextMuted
        )
    }
}
