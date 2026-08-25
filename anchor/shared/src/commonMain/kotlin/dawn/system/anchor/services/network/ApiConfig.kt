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
    /**
     * Where the web app is served. The companion embeds one of its routes — the
     * BlockNote page editor — in a web view rather than reimplementing the editor
     * and a Yjs client natively.
     */
    val webBaseUrl: String = defaultWebBaseUrl(),
    /**
     * The Hocuspocus collab server, on its own port rather than the API's. This is
     * what makes the embedded editor join the *same* document desktop is editing.
     */
    val collabWsUrl: String = defaultCollabWsUrl(),

    /**
     * The board sync room server (tldraw multiplayer), a third sidecar listener beside
     * collab. Boards refuse to mount in the embed without it.
     */
    val boardSyncWsUrl: String = defaultBoardSyncWsUrl(),
) {
    /** The embed route that mounts a page's editor. */
    fun pageEditorUrl(pageId: String): String = "$webBaseUrl/embed/page/$pageId"

    /**
     * The realtime socket. Derived from [baseUrl] rather than configured separately — it is
     * the same server, and two settings that must agree are one setting too many.
     */
    fun eventsWsUrl(): String = baseUrl
        .replaceFirst("https://", "wss://")
        .replaceFirst("http://", "ws://") + "/api/events/ws"
}

/** Platform-appropriate default base URL for the dev backend. */
expect fun defaultBaseUrl(): String

/** Platform-appropriate default base URL for the web app (the Vite dev server). */
expect fun defaultWebBaseUrl(): String

/** Platform-appropriate default URL for the collab (Hocuspocus) server. */
expect fun defaultCollabWsUrl(): String

/** Platform-appropriate default URL for the board sync (tldraw rooms) server. */
expect fun defaultBoardSyncWsUrl(): String
