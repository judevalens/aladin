package com.jvp.aladin_compose.api

import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement

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
    val type: String = "note",
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
