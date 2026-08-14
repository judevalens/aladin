package dawn.system.anchor.features.login

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import com.slack.circuit.runtime.CircuitUiState
import com.slack.circuit.runtime.presenter.Presenter
import com.slack.circuit.runtime.screen.Screen
import com.slack.circuit.runtime.ui.ui
import dawn.system.anchor.services.auth.SessionManager
import dawn.system.anchor.services.design.AnchorTheme
import dawn.system.anchor.services.di.CommonParcelize
import dawn.system.anchor.services.di.bindScreen
import io.ktor.client.plugins.ClientRequestException
import io.ktor.http.HttpStatusCode
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.launch
import org.koin.core.module.Module
import org.koin.dsl.module

/**
 * Sign-in to the Aladin backend. Shown by the auth gate in [App][dawn.system.anchor.App]
 * whenever the session is [LoggedOut][dawn.system.anchor.services.auth.AuthState.LoggedOut];
 * a successful login flips the global auth state, so this screen never navigates itself.
 */
@CommonParcelize
data object LoginScreen : Screen {
    data class State(
        val email: String,
        val password: String,
        val submitting: Boolean,
        val error: String?,
        val eventSink: (Event) -> Unit,
    ) : CircuitUiState

    sealed interface Event {
        data class EmailChanged(val value: String) : Event
        data class PasswordChanged(val value: String) : Event
        data object Submit : Event
    }
}

class LoginPresenter(private val session: SessionManager) : Presenter<LoginScreen.State> {

    @Composable
    override fun present(): LoginScreen.State {
        var email by remember { mutableStateOf("") }
        var password by remember { mutableStateOf("") }
        var submitting by remember { mutableStateOf(false) }
        var error by remember { mutableStateOf<String?>(null) }
        val scope = rememberCoroutineScope()

        return LoginScreen.State(
            email = email,
            password = password,
            submitting = submitting,
            error = error,
        ) { event ->
            when (event) {
                is LoginScreen.Event.EmailChanged -> email = event.value
                is LoginScreen.Event.PasswordChanged -> password = event.value
                LoginScreen.Event.Submit -> {
                    if (submitting || email.isBlank() || password.isEmpty()) return@State
                    scope.launch {
                        submitting = true
                        error = null
                        try {
                            session.login(email, password)
                        } catch (e: CancellationException) {
                            throw e
                        } catch (e: Throwable) {
                            error = loginErrorMessage(e)
                        } finally {
                            submitting = false
                        }
                    }
                }
            }
        }
    }
}

private fun loginErrorMessage(e: Throwable): String = when {
    e is ClientRequestException && e.response.status == HttpStatusCode.Unauthorized ->
        "Invalid email or password."
    e is ClientRequestException ->
        "Sign-in failed (${e.response.status.value}). Try again."
    else ->
        "Can't reach Aladin. Check that the backend is running and try again."
}

@Composable
fun LoginUi(state: LoginScreen.State, modifier: Modifier = Modifier) {
    val c = AnchorTheme.colors
    Box(
        modifier
            .fillMaxSize()
            .background(c.bg)
            .imePadding(),
        contentAlignment = Alignment.Center,
    ) {
        Column(
            Modifier
                .widthIn(max = 380.dp)
                .fillMaxWidth()
                .padding(horizontal = 32.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Text("Aladin", style = MaterialTheme.typography.displaySmall, color = c.ink)
            Text(
                "Sign in to your workspace",
                style = MaterialTheme.typography.bodyLarge,
                color = c.ink3,
            )
            Spacer(Modifier.height(12.dp))

            val fieldColors = OutlinedTextFieldDefaults.colors(
                focusedContainerColor = c.field,
                unfocusedContainerColor = c.field,
                focusedBorderColor = c.line,
                unfocusedBorderColor = c.line2,
                focusedTextColor = c.ink,
                unfocusedTextColor = c.ink,
                cursorColor = c.amber,
                focusedLabelColor = c.ink3,
                unfocusedLabelColor = c.ink4,
            )
            OutlinedTextField(
                value = state.email,
                onValueChange = { state.eventSink(LoginScreen.Event.EmailChanged(it)) },
                label = { Text("Email") },
                singleLine = true,
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Email),
                colors = fieldColors,
                modifier = Modifier.fillMaxWidth(),
            )
            OutlinedTextField(
                value = state.password,
                onValueChange = { state.eventSink(LoginScreen.Event.PasswordChanged(it)) },
                label = { Text("Password") },
                singleLine = true,
                visualTransformation = PasswordVisualTransformation(),
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password),
                colors = fieldColors,
                modifier = Modifier.fillMaxWidth(),
            )

            state.error?.let { message ->
                Text(
                    message,
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.error,
                )
            }

            Spacer(Modifier.height(4.dp))
            Button(
                onClick = { state.eventSink(LoginScreen.Event.Submit) },
                enabled = !state.submitting && state.email.isNotBlank() && state.password.isNotEmpty(),
                colors = ButtonDefaults.buttonColors(
                    containerColor = c.amber,
                    contentColor = c.onAmber,
                ),
                modifier = Modifier.fillMaxWidth(),
            ) {
                Text(if (state.submitting) "Signing in…" else "Sign in")
            }
        }
    }
}

val loginModule: Module = module {
    bindScreen(
        LoginScreen::class,
        presenter = { _, _ -> LoginPresenter(get()) },
        uiFactory = { ui<LoginScreen.State> { state, modifier -> LoginUi(state, modifier) } },
    )
}
