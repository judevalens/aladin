package dawn.system.anchor.services.network

import io.ktor.client.engine.HttpClientEngine
import io.ktor.client.engine.okhttp.OkHttp

actual fun httpClientEngine(): HttpClientEngine = OkHttp.create()

// The Android emulator reaches the host machine at 10.0.2.2, not localhost.
actual fun defaultBaseUrl(): String = "http://10.0.2.2:8000"

actual fun defaultWebBaseUrl(): String = "http://10.0.2.2:4173"

actual fun defaultCollabWsUrl(): String = "ws://10.0.2.2:3501"
