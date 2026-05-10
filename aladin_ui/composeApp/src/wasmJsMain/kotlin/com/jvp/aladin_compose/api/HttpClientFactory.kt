package com.jvp.aladin_compose.api

import io.ktor.client.HttpClient
import io.ktor.client.engine.js.Js
import io.ktor.client.plugins.HttpTimeout
import io.ktor.client.plugins.contentnegotiation.ContentNegotiation
import io.ktor.client.plugins.defaultRequest
import io.ktor.http.ContentType
import io.ktor.http.contentType
import io.ktor.serialization.kotlinx.json.json
import kotlinx.serialization.json.Json

@OptIn(ExperimentalWasmJsInterop::class)
@JsFun(
    """
    () => {
        if (globalThis.__aladinCredentialedFetchInstalled) return;
        const originalFetch = globalThis.fetch.bind(globalThis);
        globalThis.fetch = (input, init) => {
            const next = init ? { ...init } : {};
            if (next.credentials === undefined) {
                next.credentials = 'include';
            }
            return originalFetch(input, next);
        };
        globalThis.__aladinCredentialedFetchInstalled = true;
    }
    """
)
private external fun installCredentialedFetch()

private val clientJson = Json {
    ignoreUnknownKeys = true
    isLenient = true
    coerceInputValues = true
    explicitNulls = false
}

val httpClient: HttpClient by lazy {
    installCredentialedFetch()
    HttpClient(Js) {
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
