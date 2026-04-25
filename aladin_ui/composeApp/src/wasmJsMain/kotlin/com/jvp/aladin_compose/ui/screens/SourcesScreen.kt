package com.jvp.aladin_compose.ui.screens

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import com.jvp.aladin_compose.api.ApiClient
import com.jvp.aladin_compose.api.Source
import com.jvp.aladin_compose.api.SourceCreateRequest
import com.jvp.aladin_compose.ui_lib.AladinDark
import com.jvp.aladin_compose.ui_lib.EmptyState
import com.jvp.aladin_compose.ui_lib.ErrorState
import com.jvp.aladin_compose.ui_lib.LoadingState
import kotlinx.coroutines.launch
import kotlinx.coroutines.yield

@Composable
fun SourcesScreen() {

    val scope = rememberCoroutineScope()
    var sources by remember { mutableStateOf<List<Source>>(emptyList()) }
    var loading by remember { mutableStateOf(true) }
    var refreshing by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    var showAddDialog by remember { mutableStateOf(false) }
    var deleteConfirm by remember { mutableStateOf<Source?>(null) }

    suspend fun load() {
        if (sources.isEmpty()) loading = true else refreshing = true
        error = null
        yield()
        try {
            sources = ApiClient.getSources()
        } catch (e: Exception) {
            error = e.message ?: "Failed to load sources"
        } finally {
            loading = false
            refreshing = false
        }
    }

    LaunchedEffect(Unit) { load() }

    Column(modifier = Modifier.fillMaxSize()) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 24.dp, vertical = 16.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceBetween
        ) {
            Column {
                Text(
                    "Sources",
                    style = MaterialTheme.typography.headlineSmall,
                    fontWeight = FontWeight.SemiBold,
                    color = MaterialTheme.colorScheme.onSurface
                )
                Text(
                    "${sources.size} connected",
                    style = MaterialTheme.typography.bodySmall,
                    color = AladinDark.TextMuted
                )
            }
            Button(
                onClick = { showAddDialog = true },
                colors = ButtonDefaults.buttonColors(containerColor = MaterialTheme.colorScheme.primary),
                contentPadding = PaddingValues(horizontal = 16.dp, vertical = 8.dp)
            ) {
                Text("+ Add Source", style = MaterialTheme.typography.labelMedium)
            }
        }

        if (refreshing) {
            LinearProgressIndicator(
                modifier = Modifier.fillMaxWidth(),
                color = MaterialTheme.colorScheme.primary,
                trackColor = MaterialTheme.colorScheme.surface
            )
        }

        when {
            loading -> LoadingState()
            error != null -> ErrorState(error!!) { scope.launch { load() } }
            sources.isEmpty() -> EmptyState("No sources yet. Add one to start syncing.")
            else -> {
                LazyColumn(
                    contentPadding = PaddingValues(horizontal = 24.dp, vertical = 8.dp),
                    verticalArrangement = Arrangement.spacedBy(10.dp),
                    modifier = Modifier.fillMaxSize()
                ) {
                    items(sources, key = { it.id }) { source ->
                        SourceCard(source = source, onDelete = { deleteConfirm = source })
                    }
                }
            }
        }
    }

    if (showAddDialog) {
        AddSourceDialog(
            onDismiss = { showAddDialog = false },
            onCreated = {
                showAddDialog = false
                scope.launch { load() }
            }
        )
    }

    deleteConfirm?.let { source ->
        AlertDialog(
            onDismissRequest = { deleteConfirm = null },
            title = { Text("Delete source?") },
            text = { Text("\"${source.name}\" and its synced data will be removed.") },
            confirmButton = {
                TextButton(
                    onClick = {
                        scope.launch {
                            try {
                                ApiClient.deleteSource(source.id)
                                load()
                            } catch (_: Exception) {}
                            deleteConfirm = null
                        }
                    }
                ) { Text("Delete", color = MaterialTheme.colorScheme.error) }
            },
            dismissButton = {
                TextButton(onClick = { deleteConfirm = null }) { Text("Cancel") }
            },
            containerColor = MaterialTheme.colorScheme.surfaceVariant
        )
    }
}

