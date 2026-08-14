package dawn.system.anchor.services.auth

import kotlinx.serialization.Serializable

/**
 * Wire types for `backend_v2`'s auth endpoints (`internal/api/auth.go`). The companion
 * uses the *desktop* variants, which return the session token in the body instead of a
 * cookie — `POST /api/auth/desktop/login` → [DesktopAuthResponse].
 */
@Serializable
data class AladinUser(
    val id: String,
    val email: String,
)

@Serializable
data class AuthCredentials(
    val email: String,
    val password: String,
)

/** Body of `POST /api/auth/desktop/{login,register}`. */
@Serializable
data class DesktopAuthResponse(
    val user: AladinUser,
    val token: String,
    /** RFC 3339 timestamp; kept as a string until something needs to compute with it. */
    val expiresAt: String,
)

/** Body of `GET /api/auth/me` (and cookie-mode login). */
@Serializable
data class AuthUserResponse(
    val user: AladinUser,
)
