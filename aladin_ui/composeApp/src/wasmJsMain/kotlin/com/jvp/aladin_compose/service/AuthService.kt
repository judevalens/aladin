package com.jvp.aladin_compose.service

import com.jvp.aladin_compose.api.ApiClient
import com.jvp.aladin_compose.api.AuthRequest

data class AuthenticatedUser(
    val id: String,
    val email: String,
)

interface AuthService {
    suspend fun currentUser(): AuthenticatedUser?

    suspend fun login(email: String, password: String): AuthenticatedUser

    suspend fun register(email: String, password: String): AuthenticatedUser

    suspend fun logout()
}

class AuthServiceImpl : AuthService {
    override suspend fun currentUser(): AuthenticatedUser? =
        runCatching { ApiClient.me().user.toAuthenticatedUser() }.getOrNull()

    override suspend fun login(email: String, password: String): AuthenticatedUser =
        ApiClient.login(AuthRequest(email, password)).user.toAuthenticatedUser()

    override suspend fun register(email: String, password: String): AuthenticatedUser =
        ApiClient.register(AuthRequest(email, password)).user.toAuthenticatedUser()

    override suspend fun logout() {
        ApiClient.logout()
    }
}

private fun com.jvp.aladin_compose.api.AuthUser.toAuthenticatedUser(): AuthenticatedUser =
    AuthenticatedUser(id = id, email = email)
