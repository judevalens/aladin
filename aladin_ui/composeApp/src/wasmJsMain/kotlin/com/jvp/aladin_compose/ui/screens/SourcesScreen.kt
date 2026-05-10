package com.jvp.aladin_compose.ui.screens

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.jvp.aladin_compose.api.ApiClient
import com.jvp.aladin_compose.api.Source
import com.jvp.aladin_compose.api.SourceCreateRequest
import com.jvp.aladin_compose.features.app.AppOverlayContent
import com.jvp.aladin_compose.ui_lib.AladinColor
import com.jvp.aladin_compose.ui_lib.AladinInteractionDefaults
import com.jvp.aladin_compose.ui_lib.EmptyState
import com.jvp.aladin_compose.ui_lib.ErrorState
import com.jvp.aladin_compose.ui_lib.LoadingState
import com.jvp.aladin_compose.ui_lib.aladinClickable
import kotlinx.coroutines.launch
import kotlinx.coroutines.yield
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.contentOrNull

@Composable
fun SourcesScreen(setAppOverlay: ((AppOverlayContent?) -> Unit)? = null) {
    val scope = rememberCoroutineScope()
    var sources by remember { mutableStateOf<List<Source>>(emptyList()) }
    var loading by remember { mutableStateOf(true) }
    var refreshing by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    var localOverlay by remember { mutableStateOf<AppOverlayContent?>(null) }

    fun setOverlay(content: AppOverlayContent?) {
        if (setAppOverlay != null) {
            setAppOverlay(content)
        } else {
            localOverlay = content
        }
    }

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
    DisposableEffect(setAppOverlay) {
        onDispose {
            setAppOverlay?.invoke(null)
        }
    }

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
                    "Streams",
                    style = MaterialTheme.typography.headlineSmall,
                    fontWeight = FontWeight.SemiBold,
                    color = AladinColor.Ink
                )
                Text(
                    "${sources.size} subscribed",
                    style = MaterialTheme.typography.bodySmall,
                    color = AladinColor.InkMuted
                )
            }
            StreamDialogButton(label = "+ Add Stream", enabled = true, primary = true) {
                setOverlay {
                    AddStreamDialog(
                        onDismiss = { setOverlay(null) },
                        onCreated = {
                            setOverlay(null)
                            scope.launch { load() }
                        },
                    )
                }
            }
        }

        if (refreshing) {
            LinearProgressIndicator(
                modifier = Modifier.fillMaxWidth(),
                color = AladinColor.Ink,
                trackColor = AladinColor.Divider
            )
        }

        when {
            loading -> LoadingState()
            error != null -> ErrorState(error!!) { scope.launch { load() } }
            sources.isEmpty() -> EmptyState("No streams yet. Subscribe to one to start matching live items.")
            else -> {
                LazyColumn(
                    contentPadding = PaddingValues(horizontal = 24.dp, vertical = 8.dp),
                    verticalArrangement = Arrangement.spacedBy(10.dp),
                    modifier = Modifier.fillMaxSize()
                ) {
                    items(sources, key = { it.id }) { source ->
                        SourceCard(
                            source = source,
                            onDelete = {
                                setOverlay {
                                    RemoveStreamDialog(
                                        source = source,
                                        onDismiss = { setOverlay(null) },
                                        onRemove = {
                                            scope.launch {
                                                try {
                                                    ApiClient.deleteSource(source.id)
                                                    load()
                                                } catch (_: Exception) {
                                                } finally {
                                                    setOverlay(null)
                                                }
                                            }
                                        },
                                    )
                                }
                            },
                        )
                    }
                }
            }
        }
    }

    localOverlay?.invoke()
}