@Composable
private fun SourceCard(source: Source, onDelete: () -> Unit) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(10.dp),
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant)
    ) {
        Row(
            modifier = Modifier.padding(16.dp).fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically
        ) {
            Row(
                horizontalArrangement = Arrangement.spacedBy(12.dp),
                verticalAlignment = Alignment.CenterVertically,
                modifier = Modifier.weight(1f)
            ) {
                SourceTypeIcon(source.type)
                Column {
                    Text(
                        source.name,
                        style = MaterialTheme.typography.bodyLarge,
                        fontWeight = FontWeight.Medium,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                    Row(
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        SyncStateBadge(source.syncState)
                        if (source.lastSyncedAt != null) {
                            Text(
                                "last sync ${source.lastSyncedAt.take(10)}",
                                style = MaterialTheme.typography.labelSmall,
                                color = AladinDark.TextMuted
                            )
                        }
                    }
                }
            }
            TextButton(onClick = onDelete) {
                Text("Remove", color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.labelMedium)
            }
        }
    }
}

@Composable
private fun SourceTypeIcon(type: String) {
    val (label, bg) = when (type) {
        "reddit" -> Pair("r/", MaterialTheme.colorScheme.primaryContainer)
        "bluesky" -> Pair("bsky", MaterialTheme.colorScheme.surface)
        "twitter" -> Pair("X", MaterialTheme.colorScheme.surface)
        else -> Pair(type.take(2).uppercase(), MaterialTheme.colorScheme.surface)
    }
    Box(
        modifier = Modifier
            .size(36.dp)
            .clip(RoundedCornerShape(8.dp))
            .background(bg),
        contentAlignment = Alignment.Center
    ) {
        Text(label, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant, fontWeight = FontWeight.Bold)
    }
}

