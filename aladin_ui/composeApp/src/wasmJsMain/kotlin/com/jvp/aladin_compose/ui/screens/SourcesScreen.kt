package com.jvp.aladin_compose.ui.screens

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.jvp.aladin_compose.api.ApiClient
import com.jvp.aladin_compose.api.CreatedIntegrationToken
import com.jvp.aladin_compose.api.IntegrationToken
import com.jvp.aladin_compose.api.IntegrationTokenCreateRequest
import com.jvp.aladin_compose.api.ProviderConnectionProvider
import com.jvp.aladin_compose.api.Source
import com.jvp.aladin_compose.api.SourceCreateRequest
import com.jvp.aladin_compose.features.app.AppOverlayContent
import com.jvp.aladin_compose.copyTextToClipboard
import com.jvp.aladin_compose.openUrl
import com.jvp.aladin_compose.ui_lib.AladinColor
import com.jvp.aladin_compose.ui_lib.AladinInteractionDefaults
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
    var providers by remember { mutableStateOf<List<ProviderConnectionProvider>>(emptyList()) }
    var loading by remember { mutableStateOf(true) }
    var refreshing by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    var providerError by remember { mutableStateOf<String?>(null) }
    var localOverlay by remember { mutableStateOf<AppOverlayContent?>(null) }

    fun setOverlay(content: AppOverlayContent?) {
        if (setAppOverlay != null) {
            setAppOverlay(content)
        } else {
            localOverlay = content
        }
    }

    suspend fun load() {
        if (sources.isEmpty()) {
            loading = true
        } else {
            refreshing = true
        }
        error = null
        providerError = null
        yield()
        try {
            sources = ApiClient.getSources()
            try {
                providers = ApiClient.getProviderConnectionProviders().providers
            } catch (e: Exception) {
                providerError = e.message ?: "Failed to load provider connections"
            }
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

    Column(modifier = Modifier.fillMaxSize().background(AladinColor.Canvas)) {
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
            else ->
                SourcesWorkspace(
                    sources = sources,
                    providers = providers,
                    providerError = providerError,
                    onAddStream = {
                        setOverlay {
                            AddStreamDialog(
                                onDismiss = { setOverlay(null) },
                                onCreated = {
                                    setOverlay(null)
                                    scope.launch { load() }
                                },
                            )
                        }
                    },
                    onOpenIntegrations = {
                        setOverlay {
                            IntegrationsManagerDialog(
                                providers = providers,
                                providerError = providerError,
                                onDismiss = { setOverlay(null) },
                                onChanged = {
                                    load()
                                },
                            )
                        }
                    },
                    onOpenSource = { source ->
                        setOverlay {
                            SourceDetailsDialog(
                                source = source,
                                onDismiss = { setOverlay(null) },
                                onRemoveSource = {
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

    localOverlay?.invoke()
}

@Composable
private fun SourcesWorkspace(
    sources: List<Source>,
    providers: List<ProviderConnectionProvider>,
    providerError: String?,
    onAddStream: () -> Unit,
    onOpenIntegrations: () -> Unit,
    onOpenSource: (Source) -> Unit,
) {
    val activeCount = sources.count { it.syncState.isOperational() }
    val firstFetchCount = sources.count { it.lastSyncedAt == null }
    val providerCount = sources.map { it.type }.distinct().size
    val connectedAccountCount = providers.count { it.connected }

    Column(
        modifier = Modifier.fillMaxSize().padding(24.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        OverviewPanel(
            totalCount = sources.size,
            activeCount = activeCount,
            firstFetchCount = firstFetchCount,
            providerCount = providerCount,
            onAddStream = onAddStream,
            onManageIntegrations = onOpenIntegrations,
            connectedAccountCount = connectedAccountCount,
        )

        if (sources.isEmpty()) {
            EmptyStreamPanel(onAddStream = onAddStream, modifier = Modifier.fillMaxWidth())
        } else {
            StreamCardGallery(
                sources = sources,
                onOpenSource = onOpenSource,
                modifier = Modifier.fillMaxSize(),
            )
        }
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun OverviewPanel(
    totalCount: Int,
    activeCount: Int,
    firstFetchCount: Int,
    providerCount: Int,
    onAddStream: () -> Unit,
    onManageIntegrations: () -> Unit,
    connectedAccountCount: Int,
) {
    Column(
        modifier = Modifier.fillMaxWidth(),
        verticalArrangement = Arrangement.spacedBy(18.dp),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.Top,
        ) {
            Column(
                verticalArrangement = Arrangement.spacedBy(6.dp),
                modifier = Modifier.weight(1f),
            ) {
                Text(
                    "Live inputs",
                    style = MaterialTheme.typography.headlineSmall,
                    fontWeight = FontWeight.SemiBold,
                    color = AladinColor.Ink,
                )
                Text(
                    "A workspace-level index of the feeds and searches currently bringing in new material.",
                    style = MaterialTheme.typography.bodyMedium,
                    color = AladinColor.InkSecondary,
                )
                Text(
                    "Open any source card to inspect what it tracks and whether it looks healthy.",
                    style = MaterialTheme.typography.bodySmall,
                    color = AladinColor.InkMuted,
                )
            }
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                StreamDialogButton(
                    label = "Integrations ($connectedAccountCount)",
                    enabled = true,
                    primary = false,
                    onClick = onManageIntegrations,
                )
                StreamDialogButton(label = "+ Add Stream", enabled = true, primary = true, onClick = onAddStream)
            }
        }

        FlowRow(
            horizontalArrangement = Arrangement.spacedBy(24.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            OverviewMetricCard("Subscribed", totalCount.toString(), "sources currently feeding this workspace")
            OverviewMetricCard("Healthy", activeCount.toString(), "sources reporting a live or active state")
            OverviewMetricCard("Pending", firstFetchCount.toString(), "sources still waiting on a first refresh")
            OverviewMetricCard("Providers", providerCount.toString(), "upstream systems represented here")
        }
    }
}

@Composable
private fun OverviewMetricCard(label: String, value: String, description: String) {
    Column(
        modifier = Modifier.width(178.dp),
        verticalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        Text(
            label.uppercase(),
            style = MaterialTheme.typography.labelMedium,
            color = AladinColor.CodeText,
            fontWeight = FontWeight.SemiBold,
        )
        Text(
            value,
            style = MaterialTheme.typography.titleMedium,
            color = AladinColor.Ink,
            fontWeight = FontWeight.SemiBold,
        )
        Text(
            description,
            style = MaterialTheme.typography.bodySmall,
            color = AladinColor.InkMuted,
        )
    }
}

@Composable
private fun ProviderCard(
    provider: ProviderConnectionProvider,
    selected: Boolean,
    onClick: () -> Unit,
) {
    val shape = RoundedCornerShape(5.dp)
    val active = provider.available || provider.connected
    Column(
        modifier =
            Modifier.fillMaxWidth()
                .height(158.dp)
                .border(
                    width = 1.dp,
                    color =
                        when {
                            selected -> AladinColor.Ink
                            active -> AladinColor.Border
                            else -> AladinColor.Divider
                        },
                    shape = shape,
                )
                .background(
                    when {
                        selected -> AladinColor.InkSurface
                        active -> AladinColor.Panel
                        else -> AladinColor.PanelMuted
                    },
                    shape,
                )
                .aladinClickable(
                    shape = shape,
                    colors =
                        AladinInteractionDefaults.colors(
                            rest = Color.Transparent,
                            hovered = if (selected) AladinColor.InkSurfaceHover else AladinColor.RowHover,
                            pressed = AladinColor.ControlPressed,
                        ),
                    onClick = onClick,
                )
                .padding(14.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.Top,
        ) {
            ProviderIcon(provider.provider, subdued = !active, selected = selected)
            ProviderStatusLabel(provider)
        }
        Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(
                provider.label,
                style = MaterialTheme.typography.titleSmall,
                color = if (selected) AladinColor.OnInkSurface else if (active) AladinColor.Ink else AladinColor.InkMuted,
                fontWeight = FontWeight.SemiBold,
            )
            Text(
                provider.description.ifBlank { "Provider connection." },
                style = MaterialTheme.typography.bodySmall,
                color =
                    when {
                        selected -> AladinColor.OnInkSurface.copy(alpha = 0.74f)
                        active -> AladinColor.InkSecondary
                        else -> AladinColor.InkMuted
                    },
                maxLines = 2,
            )
        }
        Spacer(Modifier.weight(1f))
        if (active) {
            Text(
                provider.category.ifBlank { provider.backend }.uppercase(),
                style = MaterialTheme.typography.labelSmall.copy(letterSpacing = 0.25.sp),
                color = if (selected) AladinColor.OnInkSurface.copy(alpha = 0.68f) else AladinColor.CodeText,
                fontWeight = FontWeight.SemiBold,
            )
        }
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun IntegrationsManagerDialog(
    providers: List<ProviderConnectionProvider>,
    providerError: String?,
    onDismiss: () -> Unit,
    onChanged: suspend () -> Unit,
) {
    var selectedProviderId by remember(providers) {
        mutableStateOf(
            providers.firstOrNull { it.connected }?.provider
                ?: providers.firstOrNull { it.provider == "google" }?.provider
                ?: providers.firstOrNull()?.provider
        )
    }
    val selectedProvider =
        providers.firstOrNull { it.provider == selectedProviderId } ?: providers.firstOrNull()
    var section by remember { mutableStateOf(IntegrationManagerSection.ConnectedAccounts) }

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
                Modifier.width(980.dp)
                    .height(680.dp)
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
            Row(
                modifier = Modifier.fillMaxWidth().padding(horizontal = 24.dp, vertical = 18.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.Top,
            ) {
                Column(verticalArrangement = Arrangement.spacedBy(5.dp), modifier = Modifier.weight(1f)) {
                    Text(
                        "Integrations",
                        style = MaterialTheme.typography.headlineSmall,
                        color = AladinColor.Ink,
                        fontWeight = FontWeight.SemiBold,
                    )
                    Text(
                        "Manage third-party account connections. Available providers are emphasized; planned providers stay muted.",
                        style = MaterialTheme.typography.bodySmall,
                        color = AladinColor.InkMuted,
                    )
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                        IntegrationSectionTab(
                            label = "Connected Accounts",
                            selected = section == IntegrationManagerSection.ConnectedAccounts,
                            onClick = { section = IntegrationManagerSection.ConnectedAccounts },
                        )
                        IntegrationSectionTab(
                            label = "Agent Access",
                            selected = section == IntegrationManagerSection.AgentAccess,
                            onClick = { section = IntegrationManagerSection.AgentAccess },
                        )
                    }
                }
                StreamDialogButton(label = "Close", enabled = true, primary = false, onClick = onDismiss)
            }

            HorizontalDivider(color = AladinColor.Divider)

            when (section) {
                IntegrationManagerSection.ConnectedAccounts -> {
                    Row(modifier = Modifier.fillMaxSize()) {
                        Column(
                            modifier =
                                Modifier.width(610.dp)
                                    .fillMaxHeight()
                                    .padding(22.dp),
                            verticalArrangement = Arrangement.spacedBy(14.dp),
                        ) {
                            if (providerError != null) {
                                Text(providerError, style = MaterialTheme.typography.bodySmall, color = AladinColor.Ink)
                            }
                            ProviderGroupHeader(
                                title = "Available now",
                                description = "Configured providers you can connect or manage.",
                            )
                            LazyVerticalGrid(
                                columns = GridCells.Adaptive(minSize = 176.dp),
                                modifier = Modifier.weight(1f).fillMaxWidth(),
                                horizontalArrangement = Arrangement.spacedBy(12.dp),
                                verticalArrangement = Arrangement.spacedBy(12.dp),
                                contentPadding = androidx.compose.foundation.layout.PaddingValues(bottom = 6.dp),
                            ) {
                                items(providers, key = { it.provider }) { provider ->
                                    ProviderCard(
                                        provider = provider,
                                        selected = provider.provider == selectedProvider?.provider,
                                        onClick = { selectedProviderId = provider.provider },
                                    )
                                }
                            }
                            if (providers.isEmpty() && providerError == null) {
                                Text(
                                    "Provider catalog unavailable. Check that the backend is running.",
                                    style = MaterialTheme.typography.bodySmall,
                                    color = AladinColor.InkMuted,
                                )
                            }
                        }

                        VerticalProviderDivider()

                        Box(modifier = Modifier.weight(1f).fillMaxSize().padding(22.dp)) {
                            if (selectedProvider != null) {
                                ProviderDetailPane(provider = selectedProvider, onChanged = onChanged)
                            }
                        }
                    }
                }
                IntegrationManagerSection.AgentAccess -> AgentAccessPane(modifier = Modifier.fillMaxSize())
            }
        }
    }
}

private enum class IntegrationManagerSection {
    ConnectedAccounts,
    AgentAccess,
}

@Composable
private fun IntegrationSectionTab(label: String, selected: Boolean, onClick: () -> Unit) {
    val shape = RoundedCornerShape(5.dp)
    Box(
        modifier =
            Modifier.height(30.dp)
                .background(if (selected) AladinColor.InkSurface else Color.Transparent, shape)
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
                .padding(horizontal = 11.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            label,
            style = MaterialTheme.typography.labelMedium,
            color = if (selected) AladinColor.OnInkSurface else AladinColor.InkSecondary,
            fontWeight = FontWeight.Medium,
        )
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun AgentAccessPane(modifier: Modifier = Modifier) {
    val scope = rememberCoroutineScope()
    var tokens by remember { mutableStateOf<List<IntegrationToken>>(emptyList()) }
    var loading by remember { mutableStateOf(true) }
    var creating by remember { mutableStateOf(false) }
    var tokenName by remember { mutableStateOf("Claude Code") }
    var created by remember { mutableStateOf<CreatedIntegrationToken?>(null) }
    var copied by remember { mutableStateOf(false) }
    var revokingTokenId by remember { mutableStateOf<String?>(null) }
    var error by remember { mutableStateOf<String?>(null) }

    suspend fun loadTokens() {
        loading = true
        error = null
        try {
            tokens = ApiClient.getIntegrationTokens().tokens
        } catch (e: Exception) {
            error = e.message ?: "Failed to load integration tokens"
        } finally {
            loading = false
        }
    }

    LaunchedEffect(Unit) { loadTokens() }

    Row(modifier = modifier) {
        Column(
            modifier = Modifier.weight(1f).fillMaxHeight().padding(22.dp),
            verticalArrangement = Arrangement.spacedBy(14.dp),
        ) {
            ProviderGroupHeader(
                title = "Agent tokens",
                description = "DB-backed bearer tokens for MCP clients and local agents.",
            )
            if (error != null) {
                Text(error!!, style = MaterialTheme.typography.bodySmall, color = AladinColor.Ink)
            }
            when {
                loading ->
                    Text(
                        "Loading tokens...",
                        style = MaterialTheme.typography.bodySmall,
                        color = AladinColor.InkMuted,
                    )
                tokens.isEmpty() ->
                    AgentAccessEmptyState()
                else ->
                    LazyColumn(
                        modifier = Modifier.fillMaxSize(),
                        verticalArrangement = Arrangement.spacedBy(0.dp),
                    ) {
                        items(tokens, key = { it.id }) { token ->
                            IntegrationTokenRow(
                                token = token,
                                revoking = revokingTokenId == token.id,
                                onRevoke = {
                                    scope.launch {
                                        revokingTokenId = token.id
                                        error = null
                                        try {
                                            ApiClient.revokeIntegrationToken(token.id)
                                            loadTokens()
                                        } catch (e: Exception) {
                                            error = e.message ?: "Failed to revoke token"
                                        } finally {
                                            revokingTokenId = null
                                        }
                                    }
                                },
                            )
                        }
                    }
            }
        }

        VerticalProviderDivider()

        Column(
            modifier =
                Modifier.width(390.dp)
                    .fillMaxHeight()
                    .verticalScroll(rememberScrollState())
                    .padding(22.dp),
            verticalArrangement = Arrangement.spacedBy(13.dp),
        ) {
            Column(verticalArrangement = Arrangement.spacedBy(5.dp)) {
                Text(
                    "MCP setup",
                    style = MaterialTheme.typography.headlineSmall,
                    color = AladinColor.Ink,
                    fontWeight = FontWeight.SemiBold,
                )
                Text(
                    "Create a bearer token for the local MCP server.",
                    style = MaterialTheme.typography.bodySmall,
                    color = AladinColor.InkSecondary,
                )
            }

            AgentAccessFact("Endpoint", "http://localhost:8090/mcp")

            Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                DialogSectionLabel("Recommended scopes")
                FlowRow(
                    horizontalArrangement = Arrangement.spacedBy(10.dp),
                    verticalArrangement = Arrangement.spacedBy(4.dp),
                ) {
                    Text("artifacts:read", style = MaterialTheme.typography.bodySmall, color = AladinColor.InkSecondary)
                    Text("artifacts:write", style = MaterialTheme.typography.bodySmall, color = AladinColor.InkSecondary)
                }
            }

            HorizontalDivider(color = AladinColor.Divider)

            FormField(
                label = "Token name",
                value = tokenName,
                placeholder = "Claude Code",
                onChange = {
                    tokenName = it
                    error = null
                },
            )
            StreamDialogButton(
                label = if (creating) "Creating..." else "Create MCP Token",
                enabled = !creating && tokenName.trim().isNotEmpty(),
                primary = true,
                onClick = {
                    scope.launch {
                        val name = tokenName.trim()
                        if (name.isEmpty()) {
                            error = "Token name is required"
                            return@launch
                        }
                        creating = true
                        error = null
                        copied = false
                        try {
                            val response =
                                ApiClient.createIntegrationToken(
                                    IntegrationTokenCreateRequest(
                                        name = name,
                                        scopes = listOf("artifacts:read", "artifacts:write"),
                                    )
                                )
                            created = response
                            tokens =
                                listOf(response.integrationToken) +
                                    tokens.filterNot { it.id == response.integrationToken.id }
                        } catch (e: Exception) {
                            error = e.message ?: "Failed to create MCP token"
                        } finally {
                            creating = false
                        }
                    }
                },
            )

            created?.let { createdToken ->
                CreatedTokenBlock(
                    name = createdToken.integrationToken.name,
                    token = createdToken.token,
                    copied = copied,
                    onCopy = {
                        copyTextToClipboard(createdToken.token)
                        copied = true
                    },
                )
            }
        }
    }
}

@Composable
private fun AgentAccessEmptyState() {
    Column(verticalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.padding(top = 4.dp)) {
        Text(
            "No agent tokens yet",
            style = MaterialTheme.typography.titleSmall,
            color = AladinColor.Ink,
            fontWeight = FontWeight.SemiBold,
        )
        Text(
            "Create one from the setup panel. The raw token is shown once, then only metadata remains here.",
            style = MaterialTheme.typography.bodySmall,
            color = AladinColor.InkMuted,
        )
    }
}

@Composable
private fun IntegrationTokenRow(token: IntegrationToken, revoking: Boolean, onRevoke: () -> Unit) {
    Column(modifier = Modifier.fillMaxWidth()) {
        Row(
            modifier = Modifier.fillMaxWidth().padding(vertical = 13.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.Top,
        ) {
            Column(verticalArrangement = Arrangement.spacedBy(6.dp), modifier = Modifier.weight(1f)) {
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        token.name,
                        style = MaterialTheme.typography.titleSmall,
                        color = AladinColor.Ink,
                        fontWeight = FontWeight.SemiBold,
                    )
                    Text(
                        token.status.uppercase(),
                        style = MaterialTheme.typography.labelSmall.copy(letterSpacing = 0.25.sp),
                        color = if (token.status == "active") AladinColor.CodeText else AladinColor.InkMuted,
                        fontWeight = FontWeight.SemiBold,
                    )
                }
                Text(
                    token.scopes.joinToString(", ").ifBlank { "No scopes" },
                    style = MaterialTheme.typography.bodySmall,
                    color = AladinColor.InkSecondary,
                )
                Text(
                    "Created ${token.createdAt.humanDate()} · Last used ${token.lastUsedAt?.humanDate() ?: "never"} · Expires ${token.expiresAt?.humanDate() ?: "never"}",
                    style = MaterialTheme.typography.bodySmall,
                    color = AladinColor.InkMuted,
                )
            }
            if (token.status == "active") {
                StreamDialogButton(
                    label = if (revoking) "Revoking..." else "Revoke",
                    enabled = !revoking,
                    primary = false,
                    onClick = onRevoke,
                )
            }
        }
        HorizontalDivider(color = AladinColor.Divider)
    }
}

@Composable
private fun AgentAccessFact(label: String, value: String) {
    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
        DialogSectionLabel(label)
        Text(
            value,
            style = MaterialTheme.typography.bodyMedium,
            color = AladinColor.CodeText,
            fontWeight = FontWeight.Medium,
        )
    }
}

@Composable
private fun CreatedTokenBlock(name: String, token: String, copied: Boolean, onCopy: () -> Unit) {
    Column(
        modifier =
            Modifier.fillMaxWidth()
                .border(1.dp, AladinColor.InkSurface, RoundedCornerShape(6.dp))
                .background(AladinColor.CommandSurface, RoundedCornerShape(6.dp))
                .padding(10.dp),
        verticalArrangement = Arrangement.spacedBy(7.dp),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.Top,
        ) {
            Column(verticalArrangement = Arrangement.spacedBy(2.dp), modifier = Modifier.weight(1f)) {
                Text(
                    "One-time token reveal",
                    style = MaterialTheme.typography.labelSmall.copy(letterSpacing = 0.35.sp),
                    color = AladinColor.CodeText,
                    fontWeight = FontWeight.SemiBold,
                )
                Text(
                    name,
                    style = MaterialTheme.typography.titleSmall,
                    color = AladinColor.Ink,
                    fontWeight = FontWeight.SemiBold,
                )
            }
            Text(
                "temporary",
                style = MaterialTheme.typography.labelSmall.copy(letterSpacing = 0.25.sp),
                color = AladinColor.InkMuted,
                fontWeight = FontWeight.SemiBold,
            )
        }
        Text(
            "Copy \"$name\" now. It will not be shown again.",
            style = MaterialTheme.typography.bodySmall,
            color = AladinColor.InkSecondary,
        )
        Box(
            modifier =
                Modifier.fillMaxWidth()
                    .background(AladinColor.Panel, RoundedCornerShape(5.dp))
                    .border(1.dp, AladinColor.Divider, RoundedCornerShape(5.dp))
                    .padding(horizontal = 9.dp, vertical = 7.dp),
        ) {
            Text(
                token,
                style = MaterialTheme.typography.labelMedium,
                color = AladinColor.CodeText,
            )
        }
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
            StreamDialogButton(label = "Copy ${name.take(18)} token", enabled = true, primary = true, onClick = onCopy)
            if (copied) {
                Text(
                    "Copied",
                    style = MaterialTheme.typography.bodySmall,
                    color = AladinColor.CodeText,
                )
            }
        }
    }
}

@Composable
private fun VerticalProviderDivider() {
    Box(modifier = Modifier.width(1.dp).fillMaxSize().background(AladinColor.Divider))
}

@Composable
private fun ProviderGroupHeader(title: String, description: String) {
    Column(verticalArrangement = Arrangement.spacedBy(3.dp)) {
        Text(
            title.uppercase(),
            style = MaterialTheme.typography.labelSmall.copy(letterSpacing = 0.35.sp),
            color = AladinColor.CodeText,
            fontWeight = FontWeight.SemiBold,
        )
        Text(
            description,
            style = MaterialTheme.typography.bodySmall,
            color = AladinColor.InkMuted,
        )
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun ProviderDetailPane(provider: ProviderConnectionProvider, onChanged: suspend () -> Unit) {
    val scope = rememberCoroutineScope()
    var working by remember(provider.provider) { mutableStateOf(false) }
    var checking by remember(provider.provider) { mutableStateOf(false) }
    var err by remember(provider.provider) { mutableStateOf<String?>(null) }
    var openedConnect by remember(provider.provider) { mutableStateOf(false) }

    Column(modifier = Modifier.fillMaxSize(), verticalArrangement = Arrangement.spacedBy(18.dp)) {
        Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
            ProviderIcon(provider.provider, subdued = !provider.available && !provider.connected, selected = false)
            Column(verticalArrangement = Arrangement.spacedBy(7.dp)) {
                Text(
                    provider.label,
                    style = MaterialTheme.typography.headlineSmall,
                    color = AladinColor.Ink,
                    fontWeight = FontWeight.SemiBold,
                )
                Text(
                    provider.connectionStatusCopy(),
                    style = MaterialTheme.typography.bodyMedium,
                    color = AladinColor.InkSecondary,
                )
            }
        }

        HorizontalDivider(color = AladinColor.Divider)

        if (provider.capabilities.isNotEmpty()) {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                DialogSectionLabel("Capabilities")
                FlowRow(
                    horizontalArrangement = Arrangement.spacedBy(9.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    provider.capabilities.forEach { capability -> ProviderCapabilityTag(capability) }
                }
            }
        }

        Column(verticalArrangement = Arrangement.spacedBy(9.dp)) {
            DialogSectionLabel("Connection model")
            Text(
                if (provider.available || provider.connected) {
                    "Aladin creates a Nango Connect session. Nango owns provider credentials and refresh. Aladin stores only a local connection reference."
                } else {
                    "This provider is visible in the catalog, but no local provider config is enabled yet."
                },
                style = MaterialTheme.typography.bodySmall,
                color = AladinColor.InkSecondary,
            )
        }

        if (provider.grantedScopes.isNotEmpty()) {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                DialogSectionLabel("Granted scopes")
                Text(
                    provider.grantedScopes.joinToString(", "),
                    style = MaterialTheme.typography.bodySmall,
                    color = AladinColor.InkSecondary,
                )
            }
        }

        if (openedConnect && !provider.connected) {
            Text(
                "After Nango finishes, Aladin will persist the connection from the Nango webhook. Use Check connection if you need to reconcile manually.",
                style = MaterialTheme.typography.bodySmall,
                color = AladinColor.CodeText,
            )
        }

        err?.let { Text(it, color = AladinColor.Ink, style = MaterialTheme.typography.bodySmall) }

        Spacer(Modifier.weight(1f))

        Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
            if (provider.connected && provider.connectionId != null) {
                StreamDialogButton(
                    label = if (working) "Disconnecting..." else "Disconnect",
                    enabled = !working && !checking,
                    primary = false,
                    onClick = {
                        scope.launch {
                            working = true
                            err = null
                            try {
                                ApiClient.disconnectProviderConnection(provider.connectionId)
                                onChanged()
                            } catch (e: Exception) {
                                err = e.message ?: "Failed to disconnect ${provider.label}"
                            } finally {
                                working = false
                            }
                        }
                    },
                )
            }
            if (provider.available && !provider.connected) {
                StreamDialogButton(
                    label = if (working) "Opening..." else "Connect with Nango",
                    enabled = !working && !checking,
                    primary = true,
                    onClick = {
                        scope.launch {
                            working = true
                            err = null
                            try {
                                val session = ApiClient.startProviderConnect(provider.provider)
                                openUrl(session.connectLink)
                                openedConnect = true
                            } catch (e: Exception) {
                                err = e.message ?: "Failed to start ${provider.label} connection"
                            } finally {
                                working = false
                            }
                        }
                    },
                )
                StreamDialogButton(
                    label = if (checking) "Checking..." else "Check connection",
                    enabled = !working && !checking,
                    primary = false,
                    onClick = {
                        scope.launch {
                            checking = true
                            err = null
                            try {
                                ApiClient.syncProviderConnections()
                                onChanged()
                            } catch (e: Exception) {
                                err = e.message ?: "Failed to sync provider connections"
                            } finally {
                                checking = false
                            }
                        }
                    },
                )
            }
        }
    }
}

@Composable
private fun ProviderStatusLabel(provider: ProviderConnectionProvider) {
    val (label, color) =
        when {
            provider.connected -> "Connected" to AladinColor.CodeText
            provider.available -> "Available" to AladinColor.Ink
            provider.comingSoon -> "Later" to AladinColor.InkMuted
            else -> "Off" to AladinColor.InkMuted
        }
    Text(
        label.uppercase(),
        style = MaterialTheme.typography.labelSmall.copy(letterSpacing = 0.35.sp),
        color = color,
        fontWeight = FontWeight.SemiBold,
    )
}

@Composable
private fun ProviderCapabilityTag(label: String) {
    Text(
        label,
        style = MaterialTheme.typography.bodySmall,
        color = AladinColor.InkSecondary,
    )
}

@Composable
private fun ProviderIcon(provider: String, subdued: Boolean, selected: Boolean) {
    val label =
        when (provider) {
            "google" -> "G"
            "microsoft" -> "M"
            "github" -> "GH"
            "slack" -> "S"
            "notion" -> "N"
            "linear" -> "L"
            "discord" -> "D"
            "dropbox" -> "Db"
            "atlassian" -> "A"
            "figma" -> "F"
            else -> provider.take(2).uppercase()
        }
    Box(
        modifier =
            Modifier.size(34.dp)
                .clip(RoundedCornerShape(5.dp))
                .border(
                    1.dp,
                    when {
                        selected -> AladinColor.OnInkSurface.copy(alpha = 0.42f)
                        subdued -> AladinColor.Divider
                        else -> AladinColor.Border
                    },
                    RoundedCornerShape(5.dp),
                )
                .background(
                    when {
                        selected -> AladinColor.OnInkSurface.copy(alpha = 0.12f)
                        subdued -> AladinColor.PanelMuted
                        else -> AladinColor.CommandSurface
                    },
                    RoundedCornerShape(5.dp),
                ),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            label,
            style = MaterialTheme.typography.labelSmall,
            color =
                when {
                    selected -> AladinColor.OnInkSurface
                    subdued -> AladinColor.InkMuted
                    else -> AladinColor.CodeText
                },
            fontWeight = FontWeight.Bold,
        )
    }
}

@Composable
private fun EmptyStreamPanel(onAddStream: () -> Unit, modifier: Modifier = Modifier) {
    Row(
        modifier =
            modifier.border(1.dp, AladinColor.Border, RoundedCornerShape(6.dp))
                .background(AladinColor.Panel, RoundedCornerShape(6.dp))
                .padding(18.dp),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(verticalArrangement = Arrangement.spacedBy(5.dp), modifier = Modifier.weight(1f)) {
            Text(
                "No live streams yet",
                style = MaterialTheme.typography.titleMedium,
                color = AladinColor.Ink,
                fontWeight = FontWeight.SemiBold,
            )
            Text(
                "Add a Bluesky search stream to start feeding the global record pipeline.",
                style = MaterialTheme.typography.bodySmall,
                color = AladinColor.InkMuted,
            )
        }
        StreamDialogButton(label = "+ Add Stream", enabled = true, primary = true, onClick = onAddStream)
    }
}

@Composable
private fun StreamCardGallery(
    sources: List<Source>,
    onOpenSource: (Source) -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier = modifier) {
        Column(
            modifier = Modifier.fillMaxWidth().padding(bottom = 10.dp),
            verticalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            Text(
                "Sources",
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.SemiBold,
                color = AladinColor.Ink,
            )
            Text(
                "Each source is a compact feed object. Open one for health and detail.",
                style = MaterialTheme.typography.bodySmall,
                color = AladinColor.InkMuted,
            )
        }
        LazyVerticalGrid(
            columns = GridCells.Adaptive(minSize = 320.dp),
            modifier = Modifier.fillMaxSize(),
            contentPadding = androidx.compose.foundation.layout.PaddingValues(
                start = 2.dp,
                end = 2.dp,
                top = 12.dp,
                bottom = 24.dp,
            ),
            horizontalArrangement = Arrangement.spacedBy(16.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            items(sources, key = { it.id }) { source ->
                SourceCard(
                    source = source,
                    onClick = { onOpenSource(source) },
                    modifier = Modifier.fillMaxWidth(),
                )
            }
        }
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun SourceCard(source: Source, onClick: () -> Unit, modifier: Modifier = Modifier) {
    Box(
        modifier =
            modifier
                .heightIn(min = 188.dp)
                .border(1.dp, AladinColor.Border, RoundedCornerShape(6.dp))
                .aladinClickable(
                    shape = RoundedCornerShape(6.dp),
                    colors =
                        AladinInteractionDefaults.colors(
                            rest = AladinColor.Panel,
                            hovered = AladinColor.RowHover,
                            pressed = AladinColor.RowSelected,
                        ),
                    onClick = onClick,
                )
                .padding(16.dp),
    ) {
        Column(
            modifier = Modifier.fillMaxSize(),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.Top,
            ) {
                Row(
                    horizontalArrangement = Arrangement.spacedBy(12.dp),
                    verticalAlignment = Alignment.Top,
                    modifier = Modifier.weight(1f),
                ) {
                    SourceTypeIcon(source.type)
                    Column(verticalArrangement = Arrangement.spacedBy(3.dp)) {
                        Text(
                            source.name,
                            style = MaterialTheme.typography.titleSmall,
                            fontWeight = FontWeight.SemiBold,
                            color = AladinColor.Ink,
                        )
                        Text(
                            source.descriptionLine(),
                            style = MaterialTheme.typography.bodySmall,
                            color = AladinColor.InkSecondary,
                        )
                    }
                }
                FreshnessBadge(source)
            }

            FlowRow(
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                MetadataPill(source.providerLabel())
                MetadataPill(source.healthLabel())
                MetadataPill(source.lastRefreshSummary())
            }

            Spacer(Modifier.weight(1f))

            Text(
                "View details",
                style = MaterialTheme.typography.labelMedium,
                color = AladinColor.CodeText,
                fontWeight = FontWeight.SemiBold,
            )
        }
    }
}

@Composable
private fun SourceDetailsDialog(
    source: Source,
    onDismiss: () -> Unit,
    onRemoveSource: () -> Unit,
) {
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
                Modifier.width(640.dp)
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
                    .border(1.dp, AladinColor.Divider, RoundedCornerShape(6.dp))
                    .background(AladinColor.Panel, RoundedCornerShape(6.dp))
                    .verticalScroll(rememberScrollState()),
        ) {
            Row(
                modifier = Modifier.fillMaxWidth().padding(horizontal = 22.dp, vertical = 18.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.Top,
            ) {
                Column(
                    modifier = Modifier.weight(1f),
                    verticalArrangement = Arrangement.spacedBy(6.dp),
                ) {
                    Text(
                        source.providerLabel().uppercase(),
                        style = MaterialTheme.typography.labelMedium,
                        color = AladinColor.CodeText,
                        fontWeight = FontWeight.SemiBold,
                    )
                    Text(
                        source.name,
                        style = MaterialTheme.typography.headlineSmall,
                        fontWeight = FontWeight.SemiBold,
                        color = AladinColor.Ink,
                    )
                    Text(
                        source.descriptionLine(),
                        style = MaterialTheme.typography.bodyMedium,
                        color = AladinColor.InkSecondary,
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

            HorizontalDivider(color = AladinColor.Divider)

            Column(
                modifier = Modifier.fillMaxWidth().padding(22.dp),
                verticalArrangement = Arrangement.spacedBy(14.dp),
            ) {
                FlowRow(
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    FreshnessBadge(source)
                    MetadataPill(source.healthLabel())
                    MetadataPill(source.lastRefreshSummary())
                }

                DetailsCard(
                    title = "About this source",
                    body = source.detailsDescription(),
                )

                DetailsCard(
                    title = "Health status",
                    body = source.healthNarrative(),
                )

                DetailsCard(
                    title = "Recent activity",
                    body = source.activityNarrative(),
                )

                SourceFactsCard(source = source)
            }

            HorizontalDivider(color = AladinColor.Divider)

            Row(
                modifier = Modifier.fillMaxWidth().padding(horizontal = 22.dp, vertical = 14.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    "This only affects this workspace subscription.",
                    style = MaterialTheme.typography.bodySmall,
                    color = AladinColor.InkMuted,
                )
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    StreamDialogButton(label = "Close", enabled = true, primary = false, onClick = onDismiss)
                    StreamDialogButton(label = "Remove source", enabled = true, primary = false, onClick = onRemoveSource)
                }
            }
        }
    }
}

@Composable
private fun DetailsCard(title: String, body: String) {
    Column(
        modifier =
            Modifier.fillMaxWidth()
                .border(1.dp, AladinColor.Border, RoundedCornerShape(6.dp))
                .background(AladinColor.Canvas, RoundedCornerShape(6.dp))
                .padding(14.dp),
        verticalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        Text(
            title,
            style = MaterialTheme.typography.titleSmall,
            fontWeight = FontWeight.SemiBold,
            color = AladinColor.Ink,
        )
        Text(
            body,
            style = MaterialTheme.typography.bodySmall,
            color = AladinColor.InkSecondary,
        )
    }
}

@Composable
private fun SourceFactsCard(source: Source) {
    Column(
        modifier =
            Modifier.fillMaxWidth()
                .border(1.dp, AladinColor.Border, RoundedCornerShape(6.dp))
                .background(AladinColor.Canvas, RoundedCornerShape(6.dp)),
    ) {
        SourceFactRow("Provider", source.providerLabel())
        HorizontalDivider(color = AladinColor.Divider)
        SourceFactRow("Feed", source.streamSummary())
        HorizontalDivider(color = AladinColor.Divider)
        SourceFactRow("Created", source.createdAt.humanDate())
        HorizontalDivider(color = AladinColor.Divider)
        SourceFactRow("Last refresh", source.lastRefreshSummary())
    }
}

@Composable
private fun SourceFactRow(label: String, value: String) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(horizontal = 14.dp, vertical = 12.dp),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.Top,
    ) {
        Text(
            label.uppercase(),
            style = MaterialTheme.typography.labelMedium,
            color = AladinColor.CodeText,
            fontWeight = FontWeight.SemiBold,
        )
        Spacer(Modifier.width(16.dp))
        Text(
            value,
            style = MaterialTheme.typography.bodySmall,
            color = AladinColor.Ink,
            modifier = Modifier.weight(1f),
        )
    }
}

@Composable
private fun EmptySourcesState(onAddStream: () -> Unit) {
    Column(
        modifier = Modifier.fillMaxSize().padding(24.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Column(
            modifier =
                Modifier.width(560.dp)
                    .border(1.dp, AladinColor.Divider, RoundedCornerShape(6.dp))
                    .background(AladinColor.Panel, RoundedCornerShape(6.dp))
                    .padding(24.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
            horizontalAlignment = Alignment.Start,
        ) {
            Text(
                "No live inputs yet",
                style = MaterialTheme.typography.headlineSmall,
                fontWeight = FontWeight.SemiBold,
                color = AladinColor.Ink,
            )
            Text(
                "Sources are how this workspace stays fed. Add a search or feed once, then come back here to understand what each source is tracking.",
                style = MaterialTheme.typography.bodyMedium,
                color = AladinColor.InkSecondary,
            )
            StreamDialogButton(label = "+ Add Stream", enabled = true, primary = true, onClick = onAddStream)
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

private fun Source.providerLabel(): String =
    when (type) {
        "bluesky" -> "Bluesky"
        "hackernews" -> "Hacker News"
        "reddit" -> "Reddit"
        "twitter" -> "X"
        else -> type.replaceFirstChar { it.uppercase() }
    }

private fun Source.syncStateDisplay(): String =
    when (syncState) {
        "running", "syncing", "active" -> "active"
        "queued" -> "queued"
        "idle" -> "idle"
        else -> syncState
    }

private fun Source.syncStateNarrative(): String =
    when (syncState) {
        "running", "syncing", "active" -> "Healthy"
        "queued" -> "Queued"
        "idle" -> "Idle"
        else -> syncState.replaceFirstChar { it.uppercase() }
    }

private fun Source.healthNarrative(): String =
    when {
        lastSyncedAt == null -> "This source has been added, but Aladin has not recorded a first refresh yet."
        syncState.isOperational() -> "This source looks healthy and has refreshed recently enough to feel active."
        syncState == "queued" -> "This source is configured and waiting on the next refresh cycle."
        else -> "This source is connected, but it is not currently reporting an active refresh state."
    }

private fun Source.lastRefreshSummary(): String =
    lastSyncedAt?.humanDate()?.let { "refreshed $it" } ?: "awaiting first refresh"

private fun String.humanDate(): String = take(10)

private fun String.isOperational(): Boolean =
    this == "running" || this == "syncing" || this == "active"

private fun Source.healthLabel(): String = syncStateNarrative()

private fun Source.descriptionLine(): String =
    when (type) {
        "bluesky" -> "Tracking Bluesky posts for ${config.string("query") ?: "a search query"}."
        "hackernews" -> "Following the ${config.string("feed") ?: "topstories"} Hacker News feed."
        "reddit" -> "Watching posts from r/${config.string("subreddit") ?: "subreddit"}."
        "twitter" -> "Tracking X posts for ${config.string("query") ?: "a search query"}."
        else -> "A provider source feeding this workspace."
    }

private fun ProviderConnectionProvider.providerStatusLabel(): String =
    when {
        connected -> "Connected"
        available -> "Ready"
        comingSoon -> "Coming later"
        else -> "Not configured"
    }

private fun ProviderConnectionProvider.connectionStatusCopy(): String =
    when {
        connected -> "This account is connected and ready for future private-source syncers."
        available -> "Nango is configured for this provider. Use the connect flow to store credentials outside Aladin."
        comingSoon -> "This provider is part of the planned account catalog, but is not connectable yet."
        else -> "This provider exists in the backend catalog, but its local provider config is not enabled."
    }

private fun Source.detailsDescription(): String =
    when (type) {
        "bluesky" -> "This source follows the Bluesky search `${config.string("query") ?: "top posts"}` and brings matching posts into the workspace feed."
        "hackernews" -> "This source follows the Hacker News `${config.string("feed") ?: "topstories"}` feed and surfaces new items here as they arrive."
        "reddit" -> "This source watches r/${config.string("subreddit") ?: "subreddit"} and brings new posts into the workspace feed."
        "twitter" -> "This source follows the X search `${config.string("query") ?: "query"}` and surfaces matching posts here."
        else -> "This source is connected to an upstream provider and contributes new material to the workspace."
    }

private fun Source.activityNarrative(): String =
    when {
        lastSyncedAt == null -> "The source was added on ${createdAt.humanDate()} and is still waiting for its first recorded refresh."
        syncState.isOperational() -> "The source was added on ${createdAt.humanDate()} and last refreshed on ${lastSyncedAt.humanDate()}, which suggests it is currently feeding new material as expected."
        else -> "The source was added on ${createdAt.humanDate()} and last refreshed on ${lastSyncedAt.humanDate()}, but it does not currently look active."
    }

@Composable
private fun MetadataPill(label: String) {
    val shape = RoundedCornerShape(5.dp)
    Box(
        modifier =
            Modifier.border(1.dp, AladinColor.Border, shape)
                .background(AladinColor.PanelMuted, shape)
                .padding(horizontal = 8.dp, vertical = 5.dp),
    ) {
        Text(
            label,
            style = MaterialTheme.typography.labelSmall,
            color = AladinColor.CodeText,
        )
    }
}

@Composable
private fun FreshnessBadge(source: Source) {
    val (label, background, foreground) =
        when {
            source.lastSyncedAt == null -> Triple("New", AladinColor.CommandSurface, AladinColor.CodeText)
            source.syncState.isOperational() -> Triple("Healthy", AladinColor.ControlHover, AladinColor.Ink)
            source.syncState == "queued" -> Triple("Queued", AladinColor.CommandSurface, AladinColor.InkSecondary)
            else -> Triple("Needs review", AladinColor.CommandSurface, AladinColor.Ink)
        }
    val shape = RoundedCornerShape(5.dp)

    Box(
        modifier =
            Modifier.border(1.dp, AladinColor.Border, shape)
                .background(background, shape)
                .padding(horizontal = 8.dp, vertical = 5.dp),
    ) {
        Text(
            label,
            style = MaterialTheme.typography.labelSmall,
            color = foreground,
            fontWeight = FontWeight.Medium,
        )
    }
}

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