@Composable
private fun SourceCard(source: Source, onDelete: () -> Unit) {
    Box(
        modifier =
            Modifier.fillMaxWidth()
                .border(1.dp, AladinColor.Border, RoundedCornerShape(6.dp))
                .background(AladinColor.Panel, RoundedCornerShape(6.dp))
                .padding(14.dp)
    ) {
        Row(
            modifier = Modifier.fillMaxWidth(),
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
                        color = AladinColor.Ink
                    )
                    Row(
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        SyncStateBadge(source.syncState)
                        Text(
                            source.streamSummary(),
                            style = MaterialTheme.typography.labelSmall,
                            color = AladinColor.InkMuted
                        )
                        if (source.lastSyncedAt != null) {
                            Text(
                                "last refresh ${source.lastSyncedAt.take(10)}",
                                style = MaterialTheme.typography.labelSmall,
                                color = AladinColor.InkMuted
                            )
                        }
                    }
                }
            }
            StreamDialogButton(label = "Remove", enabled = true, primary = false) {
                onDelete()
            }
        }
    }
}

private fun Source.streamSummary(): String =
    when (type) {
        "bluesky" -> "search: ${config.string("query") ?: "top posts"}"
        "hackernews" -> "feed: ${config.string("feed") ?: "topstories"}"
        "reddit" -> "r/${config.string("subreddit") ?: "subreddit"}"
        "twitter" -> "search: ${config.string("query") ?: "query"}"
        else -> config.string("mode") ?: "provider stream"
    }

private fun Map<String, JsonElement>.string(key: String): String? =
    (this[key] as? JsonPrimitive)?.contentOrNull

@Composable
private fun RemoveStreamDialog(source: Source, onDismiss: () -> Unit, onRemove: () -> Unit) {
    Box(
        modifier =
            Modifier.fillMaxSize()
                .background(Color(0x66000000))
                .aladinClickable(
                    shape = RoundedCornerShape(0.dp),
                    colors =
                        AladinInteractionDefaults.colors(
                            rest = Color.Transparent,
                            hovered = Color.Transparent,
                            pressed = Color.Transparent,
                        ),
                    onClick = onDismiss,
                ),
        contentAlignment = Alignment.Center,
    ) {
        Column(
            modifier =
                Modifier.width(430.dp)
                    .aladinClickable(
                        shape = RoundedCornerShape(6.dp),
                        colors =
                            AladinInteractionDefaults.colors(
                                rest = Color.Transparent,
                                hovered = Color.Transparent,
                                pressed = Color.Transparent,
                            ),
                        onClick = {},
                    )
                    .border(1.dp, AladinColor.Ink, RoundedCornerShape(6.dp))
                    .background(AladinColor.Panel, RoundedCornerShape(6.dp)),
        ) {
            Column(
                modifier = Modifier.fillMaxWidth().padding(horizontal = 20.dp, vertical = 18.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                Text(
                    "Remove stream subscription?",
                    color = AladinColor.Ink,
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.SemiBold,
                )
                Text(
                    "\"${source.name}\" will stop matching new items for this workspace. The shared provider stream can still be used elsewhere.",
                    color = AladinColor.InkSecondary,
                    style = MaterialTheme.typography.bodySmall,
                )
            }
            HorizontalDivider(color = AladinColor.Divider)
            Row(
                modifier = Modifier.fillMaxWidth().padding(horizontal = 20.dp, vertical = 14.dp),
                horizontalArrangement = Arrangement.End,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    StreamDialogButton(label = "Cancel", enabled = true, primary = false, onClick = onDismiss)
                    StreamDialogButton(label = "Remove", enabled = true, primary = true, onClick = onRemove)
                }
            }
        }
    }
}

@Composable
private fun SourceTypeIcon(type: String) {
    val label = when (type) {
        "reddit" -> "r/"
        "bluesky" -> "bsky"
        "twitter" -> "X"
        "hackernews" -> "HN"
        else -> type.take(2).uppercase()
    }
    Box(
        modifier = Modifier
            .size(36.dp)
            .clip(RoundedCornerShape(5.dp))
            .border(1.dp, AladinColor.Border, RoundedCornerShape(5.dp))
            .background(AladinColor.CommandSurface, RoundedCornerShape(5.dp)),
        contentAlignment = Alignment.Center
    ) {
        Text(label, style = MaterialTheme.typography.labelSmall, color = AladinColor.CodeText, fontWeight = FontWeight.Bold)
    }
}