@Composable
private fun SyncStateBadge(state: String) {
    val (color, label) = when (state) {
        "syncing", "running", "active" -> Pair(MaterialTheme.colorScheme.tertiary, "active")
        "queued" -> Pair(MaterialTheme.colorScheme.tertiaryFixed, "queued")
        "idle" -> Pair(AladinDark.TextMuted, "idle")
        else -> Pair(AladinDark.TextMuted, state)
    }
    Row(
        horizontalArrangement = Arrangement.spacedBy(4.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Box(
            modifier = Modifier
                .size(6.dp)
                .clip(RoundedCornerShape(3.dp))
                .background(color)
        )
        Text(label, style = MaterialTheme.typography.labelSmall, color = color)
    }
}

// ── Add Source Dialog ─────────────────────────────────────────────────────────

private enum class SourceKind(val key: String, val label: String) {
    Reddit("reddit_subreddit", "Reddit"),
    BlueskySearch("bluesky_search", "Bluesky Search"),
    BlueskyAccount("bluesky_account", "Bluesky Account"),
    Twitter("twitter_search", "X / Twitter"),
}

@Composable
private fun AddSourceDialog(onDismiss: () -> Unit, onCreated: () -> Unit) {
    val scope = rememberCoroutineScope()
    var kind by remember { mutableStateOf(SourceKind.Reddit) }
    var name by remember { mutableStateOf("") }
    var creating by remember { mutableStateOf(false) }
    var err by remember { mutableStateOf<String?>(null) }

    // Reddit fields
    var subreddit by remember { mutableStateOf("") }
    var minScore by remember { mutableStateOf("10") }
    var topComments by remember { mutableStateOf("5") }
    var sort by remember { mutableStateOf("new") }
    var includeComments by remember { mutableStateOf(true) }

    // Bluesky/Twitter fields
    var query by remember { mutableStateOf("") }
    var handle by remember { mutableStateOf("") }
    var limit by remember { mutableStateOf("50") }
    var minLikes by remember { mutableStateOf("5") }
    var maxResults by remember { mutableStateOf("25") }

    fun resetFields() {
        subreddit = ""; query = ""; handle = ""; name = ""
    }

    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("Add Source", color = MaterialTheme.colorScheme.onSurface) },
        text = {
            Column(
                verticalArrangement = Arrangement.spacedBy(14.dp),
                modifier = Modifier.width(420.dp)
            ) {
                // Kind selector
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    SourceKind.entries.forEach { k ->
                        KindChip(k.label, kind == k) { kind = k; resetFields(); err = null }
                    }
                }

                // Common: display name
                FormField(
                    label = "Display name (optional)",
                    value = name,
                    placeholder = "Leave blank for auto",
                    onChange = { name = it }
                )

                // Per-kind fields
                when (kind) {
                    SourceKind.Reddit -> {
                        FormField("Subreddit", subreddit, "MachineLearning", { subreddit = it; err = null })
                        Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                            FormField("Min score", minScore, "10", { minScore = it }, Modifier.weight(1f),
                                keyboardType = KeyboardType.Number)
                            FormField("Top comments", topComments, "5", { topComments = it }, Modifier.weight(1f),
                                keyboardType = KeyboardType.Number)
                        }
                        Row(
                            horizontalArrangement = Arrangement.spacedBy(12.dp),
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            Column(Modifier.weight(1f)) {
                                Text("Sort", style = MaterialTheme.typography.labelSmall, color = AladinDark.TextMuted)
                                Spacer(Modifier.height(4.dp))
                                Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                                    listOf("new", "hot", "top").forEach { s ->
                                        MiniChip(s, sort == s) { sort = s }
                                    }
                                }
                            }
                            Row(
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(6.dp)
                            ) {
                                Checkbox(
                                    checked = includeComments,
                                    onCheckedChange = { includeComments = it },
                                    colors = CheckboxDefaults.colors(checkedColor = MaterialTheme.colorScheme.primary)
                                )
                                Text("Comments", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            }
                        }
                    }

                    SourceKind.BlueskySearch -> {
                        FormField("Search query", query, "open source intelligence", { query = it; err = null })
                        FormField("Items per sync", limit, "50", { limit = it },
                            keyboardType = KeyboardType.Number)
                    }

                    SourceKind.BlueskyAccount -> {
                        FormField("Handle", handle, "alice.bsky.social", { handle = it; err = null })
                        FormField("Items per sync", limit, "50", { limit = it },
                            keyboardType = KeyboardType.Number)
                    }

                    SourceKind.Twitter -> {
                        FormField("Search query", query, "knowledge graph OR vector db", { query = it; err = null })
                        Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                            FormField("Min likes", minLikes, "5", { minLikes = it }, Modifier.weight(1f),
                                keyboardType = KeyboardType.Number)
                            FormField("Max results", maxResults, "25", { maxResults = it }, Modifier.weight(1f),
                                keyboardType = KeyboardType.Number)
                        }
                    }
                }

                if (err != null) {
                    Text(err!!, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
                }
            }
        },
        confirmButton = {
            Button(
                onClick = {
                    scope.launch {
                        creating = true
                        err = null
                        try {
                            val req = when (kind) {
                                SourceKind.Reddit -> {
                                    if (subreddit.isBlank()) { err = "Subreddit is required"; return@launch }
                                    SourceCreateRequest(
                                        kind = kind.key,
                                        name = name.trimOrNull(),
                                        subreddit = subreddit.trim().removePrefix("r/"),
                                        minScore = minScore.toIntOrNull() ?: 10,
                                        includeComments = includeComments,
                                        topComments = topComments.toIntOrNull() ?: 5,
                                        sort = sort,
                                    )
                                }
                                SourceKind.BlueskySearch -> {
                                    if (query.isBlank()) { err = "Query is required"; return@launch }
                                    SourceCreateRequest(
                                        kind = kind.key,
                                        name = name.trimOrNull(),
                                        query = query.trim(),
                                        limit = limit.toIntOrNull() ?: 50,
                                    )
                                }
                                SourceKind.BlueskyAccount -> {
                                    if (handle.isBlank()) { err = "Handle is required"; return@launch }
                                    SourceCreateRequest(
                                        kind = kind.key,
                                        name = name.trimOrNull(),
                                        handle = handle.trim().removePrefix("@"),
                                        limit = limit.toIntOrNull() ?: 50,
                                    )
                                }
                                SourceKind.Twitter -> {
                                    if (query.isBlank()) { err = "Query is required"; return@launch }
                                    SourceCreateRequest(
                                        kind = kind.key,
                                        name = name.trimOrNull(),
                                        query = query.trim(),
                                        minLikes = minLikes.toIntOrNull() ?: 5,
                                        maxResults = maxResults.toIntOrNull() ?: 25,
                                    )
                                }
                            }
                            ApiClient.createSource(req)
                            onCreated()
                        } catch (e: Exception) {
                            err = e.message ?: "Failed to create source"
                        } finally {
                            creating = false
                        }
                    }
                },
                enabled = !creating,
                colors = ButtonDefaults.buttonColors(containerColor = MaterialTheme.colorScheme.primary)
            ) {
                Text(if (creating) "Adding…" else "Add")
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) { Text("Cancel") }
        },
        containerColor = MaterialTheme.colorScheme.surfaceVariant
    )
}

