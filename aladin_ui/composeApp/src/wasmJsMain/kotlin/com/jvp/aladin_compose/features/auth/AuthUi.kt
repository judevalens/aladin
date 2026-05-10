package com.jvp.aladin_compose.features.auth

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.jvp.aladin_compose.ui_lib.AladinColor
import com.jvp.aladin_compose.ui_lib.AladinInteractionDefaults
import com.jvp.aladin_compose.ui_lib.aladinClickable

data class AuthUiState(
    val email: String,
    val password: String,
    val mode: AuthMode,
    val loading: Boolean,
    val errorMessage: String?,
    val eventSink: (AuthEvent) -> Unit,
)

enum class AuthMode {
    Login,
    Register,
}

sealed interface AuthEvent {
    data class EmailChanged(val email: String) : AuthEvent
    data class PasswordChanged(val password: String) : AuthEvent
    data object ToggleMode : AuthEvent
    data object Submit : AuthEvent
}

private val AuthPanelRadius = 4.dp
private val AuthControlRadius = 6.dp
private val AuthHairline = 0.5.dp
private val AuthError = Color(0xFF8F3329)

@Composable
fun AuthUi(state: AuthUiState, modifier: Modifier = Modifier) {
    Box(
        modifier = modifier.fillMaxSize().background(AladinColor.Canvas).padding(28.dp),
        contentAlignment = Alignment.Center,
    ) {
        Column(
            modifier =
                Modifier.width(432.dp)
                    .clip(RoundedCornerShape(AuthPanelRadius))
                    .background(AladinColor.Panel)
                    .border(AuthHairline, AladinColor.Divider, RoundedCornerShape(AuthPanelRadius))
                    .padding(24.dp),
            verticalArrangement = Arrangement.spacedBy(20.dp),
        ) {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(
                    text = "AUTH / WORKSPACE",
                    color = AladinColor.InkMuted,
                    fontFamily = FontFamily.Monospace,
                    fontSize = 10.sp,
                    fontWeight = FontWeight.Medium,
                    letterSpacing = 0.9.sp,
                )
                Text(
                    text = if (state.mode == AuthMode.Login) "Enter Aladin" else "Create your workspace",
                    color = AladinColor.Ink,
                    style = MaterialTheme.typography.headlineSmall,
                )
                Text(
                    text =
                        "Private sources, credentials, and workspace data stay tied to one owner.",
                    color = AladinColor.InkSecondary,
                    style = MaterialTheme.typography.bodySmall,
                )
            }

            Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                AuthInput(
                    label = "Email",
                    value = state.email,
                    enabled = !state.loading,
                    onValueChange = { state.eventSink(AuthEvent.EmailChanged(it)) },
                )
                AuthInput(
                    label = "Password",
                    value = state.password,
                    enabled = !state.loading,
                    isPassword = true,
                    onValueChange = { state.eventSink(AuthEvent.PasswordChanged(it)) },
                )
            }

            state.errorMessage?.let {
                Box(
                    modifier =
                        Modifier.fillMaxWidth()
                            .background(AladinColor.PanelMuted, RoundedCornerShape(AuthControlRadius))
                            .border(AuthHairline, AladinColor.Divider, RoundedCornerShape(AuthControlRadius))
                            .padding(horizontal = 10.dp, vertical = 8.dp)
                ) {
                    Text(text = it, color = AuthError, style = MaterialTheme.typography.bodySmall)
                }
            }

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    text =
                        if (state.mode == AuthMode.Login) "Create account"
                        else "Back to login",
                    color = AladinColor.InkMuted,
                    style = MaterialTheme.typography.labelMedium,
                    modifier =
                        Modifier.clip(RoundedCornerShape(AuthControlRadius))
                            .aladinClickable(enabled = !state.loading) {
                                state.eventSink(AuthEvent.ToggleMode)
                            }
                            .padding(horizontal = 8.dp, vertical = 6.dp),
                )
                AuthButton(
                    label =
                        if (state.loading) "Working..."
                        else if (state.mode == AuthMode.Login) "Log in"
                        else "Register",
                    enabled = !state.loading,
                    onClick = { state.eventSink(AuthEvent.Submit) },
                )
            }
        }
    }
}

@Composable
private fun AuthInput(
    label: String,
    value: String,
    enabled: Boolean,
    onValueChange: (String) -> Unit,
    isPassword: Boolean = false,
) {
    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
        Text(
            label.uppercase(),
            color = AladinColor.InkMuted,
            style = MaterialTheme.typography.labelMedium,
        )
        BasicTextField(
            value = value,
            onValueChange = onValueChange,
            enabled = enabled,
            textStyle = TextStyle(color = AladinColor.Ink, fontSize = 14.sp),
            singleLine = true,
            visualTransformation = if (isPassword) PasswordVisualTransformation() else VisualTransformation.None,
            modifier =
                Modifier.fillMaxWidth()
                    .height(44.dp)
                    .clip(RoundedCornerShape(AuthControlRadius))
                    .background(AladinColor.Canvas)
                    .border(AuthHairline, AladinColor.Border, RoundedCornerShape(AuthControlRadius))
                    .padding(horizontal = 12.dp, vertical = 12.dp),
        )
    }
}

@Composable
private fun AuthButton(label: String, enabled: Boolean, onClick: () -> Unit) {
    Box(
        modifier =
            Modifier.clip(RoundedCornerShape(AuthControlRadius))
                .border(AuthHairline, AladinColor.Ink, RoundedCornerShape(AuthControlRadius))
                .aladinClickable(
                    enabled = enabled,
                    colors =
                        AladinInteractionDefaults.colors(
                            rest = if (enabled) AladinColor.InkSurface else AladinColor.ControlHover,
                            hovered = if (enabled) AladinColor.InkSurfaceHover else AladinColor.ControlHover,
                            pressed = AladinColor.Ink,
                            disabled = AladinColor.ControlHover,
                        ),
                    shape = RoundedCornerShape(AuthControlRadius),
                    onClick = onClick,
                )
                .padding(horizontal = 18.dp, vertical = 9.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            label,
            color = if (enabled) AladinColor.OnInkSurface else AladinColor.InkMuted,
            style = MaterialTheme.typography.labelLarge,
            fontWeight = FontWeight.SemiBold,
        )
    }
}
