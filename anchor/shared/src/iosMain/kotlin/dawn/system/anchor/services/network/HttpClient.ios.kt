package dawn.system.anchor.services.network

import io.ktor.client.engine.HttpClientEngine
import io.ktor.client.engine.darwin.Darwin

actual fun httpClientEngine(): HttpClientEngine = Darwin.create()

// A physical iPad can't use localhost — that would be the iPad itself — so this is the
// dev Mac's LAN address, which the simulator can reach too. Update it when the Mac's IP
// changes (the backend listens on all interfaces, so nothing else needs to change).
actual fun defaultBaseUrl(): String = "http://192.168.1.109:8000"