@Composable
private fun SyncStateBadge(state: String) {
    val (color, label) = when (state) {
        "syncing", "running", "active" -> Pair(AladinColor.Ink, "active")
        "queued" -> Pair(AladinColor.CodeText, "queued")
        "idle" -> Pair(AladinColor.InkMuted, "idle")
        else -> Pair(AladinColor.InkMuted, state)
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

// ── Add Stream Dialog ─────────────────────────────────────────────────────────

private enum class StreamProvider(
    val label: String,
    val eyebrow: String,
    val description: String,
    val enabled: Boolean,
) {
    Bluesky(
        "Bluesky",
        "Search",
        "Top posts from a Bluesky search query.",
        true,
    ),
    HackerNews(
        "Hacker News",
        "Feed",
        "Global feed stream support is next.",
        false,
    ),
    Reddit(
        "Reddit",
        "Subreddit",
        "Paused while provider limits are restrictive.",
        false,
    ),
    X(
        "X",
        "Search",
        "Search streams are planned.",
        false,
    ),
}

@Composable
private fun AddStreamDialog(onDismiss: () -> Unit, onCreated: () -> Unit) {
    val scope = rememberCoroutineScope()
    var provider by remember { mutableStateOf(StreamProvider.Bluesky) }
    var creating by remember { mutableStateOf(false) }
    var err by remember { mutableStateOf<String?>(null) }
    var query by remember { mutableStateOf("") }
    val addEnabled = !creating && provider == StreamProvider.Bluesky

    Box(
        modifier =
            Modifier.fillMaxSize()
                .background(Color(0x66000000))
                .aladinClickable(
                    shape = RoundedCornerShape(0.dp),
                    colors =
                        AladinInteractionDefaults.colors(
                            rest = Color.Transparent,
                            hovered = Color.Transparent,
                            pressed = Color.Transparent,
                        ),
                    onClick = onDismiss,
                ),
        contentAlignment = Alignment.Center,
    ) {
        Column(
            modifier =
                Modifier.width(520.dp)
                    .height(560.dp)
                    .aladinClickable(
                        shape = RoundedCornerShape(6.dp),
                        colors =
                            AladinInteractionDefaults.colors(
                                rest = Color.Transparent,
                                hovered = Color.Transparent,
                                pressed = Color.Transparent,
                            ),
                        onClick = {},
                    )
                    .border(1.dp, AladinColor.Ink, RoundedCornerShape(6.dp))
                    .background(AladinColor.Panel, RoundedCornerShape(6.dp)),
        ) {
            Column(
                modifier = Modifier.fillMaxWidth().padding(horizontal = 22.dp, vertical = 18.dp),
                verticalArrangement = Arrangement.spacedBy(5.dp),
            ) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.Top,
                ) {
                    Column(verticalArrangement = Arrangement.spacedBy(5.dp)) {
                        Text(
                            "Add Stream",
                            color = AladinColor.Ink,
                            style = MaterialTheme.typography.titleLarge,
                            fontWeight = FontWeight.SemiBold,
                        )
                        Text(
                            "Subscribe once. Let the backend handle fetch policy.",
                            color = AladinColor.InkMuted,
                            style = MaterialTheme.typography.bodySmall,
                        )
                    }
                    Text(
                        "ESC",
                        modifier =
                            Modifier.border(1.dp, AladinColor.Border, RoundedCornerShape(4.dp))
                                .background(AladinColor.CommandSurface, RoundedCornerShape(4.dp))
                                .aladinClickable(
                                    shape = RoundedCornerShape(4.dp),
                                    colors =
                                        AladinInteractionDefaults.colors(
                                            rest = Color.Transparent,
                                            hovered = AladinColor.ControlHover,
                                            pressed = AladinColor.ControlPressed,
                                        ),
                                    onClick = onDismiss,
                                )
                                .padding(horizontal = 8.dp, vertical = 4.dp),
                        color = AladinColor.CodeText,
                        style = MaterialTheme.typography.labelSmall,
                        fontWeight = FontWeight.SemiBold,
                    )
                }
            }

            HorizontalDivider(color = AladinColor.Divider)

            Column(
                verticalArrangement = Arrangement.spacedBy(18.dp),
                modifier =
                    Modifier.weight(1f)
                        .fillMaxWidth()
                        .verticalScroll(rememberScrollState())
                        .padding(horizontal = 22.dp, vertical = 18.dp),
            ) {
                DialogSectionLabel("Source")
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    StreamProvider.entries.forEach { option ->
                        SourceTab(
                            provider = option,
                            selected = provider == option,
                            onClick = {
                                provider = option
                                err = null
                            },
                            modifier = Modifier.weight(1f),
                        )
                    }
                }

                Column(
                    modifier =
                        Modifier.fillMaxWidth()
                            .border(1.dp, AladinColor.Border, RoundedCornerShape(6.dp))
                            .background(AladinColor.Canvas, RoundedCornerShape(6.dp))
                            .padding(14.dp),
                    verticalArrangement = Arrangement.spacedBy(13.dp),
                ) {
                    Text(
                        provider.description,
                        style = MaterialTheme.typography.bodySmall,
                        color = AladinColor.InkSecondary,
                    )

                    if (provider == StreamProvider.Bluesky) {
                        FormField(
                            label = "Search query",
                            value = query,
                            placeholder = "ai agents",
                            onChange = { query = it; err = null },
                        )
                    } else {
                        UnsupportedProviderNotice(provider)
                    }
                }

                Text(
                    "The query identifies the shared provider stream. Refresh cadence, result limits, dedupe, and matching are backend-owned.",
                    style = MaterialTheme.typography.labelSmall,
                    color = AladinColor.InkMuted,
                )

                if (err != null) {
                    Text(err!!, color = AladinColor.Ink, style = MaterialTheme.typography.bodySmall)
                }
            }

            HorizontalDivider(color = AladinColor.Divider)

            Row(
                modifier = Modifier.fillMaxWidth().padding(horizontal = 22.dp, vertical = 14.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    "Provider stream subscription",
                    color = AladinColor.CodeText,
                    style = MaterialTheme.typography.labelSmall,
                    fontWeight = FontWeight.SemiBold,
                )
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    StreamDialogButton(label = "Cancel", enabled = !creating, primary = false, onClick = onDismiss)
                    StreamDialogButton(
                        label = if (creating) "Adding..." else "Add stream",
                        enabled = addEnabled,
                        primary = true,
                        onClick = {
                            scope.launch {
                                creating = true
                                err = null
                                try {
                                    if (provider != StreamProvider.Bluesky) {
                                        err = "${provider.label} streams are not supported yet"
                                        return@launch
                                    }
                                    if (query.isBlank()) {
                                        err = "Search query is required"
                                        return@launch
                                    }
                                    val req =
                                        SourceCreateRequest(
                                            kind = "bluesky_search",
                                            query = query.trim(),
                                        )
                                    ApiClient.createSource(req)
                                    onCreated()
                                } catch (e: Exception) {
                                    err = e.message ?: "Failed to create stream"
                                } finally {
                                    creating = false
                                }
                            }
                        },
                    )
                }
            }
        }
    }
}

