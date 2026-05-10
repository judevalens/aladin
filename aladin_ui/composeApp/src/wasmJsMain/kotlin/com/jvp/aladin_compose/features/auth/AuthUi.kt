package com.jvp.aladin_compose.features.auth

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.jvp.aladin_compose.ui_lib.AladinColor
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

@Composable
fun AuthUi(state: AuthUiState, modifier: Modifier = Modifier) {
    Box(
        modifier = modifier.fillMaxSize().background(AladinColor.Canvas).padding(24.dp),
        contentAlignment = Alignment.Center,
    ) {
        Column(
            modifier =
                Modifier.width(420.dp)
                    .clip(RoundedCornerShape(18.dp))
                    .background(AladinColor.Panel)
                    .border(1.dp, AladinColor.Border, RoundedCornerShape(18.dp))
                    .padding(28.dp),
            verticalArrangement = Arrangement.spacedBy(18.dp),
        ) {
            Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                Text(
                    text = if (state.mode == AuthMode.Login) "Enter Aladin" else "Create your workspace",
                    color = AladinColor.Ink,
                    fontSize = 24.sp,
                    fontWeight = FontWeight.SemiBold,
                )
                Text(
                    text = "Sign in to keep private sources, credentials, and workspace data tied to one owner.",
                    color = AladinColor.InkSecondary,
                    fontSize = 13.sp,
                    lineHeight = 18.sp,
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
                Text(text = it, color = Color(0xFF9F3A2F), fontSize = 12.sp)
            }

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    text = if (state.mode == AuthMode.Login) "Need an account?" else "Already have one?",
                    color = AladinColor.InkMuted,
                    fontSize = 12.sp,
                    modifier =
                        Modifier.aladinClickable(enabled = !state.loading) {
                            state.eventSink(AuthEvent.ToggleMode)
                        },
                )
                AuthButton(
                    label = if (state.loading) "Working..." else if (state.mode == AuthMode.Login) "Log in" else "Register",
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
        Text(label, color = AladinColor.InkMuted, fontSize = 11.sp, fontWeight = FontWeight.SemiBold)
        BasicTextField(
            value = value,
            onValueChange = onValueChange,
            enabled = enabled,
            textStyle = TextStyle(color = AladinColor.Ink, fontSize = 14.sp),
            singleLine = true,
            visualTransformation = if (isPassword) PasswordVisualTransformation() else androidx.compose.ui.text.input.VisualTransformation.None,
            modifier =
                Modifier.fillMaxWidth()
                    .height(42.dp)
                    .clip(RoundedCornerShape(10.dp))
                    .background(AladinColor.Canvas)
                    .border(1.dp, AladinColor.Border, RoundedCornerShape(10.dp))
                    .padding(horizontal = 12.dp, vertical = 11.dp),
        )
    }
}

@Composable
private fun AuthButton(label: String, enabled: Boolean, onClick: () -> Unit) {
    Box(
        modifier =
            Modifier.clip(RoundedCornerShape(999.dp))
                .background(if (enabled) AladinColor.Ink else AladinColor.InkMuted)
                .aladinClickable(enabled = enabled, onClick = onClick)
                .padding(horizontal = 18.dp, vertical = 10.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(label, color = AladinColor.Canvas, fontSize = 12.sp, fontWeight = FontWeight.SemiBold)
    }
}