private fun String.trimOrNull(): String? = trim().ifBlank { null }

@Composable
private fun FormField(
    label: String,
    value: String,
    placeholder: String,
    onChange: (String) -> Unit,
    modifier: Modifier = Modifier,
    keyboardType: KeyboardType = KeyboardType.Text,
) {
    Column(modifier = modifier) {
        Text(label, style = MaterialTheme.typography.labelSmall, color = AladinDark.TextMuted)
        Spacer(Modifier.height(4.dp))
        OutlinedTextField(
            value = value,
            onValueChange = onChange,
            placeholder = { Text(placeholder, color = AladinDark.TextMuted) },
            singleLine = true,
            modifier = Modifier.fillMaxWidth(),
            keyboardOptions = KeyboardOptions(keyboardType = keyboardType),
            colors = OutlinedTextFieldDefaults.colors(
                focusedTextColor = MaterialTheme.colorScheme.onSurface,
                unfocusedTextColor = MaterialTheme.colorScheme.onSurface,
                focusedBorderColor = MaterialTheme.colorScheme.primary,
                unfocusedBorderColor = MaterialTheme.colorScheme.outline
            )
        )
    }
}

@Composable
private fun KindChip(label: String, selected: Boolean, onClick: () -> Unit) {
    Surface(
        onClick = onClick,
        shape = RoundedCornerShape(6.dp),
        color = if (selected) MaterialTheme.colorScheme.primaryContainer else MaterialTheme.colorScheme.surface
    ) {
        Text(
            label,
            modifier = Modifier.padding(horizontal = 10.dp, vertical = 6.dp),
            style = MaterialTheme.typography.labelSmall,
            color = if (selected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant
        )
    }
}

@Composable
private fun MiniChip(label: String, selected: Boolean, onClick: () -> Unit) {
    Surface(
        onClick = onClick,
        shape = RoundedCornerShape(4.dp),
        color = if (selected) MaterialTheme.colorScheme.primaryContainer else MaterialTheme.colorScheme.surface
    ) {
        Text(
            label,
            modifier = Modifier.padding(horizontal = 8.dp, vertical = 4.dp),
            style = MaterialTheme.typography.labelSmall,
            color = if (selected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant
        )
    }
}
