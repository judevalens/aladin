package dawn.system.anchor.services.auth

import io.ktor.client.HttpClient
import io.ktor.client.call.body
import io.ktor.client.request.get
import io.ktor.client.request.post
import io.ktor.client.request.setBody

/** Talks to `backend_v2`'s auth endpoints. Token handling lives in [SessionManager]. */
interface AuthService {
    suspend fun login(credentials: AuthCredentials): DesktopAuthResponse
    suspend fun me(): AladinUser
    suspend fun logout()
}

internal class KtorAuthService(private val client: HttpClient) : AuthService {

    override suspend fun login(credentials: AuthCredentials): DesktopAuthResponse =
        client.post("/api/auth/desktop/login") { setBody(credentials) }.body()

    override suspend fun me(): AladinUser =
        client.get("/api/auth/me").body<AuthUserResponse>().user

    override suspend fun logout() {
        // Revokes the bearer server-side; the local clear happens in SessionManager.
        client.post("/api/auth/logout")
    }
}
