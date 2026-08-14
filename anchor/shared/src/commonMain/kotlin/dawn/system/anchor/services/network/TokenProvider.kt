package dawn.system.anchor.services.network

import kotlin.concurrent.Volatile

/**
 * Supplies the current bearer token for outgoing requests. The backend authenticates
 * desktop/mobile clients with `Authorization: Bearer <token>` (see
 * `backend_v2/internal/api/auth.go`).
 */
interface TokenProvider {
    fun currentToken(): String?
}

/** Write side, held by the session layer; the HTTP client only sees [TokenProvider]. */
interface MutableTokenProvider : TokenProvider {
    fun set(value: String?)
}

/**
 * Simple in-memory token holder. [SessionManager][dawn.system.anchor.services.auth.SessionManager]
 * seeds it from [TokenStorage][dawn.system.anchor.services.auth.TokenStorage] at boot and
 * updates it on login/logout.
 */
class InMemoryTokenProvider(initial: String? = null) : MutableTokenProvider {
    @Volatile
    private var token: String? = initial

    override fun currentToken(): String? = token

    override fun set(value: String?) {
        token = value
    }
}
