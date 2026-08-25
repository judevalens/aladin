package dawn.system.anchor.services.network

import io.ktor.client.engine.HttpClientEngine
import io.ktor.client.engine.darwin.Darwin
import platform.Foundation.NSBundle

actual fun httpClientEngine(): HttpClientEngine = Darwin.create()

// Build-time override, read from Info.plist. The keys are populated from xcodebuild
// settings (see scripts/ops/ipad_install.sh), so one source tree can be built against
// the dev stack or the prod stack without editing Kotlin. Unset -> the plist value
// expands to "" and the dev default below wins, which is what an Xcode run gets.
private fun buildSetting(key: String): String? =
    (NSBundle.mainBundle.objectForInfoDictionaryKey(key) as? String)?.takeIf { it.isNotBlank() }

// A physical iPad can't use localhost — that would be the iPad itself — so the default
// is the dev Mac's LAN address, which the simulator can reach too. Update it when the
// Mac's IP changes (the backend listens on all interfaces, so nothing else changes).
actual fun defaultBaseUrl(): String =
    buildSetting("AladinApiBaseUrl") ?: "http://192.168.1.109:8000"

// Vestigial: the page editor ships inside the app bundle and loads from file://, so
// nothing reads this. Kept because ApiConfig still declares the field.
actual fun defaultWebBaseUrl(): String = "http://192.168.1.109:4173"

actual fun defaultCollabWsUrl(): String =
    buildSetting("AladinCollabWsUrl") ?: "ws://192.168.1.109:3501"

actual fun defaultBoardSyncWsUrl(): String =
    buildSetting("AladinBoardSyncWsUrl") ?: "ws://192.168.1.109:3502"