@Composable
private fun DialogSectionLabel(label: String) {
    Text(
        label.uppercase(),
        style = MaterialTheme.typography.labelSmall.copy(letterSpacing = 0.35.sp),
        color = AladinColor.CodeText,
        fontWeight = FontWeight.SemiBold,
    )
}

@Composable
private fun SourceTab(
    provider: StreamProvider,
    selected: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val shape = RoundedCornerShape(5.dp)
    Box(
        modifier =
            modifier.height(68.dp)
                .border(1.dp, if (selected) AladinColor.InkSurface else AladinColor.Border, shape)
                .background(
                    when {
                        selected -> AladinColor.InkSurface
                        provider.enabled -> AladinColor.Panel
                        else -> AladinColor.PanelMuted
                    },
                    shape,
                )
                .aladinClickable(
                    shape = shape,
                    colors =
                        AladinInteractionDefaults.colors(
                            rest = Color.Transparent,
                            hovered = if (selected) AladinColor.InkSurfaceHover else AladinColor.ControlHover,
                            pressed = if (selected) AladinColor.InkSurfaceHover else AladinColor.ControlPressed,
                        ),
                    onClick = onClick,
                )
                .padding(horizontal = 10.dp, vertical = 9.dp),
    ) {
        Column(
            verticalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            Text(
                provider.eyebrow.uppercase(),
                style = MaterialTheme.typography.labelSmall.copy(letterSpacing = 0.25.sp),
                color = if (selected) AladinColor.OnInkSurface.copy(alpha = 0.72f) else AladinColor.InkMuted,
            )
            Text(
                provider.label,
                style = MaterialTheme.typography.labelMedium,
                color =
                    when {
                        selected -> AladinColor.OnInkSurface
                        provider.enabled -> AladinColor.Ink
                        else -> AladinColor.InkDisabled
                    },
                fontWeight = FontWeight.Medium,
            )
        }
    }
}

