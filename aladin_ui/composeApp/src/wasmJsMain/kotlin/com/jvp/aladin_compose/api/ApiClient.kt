package com.jvp.aladin_compose.api

import io.ktor.client.call.body
import io.ktor.client.plugins.ClientRequestException
import io.ktor.client.plugins.HttpRequestTimeoutException
import io.ktor.client.plugins.ServerResponseException
import io.ktor.client.request.delete
import io.ktor.client.request.forms.MultiPartFormDataContent
import io.ktor.client.request.forms.formData
import io.ktor.client.request.get
import io.ktor.client.request.parameter
import io.ktor.client.request.patch
import io.ktor.client.request.post
import io.ktor.client.request.setBody
import io.ktor.http.ContentType
import io.ktor.http.Headers
import io.ktor.http.HttpHeaders
import io.ktor.http.contentType
import kotlinx.coroutines.CancellationException

const val BASE_URL = "http://localhost:8000"

class ApiException(
    val operation: String,
    val userMessage: String,
    val statusCode: Int? = null,
    val statusText: String? = null,
    cause: Throwable? = null,
) : Exception(userMessage, cause)

object ApiClient {

    suspend fun register(req: AuthRequest): AuthResponse = call("POST /api/auth/register") {
        httpClient
            .post("$BASE_URL/api/auth/register") {
                contentType(ContentType.Application.Json)
                setBody(req)
            }
            .body()
    }

    suspend fun login(req: AuthRequest): AuthResponse = call("POST /api/auth/login") {
        httpClient
            .post("$BASE_URL/api/auth/login") {
                contentType(ContentType.Application.Json)
                setBody(req)
            }
            .body()
    }

    suspend fun logout() = call("POST /api/auth/logout") {
        httpClient.post("$BASE_URL/api/auth/logout")
    }

    suspend fun me(): AuthResponse = call("GET /api/auth/me") {
        httpClient.get("$BASE_URL/api/auth/me").body()
    }

    suspend fun getFeed(
        limit: Int = 50,
        offset: Int = 0,
        topic: String? = null,
        sourceType: String? = null,
        savedOnly: Boolean = false,
        sort: String = "recent",
    ): FeedListResponse = call {
        httpClient
            .get("$BASE_URL/api/feed/") {
                parameter("limit", limit)
                parameter("offset", offset)
                parameter("sort", sort)
                if (topic != null) parameter("topic", topic)
                if (sourceType != null) parameter("source_type", sourceType)
                if (savedOnly) parameter("saved", "true")
            }
            .body()
    }

    suspend fun saveFeedItem(id: String) = call { httpClient.post("$BASE_URL/api/feed/$id/save") }

    suspend fun dismissFeedItem(id: String) = call {
        httpClient.post("$BASE_URL/api/feed/$id/dismiss")
    }

    suspend fun unsaveFeedItem(id: String) = call {
        httpClient.post("$BASE_URL/api/feed/$id/unsave")
    }

    suspend fun getFeedTopics(): List<String> =
        try {
            call {
                httpClient.get("$BASE_URL/api/feed/topics").body<List<String>?>() ?: emptyList()
            }
        } catch (_: Throwable) {
            emptyList()
        }

    suspend fun getSources(): List<Source> = call {
        httpClient.get("$BASE_URL/api/sources/").body<List<Source>?>() ?: emptyList()
    }

    suspend fun deleteSource(id: String) = call { httpClient.delete("$BASE_URL/api/sources/$id") }

    suspend fun createSource(req: SourceCreateRequest): Source = call {
        httpClient
            .post("$BASE_URL/api/sources/") {
                contentType(ContentType.Application.Json)
                setBody(req)
            }
            .body()
    }

    suspend fun getFolders(parentId: String? = null): List<FolderRecord> =
        call("GET /api/folders") {
            httpClient
                .get("$BASE_URL/api/folders/") {
                    if (parentId != null) parameter("parentId", parentId)
                }
                .body<List<FolderRecord>?>() ?: emptyList()
        }

    suspend fun getBrowserTree(): List<BrowserTreeRecord> =
        call("GET /api/browser/tree") {
            httpClient.get("$BASE_URL/api/browser/tree").body<List<BrowserTreeRecord>?>()
                ?: emptyList()
        }

    suspend fun getFolder(id: String): FolderRecord =
        call("GET /api/folders/{id}") { httpClient.get("$BASE_URL/api/folders/$id").body() }

    suspend fun getFolderBreadcrumbs(id: String): List<BreadcrumbRecord> =
        call("GET /api/folders/{id}/breadcrumbs") {
            httpClient.get("$BASE_URL/api/folders/$id/breadcrumbs").body<List<BreadcrumbRecord>?>()
                ?: emptyList()
        }

    suspend fun createFolder(req: FolderCreateRequest): FolderRecord =
        call("POST /api/folders") {
            httpClient
                .post("$BASE_URL/api/folders/") {
                    contentType(ContentType.Application.Json)
                    setBody(req)
                }
                .body()
        }

    suspend fun updateFolder(id: String, req: FolderUpdateRequest): FolderRecord =
        call("PATCH /api/folders/{id}") {
            httpClient
                .patch("$BASE_URL/api/folders/$id") {
                    contentType(ContentType.Application.Json)
                    setBody(req)
                }
                .body()
        }

