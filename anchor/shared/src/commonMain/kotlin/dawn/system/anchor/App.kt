package dawn.system.anchor

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.tooling.preview.Preview
import com.slack.circuit.foundation.Circuit
import com.slack.circuit.foundation.CircuitCompositionLocals
import com.slack.circuit.foundation.CircuitContent
import dawn.system.anchor.features.login.LoginScreen
import dawn.system.anchor.features.shell.ShellScreen
import dawn.system.anchor.services.auth.AuthState
import dawn.system.anchor.services.auth.SessionManager
import dawn.system.anchor.services.design.AnchorTheme
import dawn.system.anchor.services.sync.SyncRunner
import org.koin.mp.KoinPlatform

/**
 * Composition root: resolve the graph, gate on auth, run the shell.
 *
 * Sync is tied to the session rather than to a screen — it starts when a session exists
 * and stops when it does not, so no surface has to remember to kick it and none can leave
 * it running for a signed-out user.
 */
@Composable
@Preview
fun App() {
    val koin = remember { KoinPlatform.getKoin() }
    val circuit = remember { koin.get<Circuit>() }
    val session = remember { koin.get<SessionManager>() }
    val sync = remember { koin.get<SyncRunner>() }

    val authState by session.state.collectAsState()

    LaunchedEffect(session) {
        if (authState is AuthState.Unknown) session.restore()
    }

    LaunchedEffect(authState) {
        if (authState is AuthState.LoggedIn) sync.start()
    }

    // The handoff ships dark as the default identity; light exists and is mapped, but
    // there is no theme setting yet, so don't let the simulator's appearance decide it.
    AnchorTheme(darkTheme = true) {
        CircuitCompositionLocals(circuit) {
            when (authState) {
                AuthState.Unknown -> BootSplash()
                AuthState.LoggedOut -> CircuitContent(LoginScreen)
                is AuthState.LoggedIn -> CircuitContent(ShellScreen)
            }
        }
    }
}

@Composable
private fun BootSplash() {
    val c = AnchorTheme.colors
    Box(
        Modifier.fillMaxSize().background(c.bg),
        contentAlignment = Alignment.Center,
    ) {
        Text("Aladin", style = MaterialTheme.typography.headlineMedium, color = c.ink4)
    }
}