@Composable
private fun UnsupportedProviderNotice(provider: StreamProvider) {
    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
        Text(
            "Not wired yet",
            style = MaterialTheme.typography.labelMedium,
            color = AladinColor.Ink,
            fontWeight = FontWeight.Medium,
        )
        Text(
            "${provider.label} will reuse this same subscription surface once the backend stream is ready.",
            style = MaterialTheme.typography.bodySmall,
            color = AladinColor.InkMuted,
        )
    }
}

@Composable
private fun FormField(
    label: String,
    value: String,
    placeholder: String,
    onChange: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier = modifier) {
        Text(
            label.uppercase(),
            style = MaterialTheme.typography.labelSmall.copy(letterSpacing = 0.35.sp),
            color = AladinColor.CodeText,
            fontWeight = FontWeight.SemiBold,
        )
        Spacer(Modifier.height(6.dp))
        BasicTextField(
            value = value,
            onValueChange = onChange,
            singleLine = true,
            textStyle =
                TextStyle(
                    color = AladinColor.Ink,
                    fontSize = 14.sp,
                    lineHeight = 21.sp,
                    fontWeight = FontWeight.Medium,
                ),
            modifier =
                Modifier.fillMaxWidth()
                    .height(38.dp)
                    .border(1.dp, AladinColor.Border, RoundedCornerShape(5.dp))
                    .background(AladinColor.CommandSurface, RoundedCornerShape(5.dp))
                    .padding(horizontal = 10.dp, vertical = 9.dp),
            decorationBox = { innerTextField ->
                Box(contentAlignment = Alignment.CenterStart) {
                    if (value.isEmpty()) {
                        Text(
                            placeholder,
                            color = AladinColor.InkMuted,
                            style = MaterialTheme.typography.bodyMedium,
                        )
                    }
                    innerTextField()
                }
            },
        )
    }
}

@Composable
private fun StreamDialogButton(label: String, enabled: Boolean, primary: Boolean, onClick: () -> Unit) {
    val shape = RoundedCornerShape(5.dp)
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
                .border(1.dp, if (primary) AladinColor.InkSurface else AladinColor.Border, shape)
                .then(
                    if (enabled) {
                        Modifier.aladinClickable(
                            shape = shape,
                            colors =
                                AladinInteractionDefaults.colors(
                                    rest = Color.Transparent,
                                    hovered = if (primary) AladinColor.InkSurfaceHover else AladinColor.ControlHover,
                                    pressed = if (primary) AladinColor.InkSurfaceHover else AladinColor.ControlPressed,
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
            label,
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
