package com.jvp.aladin_compose

import androidx.compose.runtime.Composable
import com.jvp.aladin_compose.app.CircuitApp
import com.jvp.aladin_compose.ui_lib.AladinTheme

@Composable
fun App() {
    AladinTheme {
        CircuitApp()
    }
}
