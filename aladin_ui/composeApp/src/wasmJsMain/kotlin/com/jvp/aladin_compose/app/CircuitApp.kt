package com.jvp.aladin_compose.app

import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import com.jvp.aladin_compose.features.app.AppPresenter
import com.jvp.aladin_compose.features.app.AppScreen
import com.jvp.aladin_compose.features.app.AppUiFactory
import com.jvp.aladin_compose.repo.FakeFolderRepository
import com.jvp.aladin_compose.service.FolderService
import com.slack.circuit.foundation.Circuit
import com.slack.circuit.foundation.CircuitCompositionLocals
import com.slack.circuit.foundation.CircuitContent

@Composable
fun CircuitApp() {
    val scope = rememberCoroutineScope()
    val circuit = remember {
        val folderService = FolderService(FakeFolderRepository())
        Circuit.Builder()
            .addPresenterFactory(AppPresenter.Factory(folderService, scope))
            .addUiFactory(AppUiFactory())
            .build()
    }

    CircuitCompositionLocals(circuit) {
        CircuitContent(AppScreen)
    }
}
