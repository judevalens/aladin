package com.jvp.aladin_compose.api

import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement

@Serializable
data class AuthRequest(
    val email: String,
    val password: String,
)

@Serializable
data class AuthUser(
    val id: String,
    val email: String,
)

@Serializable
data class AuthResponse(
    val user: AuthUser,
)

@Serializable
data class IntegrationToken(
    val id: String,
    val name: String,
    val scopes: List<String> = emptyList(),
    val status: String,
    val expiresAt: String? = null,
    val createdAt: String,
    val lastUsedAt: String? = null,
)

@Serializable
data class IntegrationTokenListResponse(
    val tokens: List<IntegrationToken> = emptyList(),
)

@Serializable
data class IntegrationTokenCreateRequest(
    val name: String,
    val scopes: List<String>,
    val expiresAt: String? = null,
)

@Serializable
data class CreatedIntegrationToken(
    val token: String,
    val integrationToken: IntegrationToken,
)

@Serializable
data class FeedItem(
    val id: String,
    val type: String,
    val label: String,
    val content: String,
    val sourceUrl: String? = null,
    val userStatus: String? = null,
    val sourceType: String? = null,
    val sourceName: String? = null,
    val signalScore: Double = 0.0,
    val createdAt: String
)

@Serializable
data class FeedListResponse(
    val items: List<FeedItem> = emptyList(),
    val total: Int = 0,
    val limit: Int = 50,
    val offset: Int = 0
)

@Serializable
data class Source(
    val id: String,
    val name: String,
    val type: String,
    val syncMode: String,
    val syncState: String,
    val config: Map<String, JsonElement> = emptyMap(),
    val autoPromoteThreshold: Double = 0.0,
    val suggestThreshold: Double = 0.0,
    val createdAt: String,
    val lastSyncedAt: String? = null
)

@Serializable
data class SourceCreateRequest(
    val kind: String,
    val name: String? = null,
    val subreddit: String? = null,
    val feed: String? = null,
    val minScore: Int? = null,
    val includeComments: Boolean? = null,
    val topComments: Int? = null,
    val sort: String? = null,
    val query: String? = null,
    val handle: String? = null,
    val limit: Int? = null,
    val minLikes: Int? = null,
    val maxResults: Int? = null,
)

@Serializable
data class ProviderConnectionProvider(
    val provider: String,
    val label: String,
    val backend: String,
    val providerConfigKey: String = "",
    val description: String = "",
    val category: String = "",
    val capabilities: List<String> = emptyList(),
    val comingSoon: Boolean = false,
    val available: Boolean = false,
    val connected: Boolean = false,
    val connectionId: String? = null,
    val grantedScopes: List<String> = emptyList(),
)

@Serializable
data class ProviderConnectionProvidersResponse(
    val providers: List<ProviderConnectionProvider> = emptyList(),
)

@Serializable
data class ProviderConnectionRecord(
    val id: String,
    val provider: String,
    val backend: String,
    val providerConfigKey: String,
    val externalConnectionId: String,
    val status: String,
    val grantedScopes: List<String> = emptyList(),
    val metadata: Map<String, JsonElement> = emptyMap(),
    val createdAt: String,
    val updatedAt: String,
    val disconnectedAt: String? = null,
)

@Serializable
data class ProviderConnectionListResponse(
    val connections: List<ProviderConnectionRecord> = emptyList(),
)

@Serializable
data class ProviderConnectSession(
    val connectSessionToken: String,
    val connectLink: String,
    val expiresAt: String? = null,
    val nangoBaseUrl: String,
    val nangoConnectBaseUrl: String,
)

@Serializable
data class Insight(
    val id: String,
    val type: String,
    val title: String,
    val body: String,
    val entity: String? = null,
    val topic: String? = null,
    val recordIds: List<String> = emptyList(),
    val confidence: Double = 0.0,
    val userStatus: String = "pending",
    val createdAt: String
)

@Serializable
data class InsightListResponse(
    val items: List<Insight> = emptyList(),
    val total: Int = 0
)

@Serializable
data class InsightStats(
    val byType: Map<String, Int> = emptyMap(),
    val byStatus: Map<String, Int> = emptyMap(),
)

@Serializable
data class WorkerStatus(
    val status: String = "unknown",
    val queuedJobs: Int = 0,
    val activeWorkers: Int = 0
)

@Serializable
data class Quote(
    val text: String,
    val author: String
)

@Serializable
data class UserArtifact(
    val id: String,
    val type: String,
    val folderId: String? = null,
    val title: String,
    val content: String,
    val summary: String? = null,
    val sourceUrl: String? = null,
    val metadata: Map<String, JsonElement> = emptyMap(),
    val createdAt: String,
    val updatedAt: String,
)

@Serializable
data class UserArtifactCreateRequest(
    val type: String = "page",
    val folderId: String? = null,
    val title: String = "",
    val content: String = "",
    val summary: String? = null,
    val sourceUrl: String? = null,
    val metadata: Map<String, JsonElement> = emptyMap(),
)

@Serializable
data class UserArtifactUpdateRequest(
    val type: String? = null,
    val folderId: String? = null,
    val title: String? = null,
    val content: String? = null,
    val summary: String? = null,
    val sourceUrl: String? = null,
    val metadata: Map<String, JsonElement>? = null,
)

@Serializable
data class PageDocumentRecord(
    val id: String,
    val title: String,
    val content: String,
    val revision: Long,
    val updatedAt: String,
)

@Serializable
data class PageSaveRequest(
    val content: String,
    val revision: Long,
)

@Serializable
data class UploadedFileRecord(
    val id: String,
    val url: String,
    val uploadedAt: String,
)

@Serializable
data class FolderRecord(
    val id: String,
    val parentId: String? = null,
    val title: String,
)

@Serializable
data class BrowserTreeRecord(
    val id: String,
    val parentId: String? = null,
    val kind: String,
    val title: String,
    val artifactId: String? = null,
    val artifactType: String? = null,
    val updatedAt: String? = null,
    val children: List<BrowserTreeRecord> = emptyList(),
)

@Serializable
data class FolderCreateRequest(
    val title: String,
    val parentId: String? = null,
)

@Serializable
data class FolderUpdateRequest(
    val title: String,
)

@Serializable
data class BreadcrumbRecord(
    val id: String? = null,
    val label: String,
)

data class UserArtifactUploadRequest(
    val type: String,
    val filename: String,
    val bytes: ByteArray,
    val contentType: String? = null,
    val title: String? = null,
    val summary: String? = null,
    val folderId: String? = null,
)

data class FileUploadRequest(
    val filename: String,
    val bytes: ByteArray,
    val contentType: String? = null,
)
