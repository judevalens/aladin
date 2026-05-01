package com.jvp.aladin_compose.features.app

import androidx.compose.runtime.Composable
import com.jvp.aladin_compose.service.FolderService
import com.slack.circuit.runtime.CircuitContext
import com.slack.circuit.runtime.Navigator
import com.slack.circuit.runtime.presenter.Presenter
import com.slack.circuit.runtime.screen.Screen
import kotlinx.coroutines.CoroutineScope

class AppPresenter(
    private val appWorkspaceProducer: AppWorkspaceProducer,
) : Presenter<AppState> {

    @Composable
    override fun present(): AppState {
        return appWorkspaceProducer.produce()
    }

    class Factory(
        private val folderService: FolderService,
        private val scope: CoroutineScope,
    ) : Presenter.Factory {
        override fun create(screen: Screen, navigator: Navigator, context: CircuitContext): Presenter<*>? {
            return when (screen) {
                AppScreen ->
                    AppPresenter(
                        appWorkspaceProducer = defaultAppWorkspaceProducer(folderService, scope),
                    )
                else -> null
            }
        }
    }
}
