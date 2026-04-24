package com.jvp.aladin_compose.api

import io.ktor.client.HttpClient
import io.ktor.client.plugins.HttpTimeout
import io.ktor.client.plugins.contentnegotiation.ContentNegotiation
import io.ktor.client.plugins.defaultRequest
import io.ktor.http.ContentType
import io.ktor.http.contentType
import io.ktor.serialization.kotlinx.json.json
import kotlinx.serialization.json.Json

private val clientJson = Json {
    ignoreUnknownKeys = true
    isLenient = true
    coerceInputValues = true
    explicitNulls = false
}

expect fun createPlatformHttpClient(): HttpClient

val httpClient: HttpClient by lazy {
    createPlatformHttpClient().config {
        expectSuccess = true
        defaultRequest {
            contentType(ContentType.Application.Json)
        }
        install(ContentNegotiation) {
            json(clientJson)
        }
        install(HttpTimeout) {
            requestTimeoutMillis = 8_000
        }
    }
}
