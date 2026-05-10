package com.jvp.aladin_compose.app

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.sp
import com.jvp.aladin_compose.features.app.AppNavigationProducerImpl
import com.jvp.aladin_compose.features.app.AppPresenter
import com.jvp.aladin_compose.features.app.AppScreen
import com.jvp.aladin_compose.features.app.AppUiFactory
import com.jvp.aladin_compose.features.app.artifactpane.WorkPaneProducerImpl
import com.jvp.aladin_compose.features.app.artifactpane.link.LinkStateProducerImpl
import com.jvp.aladin_compose.features.app.artifactpane.page.PageStateProducerImpl
import com.jvp.aladin_compose.features.app.artifactpane.voice.VoiceStateProducerImpl
import com.jvp.aladin_compose.features.app.browser.DocumentBrowserProducerImpl
import com.jvp.aladin_compose.features.app.sidebar.SidebarProducerImpl
import com.jvp.aladin_compose.features.auth.AuthEvent
import com.jvp.aladin_compose.features.auth.AuthMode
import com.jvp.aladin_compose.features.auth.AuthUi
import com.jvp.aladin_compose.features.auth.AuthUiState
import com.jvp.aladin_compose.repo.ApiFolderRepository
import com.jvp.aladin_compose.repo.ArtifactRepositoryImpl
import com.jvp.aladin_compose.repo.doa.InMemoryArtifactDoa
import com.jvp.aladin_compose.service.AuthServiceImpl
import com.jvp.aladin_compose.service.AuthenticatedUser
import com.jvp.aladin_compose.service.BrowserVoiceCaptureService
import com.jvp.aladin_compose.service.PageDocumentSyncerImpl
import com.jvp.aladin_compose.service.web.PageEditorBridgeImpl
import com.jvp.aladin_compose.ui_lib.AladinColor
import com.slack.circuit.foundation.Circuit
import com.slack.circuit.foundation.CircuitCompositionLocals
import com.slack.circuit.foundation.CircuitContent
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.launch

@Composable
fun CircuitApp() {
    val scope = rememberCoroutineScope()
    val authService = remember { AuthServiceImpl() }
    var currentUser by remember { mutableStateOf<AuthenticatedUser?>(null) }
    var checkingAuth by remember { mutableStateOf(true) }
    var email by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    var authMode by remember { mutableStateOf(AuthMode.Login) }
    var authLoading by remember { mutableStateOf(false) }
    var authError by remember { mutableStateOf<String?>(null) }

    LaunchedEffect(Unit) {
        try {
            currentUser = authService.currentUser()
        } catch (e: CancellationException) {
            throw e
        } catch (_: Throwable) {
            currentUser = null
        } finally {
            checkingAuth = false
        }
    }

    if (checkingAuth) {
        Box(
            modifier = Modifier.fillMaxSize().background(AladinColor.Canvas),
            contentAlignment = Alignment.Center,
        ) {
            Text("Checking session", color = AladinColor.InkSecondary, fontSize = 13.sp)
        }
        return
    }

    val user = currentUser
    if (user == null) {
        AuthUi(
            state =
                AuthUiState(
                    email = email,
                    password = password,
                    mode = authMode,
                    loading = authLoading,
                    errorMessage = authError,
                    eventSink = { event ->
                        when (event) {
                            is AuthEvent.EmailChanged -> {
                                email = event.email
                                authError = null
                            }
                            is AuthEvent.PasswordChanged -> {
                                password = event.password
                                authError = null
                            }
                            AuthEvent.ToggleMode -> {
                                authMode =
                                    if (authMode == AuthMode.Login) AuthMode.Register
                                    else AuthMode.Login
                                authError = null
                            }
                            AuthEvent.Submit -> {
                                if (!authLoading) {
                                    scope.launch {
                                        authLoading = true
                                        authError = null
                                        try {
                                            val response =
                                                if (authMode == AuthMode.Login) {
                                                    authService.login(email, password)
                                                } else {
                                                    authService.register(email, password)
                                                }
                                            currentUser = response
                                            password = ""
                                        } catch (e: CancellationException) {
                                            throw e
                                        } catch (t: Throwable) {
                                            authError = t.message ?: "Authentication failed"
                                        } finally {
                                            authLoading = false
                                        }
                                    }
                                }
                            }
                        }
                    },
                )
        )
        return
    }

    val circuit = remember(user.id) {
        val folderRepository = ApiFolderRepository()
        val artifactRepository = ArtifactRepositoryImpl(InMemoryArtifactDoa())
        val pageEditorBridge = PageEditorBridgeImpl()
        val pageDocumentSyncer = PageDocumentSyncerImpl(artifactRepository, scope)
        val voiceCaptureService = BrowserVoiceCaptureService()
        Circuit.Builder()
            .addPresenterFactory(
                AppPresenter.Factory(
                    user = user,
                    onLogout = {
                        scope.launch {
                            try {
                                authService.logout()
                            } catch (e: CancellationException) {
                                throw e
                            } catch (_: Throwable) {
                                // Local auth state should still reset if the server session is already gone.
                            } finally {
                                currentUser = null
                                password = ""
                                authError = null
                                authMode = AuthMode.Login
                            }
                        }
                    },
                    appNavigationProducer = AppNavigationProducerImpl(),
                    sidebarProducer = SidebarProducerImpl(),
                    documentBrowserProducer =
                        DocumentBrowserProducerImpl(
                            folderRepository = folderRepository,
                            artifactRepository = artifactRepository,
                            voiceCaptureService = voiceCaptureService,
                            scope = scope,
                        ),
                    artifactPaneProducer =
                        WorkPaneProducerImpl(
                            artifactRepository = artifactRepository,
                            pageStateProducer =
                                PageStateProducerImpl(
                                    artifactRepository = artifactRepository,
                                    pageDocumentSyncer = pageDocumentSyncer,
                                    scope = scope,
                                    pageEditorBridge = pageEditorBridge,
                                ),
                            linkStateProducer =
                                LinkStateProducerImpl(artifactRepository = artifactRepository),
                            voiceStateProducer =
                                VoiceStateProducerImpl(artifactRepository = artifactRepository),
                            pageEditorBridge = pageEditorBridge,
                        ),
                )
            )
            .addUiFactory(AppUiFactory())
            .build()
    }

    CircuitCompositionLocals(circuit) { CircuitContent(AppScreen) }
}
