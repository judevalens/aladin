package dawn.system.anchor.services.network

import io.ktor.client.HttpClient
import io.ktor.client.engine.HttpClientEngine
import io.ktor.client.plugins.HttpTimeout
import io.ktor.client.plugins.contentnegotiation.ContentNegotiation
import io.ktor.client.plugins.defaultRequest
import io.ktor.client.plugins.logging.LogLevel
import io.ktor.client.plugins.logging.Logging
import io.ktor.client.request.header
import io.ktor.http.ContentType
import io.ktor.http.HttpHeaders
import io.ktor.http.contentType
import io.ktor.serialization.kotlinx.json.json
import kotlinx.serialization.json.Json

/** Shared JSON config. Lenient so backend additions don't break older clients. */
val anchorJson: Json = Json {
    ignoreUnknownKeys = true
    isLenient = true
    encodeDefaults = true
    explicitNulls = false
}

/** Platform-specific Ktor engine: OkHttp on Android, Darwin on iOS. */
expect fun httpClientEngine(): HttpClientEngine

/**
 * Builds the configured Ktor client for the Aladin backend: JSON negotiation, request
 * logging, timeouts, and a per-request bearer token pulled from [tokenProvider].
 */
fun createHttpClient(
    config: ApiConfig = ApiConfig(),
    tokenProvider: TokenProvider,
    engine: HttpClientEngine = httpClientEngine(),
): HttpClient = HttpClient(engine) {
    expectSuccess = true

    install(ContentNegotiation) {
        json(anchorJson)
    }

    install(Logging) {
        level = LogLevel.INFO
    }

    install(HttpTimeout) {
        requestTimeoutMillis = 30_000
        connectTimeoutMillis = 15_000
        socketTimeoutMillis = 30_000
    }

    defaultRequest {
        url(config.baseUrl)
        contentType(ContentType.Application.Json)
        tokenProvider.currentToken()?.let { token ->
            // bearerAuth sets the Authorization header; re-evaluated per request so a
            // post-login token takes effect without rebuilding the client.
            header(HttpHeaders.Authorization, "Bearer $token")
        }
    }
}
