package dawn.system.anchor.services.auth

import dawn.system.anchor.services.network.MutableTokenProvider
import io.ktor.client.plugins.ClientRequestException
import io.ktor.http.HttpStatusCode
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

sealed interface AuthState {
    /** Boot: stored token not yet checked. The app shows a splash, not the login form. */
    data object Unknown : AuthState

    data object LoggedOut : AuthState

    /** [user] is null when a stored token was accepted but `/me` was unreachable (offline). */
    data class LoggedIn(val user: AladinUser?) : AuthState
}

/**
 * Owns the auth lifecycle: the single source of truth for [state], and the only writer of
 * the persisted token ([TokenStorage]) and the in-flight request token
 * ([MutableTokenProvider]). Suspend functions are called from UI coroutine scopes; state
 * flows back through [state].
 */
class SessionManager(
    private val auth: AuthService,
    private val storage: TokenStorage,
    private val tokenProvider: MutableTokenProvider,
) {
    private val _state = MutableStateFlow<AuthState>(AuthState.Unknown)
    val state: StateFlow<AuthState> = _state.asStateFlow()

    /**
     * Boot path: validate any stored token against `/api/auth/me`. A 401 clears it (it
     * was revoked or expired); any other failure is treated as "offline" — the token is
     * kept and the app opens, since cached content should remain readable without a
     * network.
     */
    suspend fun restore() {
        val stored = storage.read()
        if (stored.isNullOrBlank()) {
            _state.value = AuthState.LoggedOut
            return
        }
        tokenProvider.set(stored)
        _state.value = try {
            AuthState.LoggedIn(auth.me())
        } catch (e: CancellationException) {
            throw e
        } catch (e: ClientRequestException) {
            if (e.response.status == HttpStatusCode.Unauthorized) {
                clearLocal()
                AuthState.LoggedOut
            } else {
                AuthState.LoggedIn(user = null)
            }
        } catch (_: Throwable) {
            AuthState.LoggedIn(user = null)
        }
    }

    /** Throws on failure (mapped to a message by the login screen); success flips [state]. */
    suspend fun login(email: String, password: String) {
        val session = auth.login(AuthCredentials(email.trim(), password))
        storage.write(session.token)
        tokenProvider.set(session.token)
        _state.value = AuthState.LoggedIn(session.user)
    }

    suspend fun logout() {
        try {
            auth.logout()
        } catch (e: CancellationException) {
            throw e
        } catch (_: Throwable) {
            // Best-effort revoke; the local session is dropped regardless.
        }
        clearLocal()
        _state.value = AuthState.LoggedOut
    }

    private fun clearLocal() {
        storage.write(null)
        tokenProvider.set(null)
    }
}
