package dawn.system.anchor.services.network

/**
 * Connection settings for the Aladin backend (`backend_v2`).
 *
 * Defaults point at the local dev backend on :8000. The host differs per platform
 * because the Android emulator can't see the host machine's `localhost` — see the
 * platform [defaultBaseUrl] actuals.
 */
data class ApiConfig(
    val baseUrl: String = defaultBaseUrl(),
)

/** Platform-appropriate default base URL for the dev backend. */
expect fun defaultBaseUrl(): String