    suspend fun getUserArtifacts(folderId: String? = null): List<UserArtifact> =
        call("GET /api/artifacts") {
            httpClient
                .get("$BASE_URL/api/artifacts/") {
                    if (folderId != null) parameter("folderId", folderId)
                }
                .body<List<UserArtifact>?>() ?: emptyList()
        }

    suspend fun getUserArtifact(id: String): UserArtifact =
        call("GET /api/artifacts/{id}") { httpClient.get("$BASE_URL/api/artifacts/$id").body() }

    suspend fun getPage(id: String): PageDocumentRecord =
        call("GET /api/pages/{id}") { httpClient.get("$BASE_URL/api/pages/$id").body() }

    suspend fun savePage(id: String, req: PageSaveRequest): PageDocumentRecord =
        call("PATCH /api/pages/{id}") {
            httpClient
                .patch("$BASE_URL/api/pages/$id") {
                    contentType(ContentType.Application.Json)
                    setBody(req)
                }
                .body()
        }

    suspend fun uploadFile(req: FileUploadRequest): UploadedFileRecord =
        call("POST /api/files/upload") {
            httpClient
                .post("$BASE_URL/api/files/upload") {
                    setBody(
                        MultiPartFormDataContent(
                            formData {
                                append(
                                    "file",
                                    req.bytes,
                                    Headers.build {
                                        append(
                                            HttpHeaders.ContentDisposition,
                                            "filename=\"${req.filename}\"",
                                        )
                                        req.contentType?.let { append(HttpHeaders.ContentType, it) }
                                    },
                                )
                            }
                        )
                    )
                }
                .body()
        }

    suspend fun createUserArtifact(req: UserArtifactCreateRequest): UserArtifact =
        call("POST /api/artifacts") {
            httpClient
                .post("$BASE_URL/api/artifacts/") {
                    contentType(ContentType.Application.Json)
                    setBody(req)
                }
                .body()
        }

    suspend fun updateUserArtifact(id: String, req: UserArtifactUpdateRequest): UserArtifact =
        call("PATCH /api/artifacts/{id}") {
            httpClient
                .patch("$BASE_URL/api/artifacts/$id") {
                    contentType(ContentType.Application.Json)
                    setBody(req)
                }
                .body()
        }

    suspend fun deleteUserArtifact(id: String) =
        call("DELETE /api/artifacts/{id}") { httpClient.delete("$BASE_URL/api/artifacts/$id") }

    suspend fun uploadUserArtifact(req: UserArtifactUploadRequest): UserArtifact = call {
        httpClient
            .post("$BASE_URL/api/artifacts/upload") {
                setBody(
                    MultiPartFormDataContent(
                        formData {
                            append("type", req.type)
                            req.title?.let { append("title", it) }
                            req.summary?.let { append("summary", it) }
                            req.folderId?.let { append("folderId", it) }
                            append(
                                "file",
                                req.bytes,
                                Headers.build {
                                    append(
                                        HttpHeaders.ContentDisposition,
                                        "filename=\"${req.filename}\"",
                                    )
                                    req.contentType?.let { append(HttpHeaders.ContentType, it) }
                                },
                            )
                        }
                    )
                )
            }
            .body()
    }

    fun userArtifactResourceUrl(id: String): String = "$BASE_URL/api/artifacts/$id/resource"

    fun backendUrl(pathOrUrl: String): String {
        if (pathOrUrl.startsWith("http://") || pathOrUrl.startsWith("https://")) {
            return pathOrUrl
        }

        return if (pathOrUrl.startsWith("/")) {
            "$BASE_URL$pathOrUrl"
        } else {
            "$BASE_URL/$pathOrUrl"
        }
    }

    suspend fun getInsights(
        limit: Int = 30,
        offset: Int = 0,
        status: String = "pending",
        type: String? = null,
    ): InsightListResponse = call {
        httpClient
            .get("$BASE_URL/api/insights/") {
                parameter("limit", limit)
                parameter("offset", offset)
                parameter("status", status)
                if (type != null) parameter("type", type)
            }
            .body()
    }

    suspend fun getInsightStats(): InsightStats =
        try {
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

    suspend fun getQuote(): Quote =
        try {
            call { httpClient.get("$BASE_URL/api/quote").body() }
        } catch (_: Throwable) {
            Quote("Knowledge is power.", "Francis Bacon")
        }

    suspend fun getWorkerStatus(): Map<String, String> =
        try {
            call { httpClient.get("$BASE_URL/api/worker/status").body() }
        } catch (_: Throwable) {
            emptyMap()
        }
}

private suspend fun <T> call(operation: String = "API request", block: suspend () -> T): T =
    try {
        block()
    } catch (e: CancellationException) {
        throw e
    } catch (e: ClientRequestException) {
        val status = e.response.status
        throw ApiException(
            operation = operation,
            userMessage = "$operation failed: $status",
            statusCode = status.value,
            statusText = status.description,
            cause = e,
        )
    } catch (e: ServerResponseException) {
        val status = e.response.status
        throw ApiException(
            operation = operation,
            userMessage = "$operation server error: $status",
            statusCode = status.value,
            statusText = status.description,
            cause = e,
        )
    } catch (e: HttpRequestTimeoutException) {
        throw ApiException(
            operation = operation,
            userMessage = "$operation timed out. Is the backend running at $BASE_URL?",
            cause = e,
        )
    } catch (e: Throwable) {
        throw ApiException(
            operation = operation,
            userMessage = "$operation failed: ${e::class.simpleName}: ${e.message}",
            cause = e,
        )
    }
