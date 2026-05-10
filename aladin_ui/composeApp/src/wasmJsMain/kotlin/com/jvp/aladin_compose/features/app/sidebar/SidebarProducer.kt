package com.jvp.aladin_compose.features.app.sidebar

import androidx.compose.runtime.Composable
import com.jvp.aladin_compose.model.NavDestination

interface SidebarProducer {
    @Composable
    fun produce(
        selectedDestination: NavDestination,
        userEmail: String,
        canCreateArtifact: Boolean,
        onNavigate: (NavDestination) -> Unit,
        onCreateFolder: () -> Unit,
        onCreateArtifact: () -> Unit,
        onCreateVoice: () -> Unit,
        onLogout: () -> Unit,
    ): SidebarState
}

data class SidebarState(
    val selectedDestination: NavDestination,
    val userEmail: String,
    val canCreateArtifact: Boolean,
    val eventSink: (SidebarEvent) -> Unit,
)

sealed interface SidebarEvent {
    data class Navigate(val destination: NavDestination) : SidebarEvent

    data object CreateFolder : SidebarEvent

    data object CreateArtifact : SidebarEvent

    data object CreateVoice : SidebarEvent

    data object Logout : SidebarEvent
}

class SidebarProducerImpl : SidebarProducer {
    @Composable
    override fun produce(
        selectedDestination: NavDestination,
        userEmail: String,
        canCreateArtifact: Boolean,
        onNavigate: (NavDestination) -> Unit,
        onCreateFolder: () -> Unit,
        onCreateArtifact: () -> Unit,
        onCreateVoice: () -> Unit,
        onLogout: () -> Unit,
    ): SidebarState {
        return SidebarState(
            selectedDestination = selectedDestination,
            userEmail = userEmail,
            canCreateArtifact = canCreateArtifact,
            eventSink = { event ->
                when (event) {
                    is SidebarEvent.Navigate -> onNavigate(event.destination)
                    SidebarEvent.CreateFolder -> onCreateFolder()
                    SidebarEvent.CreateArtifact -> onCreateArtifact()
                    SidebarEvent.CreateVoice -> onCreateVoice()
                    SidebarEvent.Logout -> onLogout()
                }
            },
        )
    }
}
