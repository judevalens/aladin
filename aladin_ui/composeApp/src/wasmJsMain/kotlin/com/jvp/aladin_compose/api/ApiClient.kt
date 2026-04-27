package com.jvp.aladin_compose.api

import io.ktor.client.call.body
import io.ktor.client.plugins.ClientRequestException
import io.ktor.client.plugins.HttpRequestTimeoutException
import io.ktor.client.plugins.ServerResponseException
import io.ktor.client.request.delete
import io.ktor.client.request.get
import io.ktor.client.request.parameter
import io.ktor.client.request.patch
import io.ktor.client.request.post
import io.ktor.client.request.setBody
import io.ktor.http.ContentType
import io.ktor.http.contentType

const val BASE_URL = "http://localhost:8000"

object ApiClient {

    suspend fun getFeed(
        limit: Int = 50,
        offset: Int = 0,
        topic: String? = null,
        sourceType: String? = null,
        savedOnly: Boolean = false,
        sort: String = "recent"
    ): FeedListResponse = call {
        httpClient.get("$BASE_URL/api/feed/") {
            parameter("limit", limit)
            parameter("offset", offset)
            parameter("sort", sort)
            if (topic != null) parameter("topic", topic)
            if (sourceType != null) parameter("source_type", sourceType)
            if (savedOnly) parameter("saved", "true")
        }.body()
    }

    suspend fun saveFeedItem(id: String) = call {
        httpClient.post("$BASE_URL/api/feed/$id/save")
    }

    suspend fun dismissFeedItem(id: String) = call {
        httpClient.post("$BASE_URL/api/feed/$id/dismiss")
    }

    suspend fun unsaveFeedItem(id: String) = call {
        httpClient.post("$BASE_URL/api/feed/$id/unsave")
    }

    suspend fun getFeedTopics(): List<String> = try {
        call { httpClient.get("$BASE_URL/api/feed/topics").body() }
    } catch (_: Throwable) {
        emptyList()
    }

    suspend fun getSources(): List<Source> = call {
        httpClient.get("$BASE_URL/api/sources/").body()
    }

    suspend fun deleteSource(id: String) = call {
        httpClient.delete("$BASE_URL/api/sources/$id")
    }

    suspend fun createSource(req: SourceCreateRequest): Source = call {
        httpClient.post("$BASE_URL/api/sources/") {
            contentType(ContentType.Application.Json)
            setBody(req)
        }.body()
    }

    suspend fun getUserArtifacts(): List<UserArtifact> = call {
        httpClient.get("$BASE_URL/api/artifacts/").body()
    }

    suspend fun getUserArtifact(id: String): UserArtifact = call {
        httpClient.get("$BASE_URL/api/artifacts/$id").body()
    }

    suspend fun createUserArtifact(req: UserArtifactCreateRequest): UserArtifact = call {
        httpClient.post("$BASE_URL/api/artifacts/") {
            contentType(ContentType.Application.Json)
            setBody(req)
        }.body()
    }

    suspend fun updateUserArtifact(id: String, req: UserArtifactUpdateRequest): UserArtifact = call {
        httpClient.patch("$BASE_URL/api/artifacts/$id") {
            contentType(ContentType.Application.Json)
            setBody(req)
        }.body()
    }

    suspend fun deleteUserArtifact(id: String) = call {
        httpClient.delete("$BASE_URL/api/artifacts/$id")
    }

    suspend fun getInsights(
        limit: Int = 30,
        offset: Int = 0,
        status: String = "pending",
        type: String? = null,
    ): InsightListResponse = call {
        httpClient.get("$BASE_URL/api/insights/") {
            parameter("limit", limit)
            parameter("offset", offset)
            parameter("status", status)
            if (type != null) parameter("type", type)
        }.body()
    }

    suspend fun getInsightStats(): InsightStats = try {
        call { httpClient.get("$BASE_URL/api/insights/stats").body() }
    } catch (_: Throwable) {
        InsightStats()
    }

    suspend fun acceptInsight(id: String) = call {
        httpClient.post("$BASE_URL/api/insights/$id/accept")
    }

    suspend fun dismissInsight(id: String) = call {
        httpClient.post("$BASE_URL/api/insights/$id/dismiss")
    }

    suspend fun getQuote(): Quote = try {
        call { httpClient.get("$BASE_URL/api/quote").body() }
    } catch (_: Throwable) {
        Quote("Knowledge is power.", "Francis Bacon")
    }

    suspend fun getWorkerStatus(): Map<String, String> = try {
        call { httpClient.get("$BASE_URL/api/worker/status").body() }
    } catch (_: Throwable) {
        emptyMap()
    }

}

private suspend fun <T> call(block: suspend () -> T): T = try {
    block()
} catch (e: ClientRequestException) {
    throw IllegalStateException("Request failed: ${e.response.status}", e)
} catch (e: ServerResponseException) {
    throw IllegalStateException("Server error: ${e.response.status}", e)
} catch (e: HttpRequestTimeoutException) {
    throw IllegalStateException("Request timed out. Is the backend running at $BASE_URL?", e)
} catch (e: Throwable) {
    throw IllegalStateException("Request failed. Is the backend running at $BASE_URL?", e)
}
